package permitverificationcache_test

import (
	"context"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func TestPermitVerificationReloadsInvalidatedArchive(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open("file:permit-verification-cache-stale?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	created, _, err := repository.Transact(ctx, "case-cache", 0, "create-cache-case", "CASE_CREATED",
		func(_ *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			next, createErr := domain.NewCase(domain.NewCaseInput{
				ID: "case-cache", TreeCode: "GS-CACHE", Location: "缓存核验点", ProtectionGrade: "一级", Owner: "巡护组",
				WorkWindowStart: now.Add(-time.Hour), WorkWindowEnd: now.Add(time.Hour), Now: now.Add(-2 * time.Hour),
			})
			if createErr != nil {
				return nil, nil, createErr
			}
			event := audit.Event("CASE_CREATED", "巡护员", domain.RolePatrol, "", next.Status, next.Version, now.Add(-2*time.Hour), nil)
			return next, &event, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	frozen, _, err := repository.Transact(ctx, created.ID, created.Version, "freeze-cache-case", "VERSION_FROZEN",
		func(current *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			next := current.Clone()
			frozenAt := now.Add(-time.Hour)
			manifest, manifestErr := audit.BuildManifest(next, "复核员", frozenAt)
			if manifestErr != nil {
				return nil, nil, manifestErr
			}
			next.Version++
			next.Status = domain.StatusFrozen
			next.UpdatedAt = frozenAt
			manifest.FrozenVersion = next.Version
			next.Frozen = &manifest
			event := audit.Event("VERSION_FROZEN", "复核员", domain.RoleReviewer, current.Status, next.Status, next.Version, frozenAt, nil)
			return next, &event, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	released, _, err := repository.Transact(ctx, frozen.ID, frozen.Version, "release-cache-case", "PERMIT_ISSUED",
		func(current *domain.RestorationCase, serial int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			next := current.Clone()
			issuedAt := now.Add(-30 * time.Minute)
			releaseErr := next.Release(domain.ReleasePermit{
				ID: "permit-cache", CaseID: next.ID, SerialNumber: serial, FrozenVersion: next.Frozen.FrozenVersion,
				ContentDigest: next.Frozen.ContentDigest, ApprovedBy: "批准人", SchemaVersion: audit.SchemaVersion,
			}, issuedAt)
			if releaseErr != nil {
				return nil, nil, releaseErr
			}
			event := audit.Event("PERMIT_ISSUED", "批准人", domain.RoleReviewer, current.Status, next.Status, next.Version, issuedAt, nil)
			return next, &event, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	service := application.New(repository).WithClock(func() time.Time { return now })
	first, err := service.VerifyPermitBySerial(ctx, released.Permit.SerialNumber)
	if err != nil || !first.Valid {
		t.Fatalf("初次核验应确认未损坏凭据有效，result=%+v err=%v", first, err)
	}

	if _, err := repository.DB().ExecContext(ctx,
		`UPDATE frozen_manifests SET manifest_json = ? WHERE case_id = ? AND frozen_version = ?`,
		[]byte("{"), released.ID, released.Frozen.FrozenVersion); err != nil {
		t.Fatal(err)
	}

	second, err := service.VerifyPermitBySerial(ctx, released.Permit.SerialNumber)
	if err == nil && second.Valid {
		t.Fatalf("归档失效后再次核验仍返回缓存的有效结果：%+v", second)
	}
}
