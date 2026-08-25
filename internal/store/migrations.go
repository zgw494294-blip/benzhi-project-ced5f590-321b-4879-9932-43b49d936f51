package store

import (
	"context"
	"fmt"
)

const schemaVersion = 1

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS schema_meta (
        version INTEGER NOT NULL,
        applied_at TEXT NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS restoration_cases (
        id TEXT PRIMARY KEY,
        version INTEGER NOT NULL CHECK(version >= 1),
        status TEXT NOT NULL,
        aggregate_json BLOB NOT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS audit_events (
        sequence INTEGER PRIMARY KEY AUTOINCREMENT,
        id TEXT NOT NULL UNIQUE,
        case_id TEXT NOT NULL REFERENCES restoration_cases(id),
        event_type TEXT NOT NULL,
        aggregate_version INTEGER NOT NULL,
        event_json BLOB NOT NULL,
        occurred_at TEXT NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS frozen_manifests (
        case_id TEXT NOT NULL REFERENCES restoration_cases(id),
        frozen_version INTEGER NOT NULL,
        content_digest TEXT NOT NULL,
        manifest_json BLOB NOT NULL,
        created_at TEXT NOT NULL,
        PRIMARY KEY(case_id, frozen_version)
    )`,
	`CREATE TABLE IF NOT EXISTS release_permits (
        serial_number INTEGER PRIMARY KEY AUTOINCREMENT,
        permit_id TEXT NOT NULL UNIQUE,
        case_id TEXT NOT NULL REFERENCES restoration_cases(id),
        frozen_version INTEGER NOT NULL,
        content_digest TEXT NOT NULL,
        permit_json BLOB NOT NULL,
        issued_at TEXT NOT NULL,
        UNIQUE(case_id, frozen_version)
    )`,
	`CREATE TABLE IF NOT EXISTS idempotency_records (
        case_id TEXT NOT NULL,
        idempotency_key TEXT NOT NULL,
        operation TEXT NOT NULL,
        response_json BLOB NOT NULL,
        created_at TEXT NOT NULL,
        PRIMARY KEY(case_id, idempotency_key)
    )`,
	`CREATE INDEX IF NOT EXISTS idx_audit_case_sequence ON audit_events(case_id, sequence)`,
}

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始迁移事务: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range migrations {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("执行数据库迁移: %w", err)
		}
	}
	var current int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_meta`).Scan(&current)
	if err != nil {
		return fmt.Errorf("读取 schemaVersion: %w", err)
	}
	if current > schemaVersion {
		return fmt.Errorf("数据库 schemaVersion %d 高于程序支持版本 %d", current, schemaVersion)
	}
	if current < schemaVersion {
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_meta(version, applied_at) VALUES(?, datetime('now'))`, schemaVersion); err != nil {
			return fmt.Errorf("记录 schemaVersion: %w", err)
		}
	}
	return tx.Commit()
}
