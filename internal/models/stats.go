package models

import (
	"database/sql"
	"fmt"
)

const ASVLimit = 1_400_000.0

type ASVEntry struct {
	HolderID   int64
	HolderName string
	BankID     int64
	BankName   string
	Total      float64
	Percent    float64 // Total / ASVLimit * 100
	Color      string  // "green", "orange", "red"
}

// ASVCheck returns aggregated amounts per (holder, bank) for active deposits
func ASVCheck(db *sql.DB) ([]ASVEntry, error) {
	rows, err := db.Query(`
		SELECT d.holder_id, h.name, d.bank_id, b.name, SUM(d.amount)
		FROM deposits d
		JOIN family_members h ON d.holder_id = h.id
		JOIN banks b ON d.bank_id = b.id
		WHERE d.status = 'active'
		GROUP BY d.holder_id, d.bank_id
		ORDER BY SUM(d.amount) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ASVEntry
	for rows.Next() {
		var e ASVEntry
		if err := rows.Scan(&e.HolderID, &e.HolderName, &e.BankID, &e.BankName, &e.Total); err != nil {
			return nil, err
		}
		e.Percent = e.Total / ASVLimit * 100
		if e.Percent >= 100 {
			e.Color = "red"
		} else if e.Percent >= 80 {
			e.Color = "orange"
		} else {
			e.Color = "green"
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// ASVCheckForPair returns the total for a specific holder+bank combination (active deposits)
func ASVCheckForPair(db *sql.DB, holderID, bankID int64) (float64, error) {
	var total float64
	err := db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0) FROM deposits
		WHERE holder_id = ? AND bank_id = ? AND status = 'active'`,
		holderID, bankID).Scan(&total)
	return total, err
}

type BankStat struct {
	BankID   int64
	BankName string
	Total    float64
	Count    int
}

func StatsByBank(db *sql.DB) ([]BankStat, error) {
	rows, err := db.Query(`
		SELECT d.bank_id, b.name, SUM(d.amount), COUNT(*)
		FROM deposits d
		JOIN banks b ON d.bank_id = b.id
		WHERE d.status = 'active'
		GROUP BY d.bank_id
		ORDER BY SUM(d.amount) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []BankStat
	for rows.Next() {
		var s BankStat
		if err := rows.Scan(&s.BankID, &s.BankName, &s.Total, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

type MemberStat struct {
	MemberID   int64
	MemberName string
	AsHolder   float64
	AsOwner    float64
}

func StatsByMember(db *sql.DB) ([]MemberStat, error) {
	rows, err := db.Query(`
		SELECT fm.id, fm.name,
			COALESCE((SELECT SUM(amount) FROM deposits WHERE holder_id = fm.id AND status = 'active'), 0),
			COALESCE((SELECT SUM(amount) FROM deposits WHERE owner_id = fm.id AND status = 'active'), 0)
		FROM family_members fm
		ORDER BY fm.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []MemberStat
	for rows.Next() {
		var s MemberStat
		if err := rows.Scan(&s.MemberID, &s.MemberName, &s.AsHolder, &s.AsOwner); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

type MatrixCell struct {
	MemberID   int64
	MemberName string
	BankID     int64
	BankName   string
	Total      float64
	HasDeposit bool
}

func DepositMatrix(db *sql.DB) ([]FamilyMember, []Bank, map[string]MatrixCell, error) {
	members, err := ListFamilyMembers(db)
	if err != nil {
		return nil, nil, nil, err
	}
	banks, err := ListBanks(db)
	if err != nil {
		return nil, nil, nil, err
	}

	rows, err := db.Query(`
		SELECT d.holder_id, d.bank_id, SUM(d.amount)
		FROM deposits d
		WHERE d.status = 'active'
		GROUP BY d.holder_id, d.bank_id`)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	matrix := make(map[string]MatrixCell)
	for rows.Next() {
		var holderID, bankID int64
		var total float64
		if err := rows.Scan(&holderID, &bankID, &total); err != nil {
			return nil, nil, nil, err
		}
		key := MatrixKey(holderID, bankID)
		matrix[key] = MatrixCell{
			MemberID:   holderID,
			BankID:     bankID,
			Total:      total,
			HasDeposit: true,
		}
	}

	return members, banks, matrix, nil
}

func MatrixKey(memberID, bankID int64) string {
	return fmt.Sprintf("%d_%d", memberID, bankID)
}

type RoleTotals struct {
	AsHolder float64
	AsOwner  float64
	NotOwn   float64 // holder_id != owner_id
}

func TotalsByRole(db *sql.DB) (RoleTotals, error) {
	var rt RoleTotals
	err := db.QueryRow(`
		SELECT
			COALESCE(SUM(amount), 0),
			COALESCE(SUM(amount), 0),
			COALESCE(SUM(CASE WHEN holder_id != owner_id THEN amount ELSE 0 END), 0)
		FROM deposits WHERE status = 'active'`).Scan(&rt.AsHolder, &rt.AsOwner, &rt.NotOwn)
	return rt, err
}

func TotalsByRoleForMember(db *sql.DB, memberID int64) (RoleTotals, error) {
	var rt RoleTotals
	row := db.QueryRow(`
		SELECT
			COALESCE((SELECT SUM(amount) FROM deposits WHERE status='active' AND holder_id = ?), 0),
			COALESCE((SELECT SUM(amount) FROM deposits WHERE status='active' AND owner_id = ?), 0),
			COALESCE((SELECT SUM(amount) FROM deposits WHERE status='active' AND holder_id = ? AND owner_id != ?), 0)`,
		memberID, memberID, memberID, memberID)
	err := row.Scan(&rt.AsHolder, &rt.AsOwner, &rt.NotOwn)
	return rt, err
}

func TotalActiveAmount(db *sql.DB) (float64, error) {
	var total float64
	err := db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM deposits WHERE status = 'active'").Scan(&total)
	return total, err
}

// TotalActiveAmountForMember returns sum of active deposits where member is holder or owner
func TotalActiveAmountForMember(db *sql.DB, memberID int64) (float64, error) {
	var total float64
	err := db.QueryRow(
		"SELECT COALESCE(SUM(amount), 0) FROM deposits WHERE status = 'active' AND (holder_id = ? OR owner_id = ?)",
		memberID, memberID).Scan(&total)
	return total, err
}

// ASVCheckForMember returns ASV entries filtered by holder_id = memberID
func ASVCheckForMember(db *sql.DB, memberID int64) ([]ASVEntry, error) {
	rows, err := db.Query(`
		SELECT d.holder_id, h.name, d.bank_id, b.name, SUM(d.amount)
		FROM deposits d
		JOIN family_members h ON d.holder_id = h.id
		JOIN banks b ON d.bank_id = b.id
		WHERE d.status = 'active' AND d.holder_id = ?
		GROUP BY d.holder_id, d.bank_id
		ORDER BY SUM(d.amount) DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ASVEntry
	for rows.Next() {
		var e ASVEntry
		if err := rows.Scan(&e.HolderID, &e.HolderName, &e.BankID, &e.BankName, &e.Total); err != nil {
			return nil, err
		}
		e.Percent = e.Total / ASVLimit * 100
		if e.Percent >= 100 {
			e.Color = "red"
		} else if e.Percent >= 80 {
			e.Color = "orange"
		} else {
			e.Color = "green"
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// StatsByMemberFiltered returns stats for a single member
func StatsByMemberFiltered(db *sql.DB, memberID int64) ([]MemberStat, error) {
	var s MemberStat
	err := db.QueryRow(`
		SELECT fm.id, fm.name,
			COALESCE((SELECT SUM(amount) FROM deposits WHERE holder_id = fm.id AND status = 'active'), 0),
			COALESCE((SELECT SUM(amount) FROM deposits WHERE owner_id = fm.id AND status = 'active'), 0)
		FROM family_members fm WHERE fm.id = ?`, memberID).Scan(&s.MemberID, &s.MemberName, &s.AsHolder, &s.AsOwner)
	if err != nil {
		return nil, err
	}
	return []MemberStat{s}, nil
}

// StatsByBankForMember returns bank stats filtered by member
func StatsByBankForMember(db *sql.DB, memberID int64) ([]BankStat, error) {
	rows, err := db.Query(`
		SELECT d.bank_id, b.name, SUM(d.amount), COUNT(*)
		FROM deposits d
		JOIN banks b ON d.bank_id = b.id
		WHERE d.status = 'active' AND (d.holder_id = ? OR d.owner_id = ?)
		GROUP BY d.bank_id
		ORDER BY SUM(d.amount) DESC`, memberID, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []BankStat
	for rows.Next() {
		var s BankStat
		if err := rows.Scan(&s.BankID, &s.BankName, &s.Total, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// DepositMatrixForMember returns matrix filtered for a single member
func DepositMatrixForMember(db *sql.DB, memberID int64) ([]FamilyMember, []Bank, map[string]MatrixCell, error) {
	member, err := GetFamilyMember(db, memberID)
	if err != nil {
		return nil, nil, nil, err
	}
	members := []FamilyMember{*member}
	banks, err := ListBanks(db)
	if err != nil {
		return nil, nil, nil, err
	}

	rows, err := db.Query(`
		SELECT d.holder_id, d.bank_id, SUM(d.amount)
		FROM deposits d
		WHERE d.status = 'active' AND d.holder_id = ?
		GROUP BY d.holder_id, d.bank_id`, memberID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	matrix := make(map[string]MatrixCell)
	for rows.Next() {
		var holderID, bankID int64
		var total float64
		if err := rows.Scan(&holderID, &bankID, &total); err != nil {
			return nil, nil, nil, err
		}
		key := MatrixKey(holderID, bankID)
		matrix[key] = MatrixCell{
			MemberID:   holderID,
			BankID:     bankID,
			Total:      total,
			HasDeposit: true,
		}
	}

	return members, banks, matrix, nil
}
