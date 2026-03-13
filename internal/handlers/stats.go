package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"banki/internal/middleware"
	"banki/internal/models"
	"banki/internal/templateutil"
)

type StatsHandler struct {
	DB *sql.DB
}

func (h *StatsHandler) Index(w http.ResponseWriter, r *http.Request) {
	isAdmin := middleware.IsAdmin(r)
	memberID := middleware.GetFamilyMemberID(r)

	var bankStats []models.BankStat
	var memberStats []models.MemberStat
	var asvEntries []models.ASVEntry
	var members []models.FamilyMember
	var banks []models.Bank
	var matrix map[string]models.MatrixCell

	if isAdmin {
		bankStats, _ = models.StatsByBank(h.DB)
		memberStats, _ = models.StatsByMember(h.DB)
		asvEntries, _ = models.ASVCheck(h.DB)
		members, banks, matrix, _ = models.DepositMatrix(h.DB)
	} else {
		bankStats, _ = models.StatsByBankForMember(h.DB, memberID)
		memberStats, _ = models.StatsByMemberFiltered(h.DB, memberID)
		asvEntries, _ = models.ASVCheckForMember(h.DB, memberID)
		members, banks, matrix, _ = models.DepositMatrixForMember(h.DB, memberID)
	}

	var totalAmount float64
	for _, bs := range bankStats {
		totalAmount += bs.Total
	}

	templateutil.Render(w, "stats/index.html", map[string]any{
		"BankStats":    bankStats,
		"TotalAmount":  totalAmount,
		"MemberStats": memberStats,
		"ASVEntries":  asvEntries,
		"Members":     members,
		"Banks":       banks,
		"Matrix":      matrix,
		"ASVLimit":    models.ASVLimit,
		"IsAdmin":     isAdmin,
	})
}

func (h *StatsHandler) ASVCheck(w http.ResponseWriter, r *http.Request) {
	holderID, _ := strconv.ParseInt(r.URL.Query().Get("holder_id"), 10, 64)
	bankID, _ := strconv.ParseInt(r.URL.Query().Get("bank_id"), 10, 64)
	amountStr := r.URL.Query().Get("amount")
	newAmount, _ := strconv.ParseFloat(amountStr, 64)

	if holderID == 0 || bankID == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	currentTotal, _ := models.ASVCheckForPair(h.DB, holderID, bankID)
	// Exclude current deposit if editing
	excludeID, _ := strconv.ParseInt(r.URL.Query().Get("exclude_id"), 10, 64)
	if excludeID > 0 {
		dep, err := models.GetDeposit(h.DB, excludeID)
		if err == nil && dep.HolderID == holderID && dep.BankID == bankID {
			currentTotal -= dep.Amount
		}
	}

	total := currentTotal + newAmount

	templateutil.RenderPartial(w, "partials/asv_warning.html", map[string]any{
		"Total":   total,
		"Limit":   models.ASVLimit,
		"Percent": total / models.ASVLimit * 100,
	})
}
