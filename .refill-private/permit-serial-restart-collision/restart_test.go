package permitserialrestartcollision

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func createCase(t *testing.T, repository *store.Store, id string, now time.Time) *domain.RestorationCase {
	t.Helper()
	restoration, _, err := repository.Transact(context.Background(), id, 0, "create-"+id, "CASE_CREATED",
		func(_ *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			next, createErr := domain.NewCase(domain.NewCaseInput{
				ID: id, TreeCode: "GS-" + id, Location: "重启复现点", ProtectionGrade: "一级", Owner: "责任人",
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
	return restoration
}

func freezeCase(t *testing.T, repository *store.Store, restoration *domain.RestorationCase, now time.Time) *domain.RestorationCase {
	t.Helper()
	frozen, _, err := repository.Transact(context.Background(), restoration.ID, restoration.Version, "freeze-"+restoration.ID, "VERSION_FROZEN",
		func(current *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			next := current.Clone()
			next.Version++
			next.Status = domain.StatusFrozen
			next.UpdatedAt = now
			next.Frozen = &domain.FrozenManifest{
				CaseID: next.ID, FrozenVersion: next.Version, SchemaVersion: audit.SchemaVersion,
				CanonicalJSON: "{}", ContentDigest: "digest-" + next.ID, FrozenBy: "复核员", FrozenAt: now,
			}
			event := audit.Event("VERSION_FROZEN", "复核员", domain.RoleReviewer, current.Status, next.Status, next.Version, now, nil)
			return next, &event, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	return frozen
}

func releaseCase(repository *store.Store, restoration *domain.RestorationCase, now time.Time) (*domain.RestorationCase, error) {
	released, _, err := repository.Transact(context.Background(), restoration.ID, restoration.Version, "release-"+restoration.ID, "PERMIT_ISSUED",
		func(current *domain.RestorationCase, serial int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			next := current.Clone()
			next.Version++
			next.Status = domain.StatusReleased
			next.UpdatedAt = now
			next.Permit = &domain.ReleasePermit{
				ID: "permit-" + next.ID, CaseID: next.ID, SerialNumber: serial,
				FrozenVersion: next.Frozen.FrozenVersion, ContentDigest: next.Frozen.ContentDigest,
				ApprovedBy: "批准人", IssuedAt: now, SchemaVersion: audit.SchemaVersion,
			}
			event := audit.Event("PERMIT_ISSUED", "批准人", domain.RoleReviewer, current.Status, next.Status, next.Version, now, nil)
			return next, &event, nil
		})
	return released, err
}

func TestPermitSerialContinuesAfterStoreRestart(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "restart.db")
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)

	firstStore, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	firstFrozen := freezeCase(t, firstStore, createCase(t, firstStore, "case-before-restart", now), now.Add(time.Minute))
	firstReleased, err := releaseCase(firstStore, firstFrozen, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if firstReleased.Permit.SerialNumber != 1 {
		t.Fatalf("首张凭据编号应为 1，得到 %d", firstReleased.Permit.SerialNumber)
	}
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}

	secondStore, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })
	secondFrozen := freezeCase(t, secondStore, createCase(t, secondStore, "case-after-restart", now.Add(3*time.Minute)), now.Add(4*time.Minute))
	secondReleased, err := releaseCase(secondStore, secondFrozen, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("重启后的新作业批准应继续分配序号：%v", err)
	}
	if secondReleased.Permit.SerialNumber != 2 {
		t.Fatalf("重启后的凭据编号应为 2，得到 %d", secondReleased.Permit.SerialNumber)
	}
}
