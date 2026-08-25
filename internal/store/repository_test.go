package store

import (
	"context"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	repository, err := Open("file:" + audit.NewID("storetest") + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return repository
}

func createStoredCase(t *testing.T, repository *Store, id string) *domain.RestorationCase {
	t.Helper()
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	created, _, err := repository.Transact(context.Background(), id, 0, "create-"+id, "CASE_CREATED", func(current *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
		next, err := domain.NewCase(domain.NewCaseInput{
			ID: id, TreeCode: "GS-" + id, Location: "测试点", ProtectionGrade: "一级", Owner: "责任人",
			WorkWindowStart: now.Add(time.Hour), WorkWindowEnd: now.Add(2 * time.Hour), Now: now,
		})
		if err != nil {
			return nil, nil, err
		}
		event := audit.Event("CASE_CREATED", "测试员", domain.RolePatrol, "", next.Status, next.Version, now, nil)
		return next, &event, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func TestTransactIdempotencyAndExpectedVersion(t *testing.T) {
	repository := openTestStore(t)
	created := createStoredCase(t, repository, "case-one")
	called := false
	cached, replay, err := repository.Transact(context.Background(), created.ID, 0, "create-"+created.ID, "CASE_CREATED", func(*domain.RestorationCase, int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
		called = true
		return nil, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replay || called || cached.Version != created.Version {
		t.Fatal("相同幂等键应返回首次响应且不重复执行")
	}
	_, _, err = repository.Transact(context.Background(), created.ID, 0, "stale-write", "UPDATE", func(*domain.RestorationCase, int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
		return nil, nil, nil
	})
	if domain.ErrorCodeOf(err) != domain.CodeConflict {
		t.Fatalf("陈旧 expectedVersion 应被拒绝，得到 %v", err)
	}
	events, err := repository.AuditTrail(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("幂等重放不得追加审计，得到 %d 条", len(events))
	}
}

func archiveAndRelease(t *testing.T, repository *Store, restoration *domain.RestorationCase, key string) *domain.RestorationCase {
	t.Helper()
	now := restoration.UpdatedAt.Add(time.Minute)
	frozen, _, err := repository.Transact(context.Background(), restoration.ID, restoration.Version, key+"-freeze", "VERSION_FROZEN", func(current *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
		next := current.Clone()
		next.Version++
		next.Status = domain.StatusFrozen
		next.UpdatedAt = now
		next.Frozen = &domain.FrozenManifest{CaseID: next.ID, FrozenVersion: next.Version, ContentDigest: "digest-" + key, CanonicalJSON: "{}", SchemaVersion: audit.SchemaVersion, FrozenBy: "复核员", FrozenAt: now}
		event := audit.Event("VERSION_FROZEN", "复核员", domain.RoleReviewer, current.Status, next.Status, next.Version, now, nil)
		return next, &event, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	released, _, err := repository.Transact(context.Background(), frozen.ID, frozen.Version, key+"-release", "PERMIT_ISSUED", func(current *domain.RestorationCase, serial int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
		next := current.Clone()
		next.Version++
		next.Status = domain.StatusReleased
		next.UpdatedAt = now.Add(time.Minute)
		next.Permit = &domain.ReleasePermit{ID: "permit-" + key, CaseID: next.ID, SerialNumber: serial, FrozenVersion: next.Frozen.FrozenVersion, ContentDigest: next.Frozen.ContentDigest, ApprovedBy: "负责人", IssuedAt: next.UpdatedAt, SchemaVersion: audit.SchemaVersion}
		event := audit.Event("PERMIT_ISSUED", "负责人", domain.RoleReviewer, current.Status, next.Status, next.Version, next.UpdatedAt, nil)
		return next, &event, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return released
}

func TestPermitSerialMonotonicAndArchiveIntegrity(t *testing.T) {
	repository := openTestStore(t)
	first := archiveAndRelease(t, repository, createStoredCase(t, repository, "case-a"), "a")
	second := archiveAndRelease(t, repository, createStoredCase(t, repository, "case-b"), "b")
	if first.Permit.SerialNumber != 1 || second.Permit.SerialNumber != 2 {
		t.Fatalf("凭据序号应单调递增，得到 %d、%d", first.Permit.SerialNumber, second.Permit.SerialNumber)
	}
	if _, err := repository.Get(context.Background(), first.ID); err != nil {
		t.Fatalf("未篡改归档应可恢复：%v", err)
	}
	if _, err := repository.db.Exec(`UPDATE frozen_manifests SET content_digest = 'tampered' WHERE case_id = ?`, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(context.Background(), first.ID); err == nil {
		t.Fatal("归档摘要被改动后恢复必须失败")
	}
}
