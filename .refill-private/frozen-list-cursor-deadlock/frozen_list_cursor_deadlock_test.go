package frozenlistcursordeadlock_test

import (
	"context"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func TestFrozenCaseListDoesNotWaitOnItsOwnCursor(t *testing.T) {
	repository, err := store.Open("file:frozen-list-cursor-deadlock?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	created, _, err := repository.Transact(context.Background(), "case-frozen-list", 0, "create", "CASE_CREATED",
		func(_ *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			next, createErr := domain.NewCase(domain.NewCaseInput{
				ID: "case-frozen-list", TreeCode: "GS-LIST-001", Location: "测试样地", ProtectionGrade: "一级", Owner: "责任人",
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

	frozenAt := now.Add(time.Minute)
	_, _, err = repository.Transact(context.Background(), created.ID, created.Version, "freeze", "VERSION_FROZEN",
		func(current *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			next := current.Clone()
			next.Version++
			next.Status = domain.StatusFrozen
			next.UpdatedAt = frozenAt
			next.Frozen = &domain.FrozenManifest{
				CaseID: next.ID, FrozenVersion: next.Version, SchemaVersion: audit.SchemaVersion,
				CanonicalJSON: "{}", ContentDigest: "stable-list-digest", FrozenBy: "复核员", FrozenAt: frozenAt,
			}
			event := audit.Event("VERSION_FROZEN", "复核员", domain.RoleReviewer, current.Status, next.Status, next.Version, frozenAt, nil)
			return next, &event, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	items, err := repository.List(ctx)
	if err != nil {
		t.Fatalf("冻结作业列表不应等待自身尚未关闭的查询游标：%v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("冻结作业列表结果错误：%+v", items)
	}
}
