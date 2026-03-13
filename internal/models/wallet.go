package models

import "database/sql"

type Wallet struct {
	ID      int64
	Name    string
	Members []FamilyMember
}

type WalletSummary struct {
	Wallet       Wallet
	Deposits     []Deposit
	TotalAmount  float64
	MemberTotals []WalletMemberTotal
}

type WalletMemberTotal struct {
	Member  FamilyMember
	AsHolder float64
	AsOwner  float64
}

func ListWallets(db *sql.DB) ([]Wallet, error) {
	rows, err := db.Query("SELECT id, name FROM wallets ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []Wallet
	for rows.Next() {
		var w Wallet
		if err := rows.Scan(&w.ID, &w.Name); err != nil {
			return nil, err
		}
		wallets = append(wallets, w)
	}
	rows.Close()

	// Fetch members after closing the first cursor to avoid connection deadlock
	for i := range wallets {
		wallets[i].Members, _ = getWalletMembers(db, wallets[i].ID)
	}
	return wallets, nil
}

func GetWallet(db *sql.DB, id int64) (*Wallet, error) {
	w := &Wallet{}
	err := db.QueryRow("SELECT id, name FROM wallets WHERE id = ?", id).Scan(&w.ID, &w.Name)
	if err != nil {
		return nil, err
	}
	w.Members, _ = getWalletMembers(db, w.ID)
	return w, nil
}

func CreateWallet(db *sql.DB, name string, memberIDs []int64) (int64, error) {
	res, err := db.Exec("INSERT INTO wallets (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	for _, mid := range memberIDs {
		db.Exec("INSERT INTO wallet_members (wallet_id, family_member_id) VALUES (?, ?)", id, mid)
	}
	return id, nil
}

func UpdateWallet(db *sql.DB, id int64, name string, memberIDs []int64) error {
	if _, err := db.Exec("UPDATE wallets SET name = ? WHERE id = ?", name, id); err != nil {
		return err
	}
	db.Exec("DELETE FROM wallet_members WHERE wallet_id = ?", id)
	for _, mid := range memberIDs {
		db.Exec("INSERT INTO wallet_members (wallet_id, family_member_id) VALUES (?, ?)", id, mid)
	}
	return nil
}

func DeleteWallet(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM wallets WHERE id = ?", id)
	return err
}

// ListWalletsForMember returns wallets where the given member is a participant
func ListWalletsForMember(db *sql.DB, memberID int64) ([]Wallet, error) {
	rows, err := db.Query(`
		SELECT w.id, w.name FROM wallets w
		JOIN wallet_members wm ON w.id = wm.wallet_id
		WHERE wm.family_member_id = ?
		ORDER BY w.name`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallets []Wallet
	for rows.Next() {
		var w Wallet
		if err := rows.Scan(&w.ID, &w.Name); err != nil {
			return nil, err
		}
		wallets = append(wallets, w)
	}
	rows.Close()

	for i := range wallets {
		wallets[i].Members, _ = getWalletMembers(db, wallets[i].ID)
	}
	return wallets, nil
}

// IsMemberOfWallet checks if a family member is a participant of the wallet
func IsMemberOfWallet(db *sql.DB, walletID, memberID int64) bool {
	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM wallet_members WHERE wallet_id = ? AND family_member_id = ?",
		walletID, memberID).Scan(&count)
	return count > 0
}

func getWalletMembers(db *sql.DB, walletID int64) ([]FamilyMember, error) {
	rows, err := db.Query(`
		SELECT fm.id, fm.name FROM family_members fm
		JOIN wallet_members wm ON fm.id = wm.family_member_id
		WHERE wm.wallet_id = ? ORDER BY fm.name`, walletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []FamilyMember
	for rows.Next() {
		var m FamilyMember
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func GetWalletSummary(db *sql.DB, id int64) (*WalletSummary, error) {
	wallet, err := GetWallet(db, id)
	if err != nil {
		return nil, err
	}

	if len(wallet.Members) == 0 {
		return &WalletSummary{Wallet: *wallet}, nil
	}

	// Build member ID list for query
	memberIDs := make([]any, len(wallet.Members))
	placeholders := ""
	for i, m := range wallet.Members {
		memberIDs[i] = m.ID
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
	}

	// Get all deposits where holder or owner is a wallet member
	query := `SELECT d.id, d.bank_id, d.holder_id, d.owner_id,
		d.amount, d.interest_rate, d.open_date, d.end_date,
		d.has_capitalization, d.is_replenishable, d.status, d.notes,
		b.name, h.name, o.name
		FROM deposits d
		JOIN banks b ON d.bank_id = b.id
		JOIN family_members h ON d.holder_id = h.id
		JOIN family_members o ON d.owner_id = o.id
		WHERE d.status = 'active'
		AND (d.holder_id IN (` + placeholders + `) OR d.owner_id IN (` + placeholders + `))
		ORDER BY d.end_date ASC`

	args := append(memberIDs, memberIDs...)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &WalletSummary{Wallet: *wallet}
	seen := make(map[int64]bool)

	for rows.Next() {
		var d Deposit
		if err := rows.Scan(
			&d.ID, &d.BankID, &d.HolderID, &d.OwnerID,
			&d.Amount, &d.InterestRate, &d.OpenDate, &d.EndDate,
			&d.HasCapitalization, &d.IsReplenishable, &d.Status, &d.Notes,
			&d.BankName, &d.HolderName, &d.OwnerName,
		); err != nil {
			return nil, err
		}
		if !seen[d.ID] {
			summary.Deposits = append(summary.Deposits, d)
			summary.TotalAmount += d.Amount
			seen[d.ID] = true
		}
	}

	// Compute per-member totals
	for _, m := range wallet.Members {
		mt := WalletMemberTotal{Member: m}
		for _, d := range summary.Deposits {
			if d.HolderID == m.ID {
				mt.AsHolder += d.Amount
			}
			if d.OwnerID == m.ID {
				mt.AsOwner += d.Amount
			}
		}
		summary.MemberTotals = append(summary.MemberTotals, mt)
	}

	return summary, nil
}
