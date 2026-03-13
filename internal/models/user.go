package models

import (
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID             int64
	Username       string
	PasswordHash   string
	FamilyMemberID sql.NullInt64
}

func Authenticate(db *sql.DB, username, password string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, username, password_hash, family_member_id FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.FamilyMemberID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}
	return u, nil
}

func EnsureDefaultUser(db *sql.DB, username, password string) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hash))
	return err
}

// CreateUserForMember creates a login for a family member
func CreateUserForMember(db *sql.DB, familyMemberID int64, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"INSERT INTO users (username, password_hash, family_member_id) VALUES (?, ?, ?)",
		username, string(hash), familyMemberID,
	)
	return err
}

// UpdateUserPassword changes password for the user linked to a family member
func UpdateUserPassword(db *sql.DB, familyMemberID int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"UPDATE users SET password_hash = ? WHERE family_member_id = ?",
		string(hash), familyMemberID,
	)
	return err
}

// DeleteUserForMember removes login for a family member
func DeleteUserForMember(db *sql.DB, familyMemberID int64) error {
	_, err := db.Exec("DELETE FROM users WHERE family_member_id = ?", familyMemberID)
	return err
}

// GetUserByMemberID returns the user linked to a family member, if any
func GetUserByMemberID(db *sql.DB, familyMemberID int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, username, password_hash, family_member_id FROM users WHERE family_member_id = ?",
		familyMemberID,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.FamilyMemberID)
	if err != nil {
		return nil, err
	}
	return u, nil
}
