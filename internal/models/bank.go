package models

import (
	"database/sql"
	"fmt"
)

type Bank struct {
	ID    int64
	Name  string
	Notes string
	Logo  string
}

func ListBanks(db *sql.DB) ([]Bank, error) {
	rows, err := db.Query("SELECT id, name, notes, logo FROM banks ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var banks []Bank
	for rows.Next() {
		var b Bank
		if err := rows.Scan(&b.ID, &b.Name, &b.Notes, &b.Logo); err != nil {
			return nil, err
		}
		banks = append(banks, b)
	}
	return banks, nil
}

func GetBank(db *sql.DB, id int64) (*Bank, error) {
	b := &Bank{}
	err := db.QueryRow("SELECT id, name, notes, logo FROM banks WHERE id = ?", id).Scan(&b.ID, &b.Name, &b.Notes, &b.Logo)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func CreateBank(db *sql.DB, name, notes string) (int64, error) {
	res, err := db.Exec("INSERT INTO banks (name, notes) VALUES (?, ?)", name, notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateBank(db *sql.DB, id int64, name, notes string) error {
	_, err := db.Exec("UPDATE banks SET name = ?, notes = ? WHERE id = ?", name, notes, id)
	return err
}

func UpdateBankLogo(db *sql.DB, id int64, filename string) error {
	_, err := db.Exec("UPDATE banks SET logo = ? WHERE id = ?", filename, id)
	return err
}

func DeleteBank(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM banks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("невозможно удалить банк: есть связанные вклады")
	}
	return nil
}
