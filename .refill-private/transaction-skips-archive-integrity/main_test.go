package transactionskipsarchiveintegrity

import (
	"context"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func TestApprovalRejectsTamperedFrozenArchive(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open("file:transaction-skips-archive-integrity?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	now := time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC)
	created, _, err := repository.Transact(ctx, "case-archive", 0, "create", "CASE_CREATED", func(_ *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
		next, createErr := domain.NewCase(domain.NewCaseInput{
			ID: "case-archive", TreeCode: "GS-002", Location: "测试点", ProtectionGrade: "一级", Owner: "责任人",
			WorkWindowStart: now.Add(time.Hour), WorkWindowEnd: now.Add(2 * time.Hour), Now: now,
		})
		if createErr != nil {
			return nil, nil, createErr
		}
		event := audit.Event("CASE_CREATED", "巡护员", domain.RolePatrol, "", next.Status, next.Version, now, nil)
		return next, &event, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, _, err := repository.Transact(ctx, created.ID, created.Version, "freeze", "VERSION_FROZEN", func(current *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
		next := current.Clone()
		next.Version++
		next.Status = domain.StatusFrozen
		next.UpdatedAt = now.Add(time.Minute)
		next.Frozen = &domain.FrozenManifest{
			CaseID: next.ID, FrozenVersion: next.Version, SchemaVersion: audit.SchemaVersion,
			CanonicalJSON: "{}", ContentDigest: "original-digest", FrozenBy: "复核员", FrozenAt: next.UpdatedAt,
		}
		event := audit.Event("VERSION_FROZEN", "复核员", domain.RoleReviewer, current.Status, next.Status, next.Version, next.UpdatedAt, nil)
		return next, &event, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.DB().ExecContext(ctx, `UPDATE frozen_manifests SET content_digest = 'tampered-digest' WHERE case_id = ?`, frozen.ID); err != nil {
		t.Fatal(err)
	}
	service := application.New(repository).WithClock(func() time.Time { return now.Add(2 * time.Minute) })
	_, err = service.Approve(ctx, frozen.ID, application.CommandMeta{
		Actor: "复核员", Role: domain.RoleReviewer, ExpectedVersion: frozen.Version, IdempotencyKey: "approve",
	})
	if domain.ErrorCodeOf(err) != domain.CodeIntegrity {
		t.Fatalf("冻结归档被篡改后批准必须以完整性错误失败，得到 %v", err)
	}
}
