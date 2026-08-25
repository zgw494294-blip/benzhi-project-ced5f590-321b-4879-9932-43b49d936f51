package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

type Mutation func(current *domain.RestorationCase, nextPermitSerial int64) (*domain.RestorationCase, *domain.AuditEvent, error)

func scanCase(row interface{ Scan(...any) error }) (*domain.RestorationCase, error) {
	var encoded []byte
	if err := row.Scan(&encoded); err != nil {
		return nil, err
	}
	var restoration domain.RestorationCase
	if err := json.Unmarshal(encoded, &restoration); err != nil {
		return nil, fmt.Errorf("解析作业聚合: %w", err)
	}
	if err := restoration.ValidateLinks(); err != nil {
		return nil, fmt.Errorf("恢复聚合关联: %w", err)
	}
	return &restoration, nil
}

func (s *Store) Get(ctx context.Context, caseID string) (*domain.RestorationCase, error) {
	restoration, err := scanCase(s.db.QueryRowContext(ctx, `SELECT aggregate_json FROM restoration_cases WHERE id = ?`, caseID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewError(domain.CodeNotFound, "作业档案不存在")
	}
	if err == nil {
		err = s.verifyAppendOnlyRecords(ctx, restoration)
	}
	return restoration, err
}

func (s *Store) List(ctx context.Context) ([]*domain.RestorationCase, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT aggregate_json FROM restoration_cases ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.RestorationCase
	for rows.Next() {
		restoration, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, restoration)
	}
	return result, rows.Err()
}

func (s *Store) Transact(ctx context.Context, caseID string, expectedVersion int64, idempotencyKey, operation string, mutation Mutation) (*domain.RestorationCase, bool, error) {
	if idempotencyKey == "" {
		return nil, false, domain.NewError(domain.CodeValidation, "idempotencyKey 不能为空")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var cached []byte
	err = tx.QueryRowContext(ctx, `SELECT response_json FROM idempotency_records WHERE case_id = ? AND idempotency_key = ?`, caseID, idempotencyKey).Scan(&cached)
	if err == nil {
		var result domain.RestorationCase
		if err := json.Unmarshal(cached, &result); err != nil {
			return nil, false, err
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return &result, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	var current *domain.RestorationCase
	current, err = scanCase(tx.QueryRowContext(ctx, `SELECT aggregate_json FROM restoration_cases WHERE id = ?`, caseID))
	if errors.Is(err, sql.ErrNoRows) {
		current = nil
	} else if err != nil {
		return nil, false, err
	}
	actualVersion := int64(0)
	if current != nil {
		actualVersion = current.Version
	}
	if actualVersion != expectedVersion {
		return nil, false, domain.NewError(domain.CodeConflict, "版本冲突：期望 %d，实际 %d", expectedVersion, actualVersion)
	}
	s.permitSerialMu.Lock()
	nextSerial := s.nextPermitSerial
	defer s.permitSerialMu.Unlock()
	next, event, err := mutation(current, nextSerial)
	if err != nil {
		return nil, false, err
	}
	if next == nil || next.ID != caseID {
		return nil, false, fmt.Errorf("事务返回了无效聚合")
	}
	encoded, err := json.Marshal(next)
	if err != nil {
		return nil, false, err
	}
	if current == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO restoration_cases(id, version, status, aggregate_json, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)`,
			next.ID, next.Version, next.Status, encoded, next.CreatedAt.Format(time.RFC3339Nano), next.UpdatedAt.Format(time.RFC3339Nano))
	} else {
		result, updateErr := tx.ExecContext(ctx, `UPDATE restoration_cases SET version = ?, status = ?, aggregate_json = ?, updated_at = ? WHERE id = ? AND version = ?`,
			next.Version, next.Status, encoded, next.UpdatedAt.Format(time.RFC3339Nano), next.ID, expectedVersion)
		if updateErr != nil {
			err = updateErr
		} else if affected, _ := result.RowsAffected(); affected != 1 {
			err = domain.NewError(domain.CodeConflict, "并发写入冲突")
		}
	}
	if err != nil {
		return nil, false, err
	}
	if event == nil {
		return nil, false, fmt.Errorf("写事务缺少审计事件")
	}
	event.CaseID, event.Version = next.ID, next.Version
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return nil, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id, case_id, event_type, aggregate_version, event_json, occurred_at) VALUES(?, ?, ?, ?, ?, ?)`,
		event.ID, next.ID, event.Type, next.Version, eventJSON, event.OccurredAt.Format(time.RFC3339Nano)); err != nil {
		return nil, false, err
	}
	if next.Frozen != nil && (current == nil || current.Frozen == nil) {
		manifestJSON, _ := json.Marshal(next.Frozen)
		if _, err := tx.ExecContext(ctx, `INSERT INTO frozen_manifests(case_id, frozen_version, content_digest, manifest_json, created_at) VALUES(?, ?, ?, ?, ?)`,
			next.ID, next.Frozen.FrozenVersion, next.Frozen.ContentDigest, manifestJSON, next.Frozen.FrozenAt.Format(time.RFC3339Nano)); err != nil {
			return nil, false, fmt.Errorf("追加冻结清单: %w", err)
		}
	}
	if next.Permit != nil && (current == nil || current.Permit == nil) {
		permitJSON, _ := json.Marshal(next.Permit)
		if _, err := tx.ExecContext(ctx, `INSERT INTO release_permits(serial_number, permit_id, case_id, frozen_version, content_digest, permit_json, issued_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
			next.Permit.SerialNumber, next.Permit.ID, next.ID, next.Permit.FrozenVersion, next.Permit.ContentDigest, permitJSON, next.Permit.IssuedAt.Format(time.RFC3339Nano)); err != nil {
			return nil, false, fmt.Errorf("签发放行凭据: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency_records(case_id, idempotency_key, operation, response_json, created_at) VALUES(?, ?, ?, ?, ?)`,
		caseID, idempotencyKey, operation, encoded, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	if next.Permit != nil && (current == nil || current.Permit == nil) {
		s.nextPermitSerial++
	}
	return next, false, nil
}

func (s *Store) AuditTrail(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_json FROM audit_events WHERE case_id = ? ORDER BY sequence`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.AuditEvent
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		var event domain.AuditEvent
		if err := json.Unmarshal(encoded, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
