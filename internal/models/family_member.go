package models

import (
	"database/sql"
	"fmt"
)

type FamilyMember struct {
	ID       int64
	Name     string
	Username string // пустой, если нет аккаунта
	HasLogin bool
}

func ListFamilyMembers(db *sql.DB) ([]FamilyMember, error) {
	rows, err := db.Query(`
		SELECT fm.id, fm.name, COALESCE(u.username, '')
		FROM family_members fm
		LEFT JOIN users u ON u.family_member_id = fm.id
		ORDER BY fm.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []FamilyMember
	for rows.Next() {
		var m FamilyMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Username); err != nil {
			return nil, err
		}
		m.HasLogin = m.Username != ""
		members = append(members, m)
	}
	return members, nil
}

func GetFamilyMember(db *sql.DB, id int64) (*FamilyMember, error) {
	m := &FamilyMember{}
	err := db.QueryRow("SELECT id, name FROM family_members WHERE id = ?", id).Scan(&m.ID, &m.Name)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func CreateFamilyMember(db *sql.DB, name string) (int64, error) {
	res, err := db.Exec("INSERT INTO family_members (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateFamilyMember(db *sql.DB, id int64, name string) error {
	_, err := db.Exec("UPDATE family_members SET name = ? WHERE id = ?", name, id)
	return err
}

func DeleteFamilyMember(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM family_members WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("невозможно удалить: есть связанные вклады")
	}
	return nil
}
