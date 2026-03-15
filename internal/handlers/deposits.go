package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"banki/internal/middleware"
	"banki/internal/models"
	"banki/internal/templateutil"

	"github.com/go-chi/chi/v5"
)

type HolderGroup struct {
	HolderName  string
	TotalAmount float64
	Count       int
	Deposits    []models.Deposit
}

type DepositsHandler struct {
	DB *sql.DB
}

func (h *DepositsHandler) Index(w http.ResponseWriter, r *http.Request) {
	banks, _ := models.ListBanks(h.DB)
	members, _ := models.ListFamilyMembers(h.DB)
	isAdmin := middleware.IsAdmin(r)
	memberID := middleware.GetFamilyMemberID(r)

	f := h.parseFilter(r)
	if !isAdmin {
		f.MemberID = memberID
	}
	deposits, _ := models.ListDeposits(h.DB, f)

	var activeDeposits, closedDeposits []models.Deposit
	for _, d := range deposits {
		if d.Status == "active" {
			activeDeposits = append(activeDeposits, d)
		} else {
			closedDeposits = append(closedDeposits, d)
		}
	}

	asvOver := h.buildASVOver(isAdmin, memberID)

	var holderGroups []HolderGroup
	groupByHolder := isAdmin && f.HolderID == 0

	if groupByHolder {
		groupMap := make(map[int64]*HolderGroup)
		var groupOrder []int64
		for _, d := range activeDeposits {
			g, ok := groupMap[d.HolderID]
			if !ok {
				g = &HolderGroup{HolderName: d.HolderName}
				groupMap[d.HolderID] = g
				groupOrder = append(groupOrder, d.HolderID)
			}
			g.Deposits = append(g.Deposits, d)
			g.TotalAmount += d.Amount
			g.Count++
		}
		for _, id := range groupOrder {
			holderGroups = append(holderGroups, *groupMap[id])
		}
	}

	templateutil.Render(w, "deposits/index.html", map[string]any{
		"ActiveDeposits": activeDeposits,
		"ClosedDeposits": closedDeposits,
		"Banks":          banks,
		"Members":        members,
		"Filter":         f,
		"IsAdmin":        isAdmin,
		"MemberID":       memberID,
		"ASVOver":        asvOver,
		"HolderGroups":   holderGroups,
		"GroupByHolder":  groupByHolder,
	})
}

func (h *DepositsHandler) List(w http.ResponseWriter, r *http.Request) {
	isAdmin := middleware.IsAdmin(r)
	f := h.parseFilter(r)
	if !isAdmin {
		f.MemberID = middleware.GetFamilyMemberID(r)
	}
	deposits, _ := models.ListDeposits(h.DB, f)
	memberID := middleware.GetFamilyMemberID(r)

	var activeDeposits, closedDeposits []models.Deposit
	for _, d := range deposits {
		if d.Status == "active" {
			activeDeposits = append(activeDeposits, d)
		} else {
			closedDeposits = append(closedDeposits, d)
		}
	}

	asvOver := h.buildASVOver(isAdmin, memberID)

	var holderGroups []HolderGroup
	groupByHolder := isAdmin && f.HolderID == 0

	if groupByHolder {
		groupMap := make(map[int64]*HolderGroup)
		var groupOrder []int64
		for _, d := range activeDeposits {
			g, ok := groupMap[d.HolderID]
			if !ok {
				g = &HolderGroup{HolderName: d.HolderName}
				groupMap[d.HolderID] = g
				groupOrder = append(groupOrder, d.HolderID)
			}
			g.Deposits = append(g.Deposits, d)
			g.TotalAmount += d.Amount
			g.Count++
		}
		for _, id := range groupOrder {
			holderGroups = append(holderGroups, *groupMap[id])
		}
	}

	templateutil.RenderPartial(w, "deposits/list.html", map[string]any{
		"ActiveDeposits": activeDeposits,
		"ClosedDeposits": closedDeposits,
		"IsAdmin":        isAdmin,
		"ASVOver":        asvOver,
		"HolderGroups":   holderGroups,
		"GroupByHolder":  groupByHolder,
	})
}

func (h *DepositsHandler) Create(w http.ResponseWriter, r *http.Request) {
	d := h.parseForm(r)
	if !middleware.IsAdmin(r) {
		memberID := middleware.GetFamilyMemberID(r)
		if d.HolderID != memberID && d.OwnerID != memberID {
			http.Error(w, "Доступ запрещён", http.StatusForbidden)
			return
		}
	}
	if _, err := models.CreateDeposit(h.DB, d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/deposits", http.StatusSeeOther)
}

func (h *DepositsHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	deposit, err := models.GetDeposit(h.DB, id)
	if err != nil {
		http.Error(w, "Вклад не найден", http.StatusNotFound)
		return
	}
	if !middleware.IsAdmin(r) {
		memberID := middleware.GetFamilyMemberID(r)
		if deposit.HolderID != memberID && deposit.OwnerID != memberID {
			http.Error(w, "Доступ запрещён", http.StatusForbidden)
			return
		}
	}
	banks, _ := models.ListBanks(h.DB)
	members, _ := models.ListFamilyMembers(h.DB)

	templateutil.Render(w, "deposits/form.html", map[string]any{
		"Deposit":  deposit,
		"Banks":    banks,
		"Members":  members,
		"IsAdmin":  middleware.IsAdmin(r),
		"MemberID": middleware.GetFamilyMemberID(r),
	})
}

func (h *DepositsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if !middleware.IsAdmin(r) {
		existing, err := models.GetDeposit(h.DB, id)
		if err != nil {
			http.Error(w, "Вклад не найден", http.StatusNotFound)
			return
		}
		memberID := middleware.GetFamilyMemberID(r)
		if existing.HolderID != memberID && existing.OwnerID != memberID {
			http.Error(w, "Доступ запрещён", http.StatusForbidden)
			return
		}
	}
	d := h.parseForm(r)
	d.ID = id
	if !middleware.IsAdmin(r) {
		memberID := middleware.GetFamilyMemberID(r)
		if d.HolderID != memberID && d.OwnerID != memberID {
			http.Error(w, "Доступ запрещён", http.StatusForbidden)
			return
		}
	}
	if err := models.UpdateDeposit(h.DB, d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/deposits", http.StatusSeeOther)
}

func (h *DepositsHandler) Close(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if !middleware.IsAdmin(r) {
		existing, err := models.GetDeposit(h.DB, id)
		if err != nil {
			http.Error(w, "Вклад не найден", http.StatusNotFound)
			return
		}
		memberID := middleware.GetFamilyMemberID(r)
		if existing.HolderID != memberID && existing.OwnerID != memberID {
			http.Error(w, "Доступ запрещён", http.StatusForbidden)
			return
		}
	}
	if err := models.CloseDeposit(h.DB, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/deposits", http.StatusSeeOther)
}

func (h *DepositsHandler) parseFilter(r *http.Request) models.DepositFilter {
	var f models.DepositFilter
	f.BankID, _ = strconv.ParseInt(r.URL.Query().Get("bank_id"), 10, 64)
	f.HolderID, _ = strconv.ParseInt(r.URL.Query().Get("holder_id"), 10, 64)
	f.OwnerID, _ = strconv.ParseInt(r.URL.Query().Get("owner_id"), 10, 64)
	f.Status = r.URL.Query().Get("status")
	return f
}

func (h *DepositsHandler) buildASVOver(isAdmin bool, memberID int64) map[string]bool {
	var asvEntries []models.ASVEntry
	if isAdmin {
		asvEntries, _ = models.ASVCheck(h.DB)
	} else {
		asvEntries, _ = models.ASVCheckForMember(h.DB, memberID)
	}
	asvOver := make(map[string]bool)
	for _, e := range asvEntries {
		if e.Total > models.ASVLimit {
			asvOver[fmt.Sprintf("%d_%d", e.HolderID, e.BankID)] = true
		}
	}
	return asvOver
}

func (h *DepositsHandler) parseForm(r *http.Request) *models.Deposit {
	d := &models.Deposit{}
	d.BankID, _ = strconv.ParseInt(r.FormValue("bank_id"), 10, 64)
	d.HolderID, _ = strconv.ParseInt(r.FormValue("holder_id"), 10, 64)
	d.OwnerID, _ = strconv.ParseInt(r.FormValue("owner_id"), 10, 64)
	d.Amount, _ = strconv.ParseFloat(strings.ReplaceAll(r.FormValue("amount"), " ", ""), 64)
	d.InterestRate, _ = strconv.ParseFloat(r.FormValue("interest_rate"), 64)
	d.OpenDate = r.FormValue("open_date")
	d.EndDate = r.FormValue("end_date")
	d.HasCapitalization = r.FormValue("has_capitalization") == "on"
	d.IsReplenishable = r.FormValue("is_replenishable") == "on"
	d.Notes = strings.TrimSpace(r.FormValue("notes"))
	return d
}
