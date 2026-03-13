package database

import (
	"database/sql"
	"fmt"
	"strings"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS family_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		family_member_id INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (family_member_id) REFERENCES family_members(id) ON DELETE SET NULL
	)`,
	`CREATE TABLE IF NOT EXISTS banks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		notes TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS deposits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bank_id INTEGER NOT NULL,
		holder_id INTEGER NOT NULL,
		owner_id INTEGER NOT NULL,
		amount REAL NOT NULL,
		interest_rate REAL NOT NULL,
		open_date DATE NOT NULL,
		end_date DATE NOT NULL,
		has_capitalization INTEGER DEFAULT 0,
		is_replenishable INTEGER DEFAULT 0,
		status TEXT DEFAULT 'active' CHECK(status IN ('active','closed')),
		notes TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (bank_id) REFERENCES banks(id) ON DELETE RESTRICT,
		FOREIGN KEY (holder_id) REFERENCES family_members(id) ON DELETE RESTRICT,
		FOREIGN KEY (owner_id) REFERENCES family_members(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS wallets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS wallet_members (
		wallet_id INTEGER NOT NULL,
		family_member_id INTEGER NOT NULL,
		PRIMARY KEY (wallet_id, family_member_id),
		FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE CASCADE,
		FOREIGN KEY (family_member_id) REFERENCES family_members(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_deposits_holder_bank_status ON deposits(holder_id, bank_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_deposits_end_date ON deposits(end_date) WHERE status = 'active'`,
	`ALTER TABLE banks ADD COLUMN logo TEXT DEFAULT ''`,
}

func Migrate(db *sql.DB) error {
	for i, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			// ALTER TABLE ADD COLUMN fails if column already exists — safe to ignore
			if !isAlterTableDuplicate(err) {
				return fmt.Errorf("migration %d: %w", i, err)
			}
		}
	}
	return nil
}

func isAlterTableDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}
