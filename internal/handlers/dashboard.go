package handlers

import (
	"database/sql"
	"fmt"
	"net/http"

	"banki/internal/middleware"
	"banki/internal/models"
	"banki/internal/templateutil"
)

type DashboardHandler struct {
	DB *sql.DB
}

func (h *DashboardHandler) Index(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	isAdmin := session.IsAdmin
	memberID := session.FamilyMemberID

	var totalAmount float64
	var memberStats []models.MemberStat
	var asvEntries []models.ASVEntry
	var expiring []models.Deposit
	var wallets []models.Wallet
	var activeDeposits []models.Deposit

	if isAdmin {
		totalAmount, _ = models.TotalActiveAmount(h.DB)
		memberStats, _ = models.StatsByMember(h.DB)
		asvEntries, _ = models.ASVCheck(h.DB)
		expiring, _ = models.ExpiringDeposits(h.DB, 30)
		wallets, _ = models.ListWallets(h.DB)
		activeDeposits, _ = models.ListDeposits(h.DB, models.DepositFilter{Status: "active"})
	} else {
		totalAmount, _ = models.TotalActiveAmountForMember(h.DB, memberID)
		memberStats, _ = models.StatsByMemberFiltered(h.DB, memberID)
		asvEntries, _ = models.ASVCheckForMember(h.DB, memberID)
		expiring, _ = models.ExpiringDepositsForMember(h.DB, 30, memberID)
		wallets, _ = models.ListWalletsForMember(h.DB, memberID)
		activeDeposits, _ = models.ListDeposits(h.DB, models.DepositFilter{Status: "active", MemberID: memberID})
	}

	var totalExpectedProfit float64
	for _, d := range activeDeposits {
		totalExpectedProfit += d.TotalProfit()
	}

	asvOver := make(map[string]bool)
	for _, e := range asvEntries {
		if e.Total > models.ASVLimit {
			asvOver[fmt.Sprintf("%d_%d", e.HolderID, e.BankID)] = true
		}
	}

	data := map[string]any{
		"Username":            session.Username,
		"IsAdmin":             isAdmin,
		"TotalAmount":         totalAmount,
		"TotalExpectedProfit": totalExpectedProfit,
		"MemberStats":         memberStats,
		"ASVOver":             asvOver,
		"Expiring":            expiring,
		"Wallets":             wallets,
	}
	templateutil.Render(w, "dashboard.html", data)
}
