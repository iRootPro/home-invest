package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"banki/internal/middleware"
	"banki/internal/models"
	"banki/internal/templateutil"

	"github.com/go-chi/chi/v5"
)

type WalletsHandler struct {
	DB *sql.DB
}

func (h *WalletsHandler) Index(w http.ResponseWriter, r *http.Request) {
	isAdmin := middleware.IsAdmin(r)
	var wallets []models.Wallet
	if isAdmin {
		wallets, _ = models.ListWallets(h.DB)
	} else {
		wallets, _ = models.ListWalletsForMember(h.DB, middleware.GetFamilyMemberID(r))
	}
	members, _ := models.ListFamilyMembers(h.DB)
	templateutil.Render(w, "wallets/index.html", map[string]any{
		"Wallets": wallets,
		"Members": members,
		"IsAdmin": isAdmin,
	})
}

func (h *WalletsHandler) Create(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Название обязательно", http.StatusBadRequest)
		return
	}
	memberIDs := parseMemberIDs(r)
	if _, err := models.CreateWallet(h.DB, name, memberIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/wallets", http.StatusSeeOther)
}

func (h *WalletsHandler) View(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if !middleware.IsAdmin(r) {
		memberID := middleware.GetFamilyMemberID(r)
		if !models.IsMemberOfWallet(h.DB, id, memberID) {
			http.Error(w, "Доступ запрещён", http.StatusForbidden)
			return
		}
	}
	summary, err := models.GetWalletSummary(h.DB, id)
	if err != nil {
		http.Error(w, "Кошелёк не найден", http.StatusNotFound)
		return
	}
	templateutil.Render(w, "wallets/view.html", map[string]any{
		"Summary": summary,
		"IsAdmin": middleware.IsAdmin(r),
	})
}

func (h *WalletsHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	wallet, err := models.GetWallet(h.DB, id)
	if err != nil {
		http.Error(w, "Кошелёк не найден", http.StatusNotFound)
		return
	}
	allMembers, _ := models.ListFamilyMembers(h.DB)

	// Mark which members are in this wallet
	memberSet := make(map[int64]bool)
	for _, m := range wallet.Members {
		memberSet[m.ID] = true
	}

	templateutil.Render(w, "wallets/form.html", map[string]any{
		"Wallet":    wallet,
		"Members":   allMembers,
		"MemberSet": memberSet,
		"IsAdmin":   true,
	})
}

func (h *WalletsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Название обязательно", http.StatusBadRequest)
		return
	}
	memberIDs := parseMemberIDs(r)
	if err := models.UpdateWallet(h.DB, id, name, memberIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/wallets", http.StatusSeeOther)
}

func (h *WalletsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := models.DeleteWallet(h.DB, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/wallets", http.StatusSeeOther)
}

func parseMemberIDs(r *http.Request) []int64 {
	r.ParseForm()
	var ids []int64
	for _, v := range r.Form["member_ids"] {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
