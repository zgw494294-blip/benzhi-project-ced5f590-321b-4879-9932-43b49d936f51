package store

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaVersion = 2

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
        request_digest TEXT NOT NULL DEFAULT '',
        response_json BLOB NOT NULL,
        created_at TEXT NOT NULL,
        PRIMARY KEY(case_id, idempotency_key)
    )`,
	`CREATE INDEX IF NOT EXISTS idx_audit_case_sequence ON audit_events(case_id, sequence)`,
}

// ensureColumn adds a column to a table when it is missing, so that databases
// created before the column was part of the CREATE TABLE statement are upgraded
// in place without rebuilding the table. It is idempotent and safe to run on
// every migration.
func ensureColumn(ctx context.Context, tx *sql.Tx, table, column, definition string) error {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM pragma_table_info(?) WHERE name = ?`, table, column)
	if err != nil {
		return fmt.Errorf("检查列 %s.%s 是否存在: %w", table, column, err)
	}
	exists := rows.Next()
	if err := rows.Close(); err != nil {
		return fmt.Errorf("关闭列检查游标: %w", err)
	}
	if exists {
		return nil
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition)); err != nil {
		return fmt.Errorf("为 %s 添加列 %s: %w", table, column, err)
	}
	return nil
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
	if err := ensureColumn(ctx, tx, "idempotency_records", "request_digest", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
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
