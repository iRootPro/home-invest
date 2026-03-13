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
	var roleTotals models.RoleTotals
	var memberStats []models.MemberStat
	var asvEntries []models.ASVEntry
	var expiring []models.Deposit
	var wallets []models.Wallet
	var activeDeposits []models.Deposit

	if isAdmin {
		totalAmount, _ = models.TotalActiveAmount(h.DB)
		roleTotals, _ = models.TotalsByRole(h.DB)
		memberStats, _ = models.StatsByMember(h.DB)
		asvEntries, _ = models.ASVCheck(h.DB)
		expiring, _ = models.ExpiringDeposits(h.DB, 30)
		wallets, _ = models.ListWallets(h.DB)
		activeDeposits, _ = models.ListDeposits(h.DB, models.DepositFilter{Status: "active"})
	} else {
		totalAmount, _ = models.TotalActiveAmountForMember(h.DB, memberID)
		roleTotals, _ = models.TotalsByRoleForMember(h.DB, memberID)
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

	// Deposit count and weighted average rate
	depositCount := len(activeDeposits)
	var weightedRate float64
	if totalAmount > 0 {
		for _, d := range activeDeposits {
			weightedRate += d.InterestRate * d.Amount
		}
		weightedRate /= totalAmount
	}

	// Bank stats
	var bankStats []models.BankStat
	if isAdmin {
		bankStats, _ = models.StatsByBank(h.DB)
	} else {
		bankStats, _ = models.StatsByBankForMember(h.DB, memberID)
	}

	// ASV risk summary
	asvOver := make(map[string]bool)
	var asvRed, asvAmber, asvGreen int
	for _, e := range asvEntries {
		if e.Total > models.ASVLimit {
			asvOver[fmt.Sprintf("%d_%d", e.HolderID, e.BankID)] = true
		}
		switch e.Color {
		case "red":
			asvRed++
		case "orange":
			asvAmber++
		default:
			asvGreen++
		}
	}

	// Wallet summaries
	var walletSummaries []*models.WalletSummary
	for _, w := range wallets {
		s, _ := models.GetWalletSummary(h.DB, w.ID)
		if s != nil {
			walletSummaries = append(walletSummaries, s)
		}
	}

	data := map[string]any{
		"Username":            session.Username,
		"IsAdmin":             isAdmin,
		"TotalAmount":         totalAmount,
		"TotalExpectedProfit": totalExpectedProfit,
		"DepositCount":        depositCount,
		"WeightedRate":        weightedRate,
		"RoleTotals":          roleTotals,
		"MemberStats":         memberStats,
		"BankStats":           bankStats,
		"ASVOver":             asvOver,
		"ASVRed":              asvRed,
		"ASVAmber":            asvAmber,
		"ASVGreen":            asvGreen,
		"Expiring":            expiring,
		"Wallets":             walletSummaries,
	}
	templateutil.Render(w, "dashboard.html", data)
}
