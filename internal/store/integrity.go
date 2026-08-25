package store

import (
	"context"
	"database/sql"
	"fmt"
)

type IntegrityStatus struct {
	SchemaVersion int    `json:"schemaVersion"`
	JournalMode   string `json:"journalMode"`
	ForeignKeys   bool   `json:"foreignKeys"`
	QuickCheck    string `json:"quickCheck"`
}

func (s *Store) Integrity(ctx context.Context) (IntegrityStatus, error) {
	var status IntegrityStatus
	var foreignKeys int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_meta`).Scan(&status.SchemaVersion); err != nil {
		return status, err
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&status.JournalMode); err != nil {
		return status, err
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		return status, err
	}
	status.ForeignKeys = foreignKeys == 1
	if err := s.db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&status.QuickCheck); err != nil && err != sql.ErrNoRows {
		return status, err
	}
	if status.SchemaVersion != schemaVersion || !status.ForeignKeys || status.QuickCheck != "ok" {
		return status, fmt.Errorf("SQLite 完整性检查失败: %+v", status)
	}
	return status, nil
}
