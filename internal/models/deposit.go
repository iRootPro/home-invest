package models

import (
	"database/sql"
	"math"
	"time"
)

type Deposit struct {
	ID                int64
	BankID            int64
	HolderID          int64
	OwnerID           int64
	Amount            float64
	InterestRate      float64
	OpenDate          string
	EndDate           string
	HasCapitalization bool
	IsReplenishable   bool
	Status            string
	Notes             string
	ClosedAmount      *float64
	// Joined fields
	BankName   string
	BankLogo   string
	HolderName string
	OwnerName  string
}

// HasClosedAmount returns true if the deposit was closed with a recorded amount.
func (d Deposit) HasClosedAmount() bool {
	return d.ClosedAmount != nil
}

// ClosedAmountVal returns the closed amount value (0 if not set).
func (d Deposit) ClosedAmountVal() float64 {
	if d.ClosedAmount != nil {
		return *d.ClosedAmount
	}
	return 0
}

// ActualProfit returns the actual profit (closed_amount - amount).
func (d Deposit) ActualProfit() float64 {
	if d.ClosedAmount != nil {
		return *d.ClosedAmount - d.Amount
	}
	return 0
}

// TotalProfit returns the estimated profit for the entire deposit term.
func (d Deposit) TotalProfit() float64 {
	from := parseDate(d.OpenDate)
	to := parseDate(d.EndDate)
	if from.IsZero() || to.IsZero() {
		return 0
	}
	return calcProfit(d.Amount, d.InterestRate, from, to, d.HasCapitalization)
}

func parseDate(s string) time.Time {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func calcProfit(amount, rate float64, from, to time.Time, capitalization bool) float64 {
	if amount <= 0 || rate <= 0 || !to.After(from) {
		return 0
	}
	if !capitalization {
		days := to.Sub(from).Hours() / 24
		return amount * rate / 100 * days / 365
	}
	months := monthsBetween(from, to)
	monthlyRate := rate / 12 / 100
	result := amount * math.Pow(1+monthlyRate, float64(months))
	// remaining days after full months
	afterMonths := from.AddDate(0, months, 0)
	remainDays := to.Sub(afterMonths).Hours() / 24
	if remainDays > 0 {
		result = result * (1 + rate/100*remainDays/365)
	}
	return result - amount
}

func monthsBetween(from, to time.Time) int {
	months := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
	if to.Day() < from.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}

type DepositFilter struct {
	BankID   int64
	HolderID int64
	OwnerID  int64
	Status   string // "active", "closed", "" (all)
	MemberID int64  // if > 0, filter by (holder_id = ? OR owner_id = ?)
}

func ListDeposits(db *sql.DB, f DepositFilter) ([]Deposit, error) {
	query := `SELECT d.id, d.bank_id, d.holder_id, d.owner_id,
		d.amount, d.interest_rate, d.open_date, d.end_date,
		d.has_capitalization, d.is_replenishable, d.status, d.notes, d.closed_amount,
		b.name, b.logo, h.name, o.name
		FROM deposits d
		JOIN banks b ON d.bank_id = b.id
		JOIN family_members h ON d.holder_id = h.id
		JOIN family_members o ON d.owner_id = o.id
		WHERE 1=1`
	var args []any

	if f.BankID > 0 {
		query += " AND d.bank_id = ?"
		args = append(args, f.BankID)
	}
	if f.HolderID > 0 {
		query += " AND d.holder_id = ?"
		args = append(args, f.HolderID)
	}
	if f.OwnerID > 0 {
		query += " AND d.owner_id = ?"
		args = append(args, f.OwnerID)
	}
	if f.Status != "" {
		query += " AND d.status = ?"
		args = append(args, f.Status)
	}
	if f.MemberID > 0 {
		query += " AND (d.holder_id = ? OR d.owner_id = ?)"
		args = append(args, f.MemberID, f.MemberID)
	}
	query += " ORDER BY d.status ASC, d.end_date ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deposits []Deposit
	for rows.Next() {
		var d Deposit
		if err := rows.Scan(
			&d.ID, &d.BankID, &d.HolderID, &d.OwnerID,
			&d.Amount, &d.InterestRate, &d.OpenDate, &d.EndDate,
			&d.HasCapitalization, &d.IsReplenishable, &d.Status, &d.Notes, &d.ClosedAmount,
			&d.BankName, &d.BankLogo, &d.HolderName, &d.OwnerName,
		); err != nil {
			return nil, err
		}
		deposits = append(deposits, d)
	}
	return deposits, nil
}

func GetDeposit(db *sql.DB, id int64) (*Deposit, error) {
	d := &Deposit{}
	err := db.QueryRow(`SELECT d.id, d.bank_id, d.holder_id, d.owner_id,
		d.amount, d.interest_rate, d.open_date, d.end_date,
		d.has_capitalization, d.is_replenishable, d.status, d.notes, d.closed_amount,
		b.name, b.logo, h.name, o.name
		FROM deposits d
		JOIN banks b ON d.bank_id = b.id
		JOIN family_members h ON d.holder_id = h.id
		JOIN family_members o ON d.owner_id = o.id
		WHERE d.id = ?`, id).Scan(
		&d.ID, &d.BankID, &d.HolderID, &d.OwnerID,
		&d.Amount, &d.InterestRate, &d.OpenDate, &d.EndDate,
		&d.HasCapitalization, &d.IsReplenishable, &d.Status, &d.Notes,
		&d.BankName, &d.BankLogo, &d.HolderName, &d.OwnerName,
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func CreateDeposit(db *sql.DB, d *Deposit) (int64, error) {
	res, err := db.Exec(`INSERT INTO deposits
		(bank_id, holder_id, owner_id, amount, interest_rate,
		 open_date, end_date, has_capitalization, is_replenishable, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.BankID, d.HolderID, d.OwnerID, d.Amount, d.InterestRate,
		d.OpenDate, d.EndDate, boolToInt(d.HasCapitalization), boolToInt(d.IsReplenishable), d.Notes,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateDeposit(db *sql.DB, d *Deposit) error {
	_, err := db.Exec(`UPDATE deposits SET
		bank_id = ?, holder_id = ?, owner_id = ?, amount = ?, interest_rate = ?,
		open_date = ?, end_date = ?, has_capitalization = ?, is_replenishable = ?,
		notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		d.BankID, d.HolderID, d.OwnerID, d.Amount, d.InterestRate,
		d.OpenDate, d.EndDate, boolToInt(d.HasCapitalization), boolToInt(d.IsReplenishable),
		d.Notes, d.ID,
	)
	return err
}

func CloseDeposit(db *sql.DB, id int64, closedAmount float64) error {
	_, err := db.Exec("UPDATE deposits SET status = 'closed', closed_amount = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", closedAmount, id)
	return err
}

// ExpiringDepositsForMember returns expiring deposits filtered by member
func ExpiringDepositsForMember(db *sql.DB, days int, memberID int64) ([]Deposit, error) {
	today := time.Now().Format("2006-01-02")
	deadline := time.Now().AddDate(0, 0, days).Format("2006-01-02")
	query := `SELECT d.id, d.bank_id, d.holder_id, d.owner_id,
		d.amount, d.interest_rate, d.open_date, d.end_date,
		d.has_capitalization, d.is_replenishable, d.status, d.notes, d.closed_amount,
		b.name, b.logo, h.name, o.name
		FROM deposits d
		JOIN banks b ON d.bank_id = b.id
		JOIN family_members h ON d.holder_id = h.id
		JOIN family_members o ON d.owner_id = o.id
		WHERE d.status = 'active' AND d.end_date >= ? AND d.end_date <= ?
		AND (d.holder_id = ? OR d.owner_id = ?)
		ORDER BY d.end_date ASC`
	rows, err := db.Query(query, today, deadline, memberID, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deposits []Deposit
	for rows.Next() {
		var d Deposit
		if err := rows.Scan(
			&d.ID, &d.BankID, &d.HolderID, &d.OwnerID,
			&d.Amount, &d.InterestRate, &d.OpenDate, &d.EndDate,
			&d.HasCapitalization, &d.IsReplenishable, &d.Status, &d.Notes, &d.ClosedAmount,
			&d.BankName, &d.BankLogo, &d.HolderName, &d.OwnerName,
		); err != nil {
			return nil, err
		}
		deposits = append(deposits, d)
	}
	return deposits, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ExpiringDeposits returns active deposits ending within given days
func ExpiringDeposits(db *sql.DB, days int) ([]Deposit, error) {
	today := time.Now().Format("2006-01-02")
	deadline := time.Now().AddDate(0, 0, days).Format("2006-01-02")
	query := `SELECT d.id, d.bank_id, d.holder_id, d.owner_id,
		d.amount, d.interest_rate, d.open_date, d.end_date,
		d.has_capitalization, d.is_replenishable, d.status, d.notes, d.closed_amount,
		b.name, b.logo, h.name, o.name
		FROM deposits d
		JOIN banks b ON d.bank_id = b.id
		JOIN family_members h ON d.holder_id = h.id
		JOIN family_members o ON d.owner_id = o.id
		WHERE d.status = 'active' AND d.end_date >= ? AND d.end_date <= ?
		ORDER BY d.end_date ASC`
	rows, err := db.Query(query, today, deadline)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deposits []Deposit
	for rows.Next() {
		var d Deposit
		if err := rows.Scan(
			&d.ID, &d.BankID, &d.HolderID, &d.OwnerID,
			&d.Amount, &d.InterestRate, &d.OpenDate, &d.EndDate,
			&d.HasCapitalization, &d.IsReplenishable, &d.Status, &d.Notes, &d.ClosedAmount,
			&d.BankName, &d.BankLogo, &d.HolderName, &d.OwnerName,
		); err != nil {
			return nil, err
		}
		deposits = append(deposits, d)
	}
	return deposits, nil
}
