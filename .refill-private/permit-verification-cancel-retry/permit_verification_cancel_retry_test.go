package permit_verification_cancel_retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func TestPermitVerificationHonorsCanceledContext(t *testing.T) {
	repository, err := store.Open("file:permit-cancel-retry?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	created := transact(t, repository, "case-cancel", 0, "create", func(_ *domain.RestorationCase, _ int64) *domain.RestorationCase {
		restoration, createErr := domain.NewCase(domain.NewCaseInput{
			ID: "case-cancel", TreeCode: "GS-CANCEL", Location: "北侧保护区", ProtectionGrade: "一级", Owner: "巡护组",
			WorkWindowStart: now.Add(-time.Hour), WorkWindowEnd: now.Add(time.Hour), Now: now,
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return restoration
	})
	frozen := transact(t, repository, created.ID, created.Version, "freeze", func(current *domain.RestorationCase, _ int64) *domain.RestorationCase {
		next := current.Clone()
		next.Version++
		next.Status = domain.StatusFrozen
		next.UpdatedAt = now.Add(time.Minute)
		next.Frozen = &domain.FrozenManifest{
			CaseID: next.ID, FrozenVersion: next.Version, ContentDigest: "digest-cancel", CanonicalJSON: "{}",
			SchemaVersion: audit.SchemaVersion, FrozenBy: "复核员", FrozenAt: next.UpdatedAt,
		}
		return next
	})
	released := transact(t, repository, frozen.ID, frozen.Version, "release", func(current *domain.RestorationCase, serial int64) *domain.RestorationCase {
		next := current.Clone()
		next.Version++
		next.Status = domain.StatusReleased
		next.UpdatedAt = now.Add(2 * time.Minute)
		next.Permit = &domain.ReleasePermit{
			ID: "permit-cancel", CaseID: next.ID, SerialNumber: serial, FrozenVersion: next.Frozen.FrozenVersion,
			ContentDigest: next.Frozen.ContentDigest, ApprovedBy: "复核负责人", IssuedAt: next.UpdatedAt, SchemaVersion: audit.SchemaVersion,
		}
		return next
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = application.New(repository).VerifyPermitBySerial(ctx, released.Permit.SerialNumber)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的核验请求必须传播 context.Canceled，得到 %v", err)
	}
}

func transact(t *testing.T, repository *store.Store, caseID string, expectedVersion int64, key string, mutate func(*domain.RestorationCase, int64) *domain.RestorationCase) *domain.RestorationCase {
	t.Helper()
	next, _, err := repository.Transact(context.Background(), caseID, expectedVersion, key, key, func(current *domain.RestorationCase, serial int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
		result := mutate(current, serial)
		event := audit.Event(key, "测试操作者", domain.RoleReviewer, "", result.Status, result.Version, result.UpdatedAt, nil)
		return result, &event, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return next
}
