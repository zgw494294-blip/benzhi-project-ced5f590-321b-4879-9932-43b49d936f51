package idempotencyoperationcollision

import (
	"context"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func TestIdempotencyKeyCannotReplayDifferentOperation(t *testing.T) {
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	newService := func(t *testing.T, name string) *application.Service {
		t.Helper()
		repository, err := store.Open("file:" + name + "?mode=memory&cache=shared")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = repository.Close() })
		return application.New(repository).WithClock(func() time.Time { return now })
	}
	create := func(t *testing.T, service *application.Service, id, key string) *domain.RestorationCase {
		t.Helper()
		created, err := service.CreateCase(context.Background(), application.CreateCaseCommand{
			CommandMeta: application.CommandMeta{Actor: "巡护员", Role: domain.RolePatrol, IdempotencyKey: key},
			ID:          id, TreeCode: "GS-001", Location: "测试点", ProtectionGrade: "一级", Owner: "责任人",
			WorkWindowStart: now.Add(time.Hour), WorkWindowEnd: now.Add(2 * time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
		return created
	}

	t.Run("different operation", func(t *testing.T) {
		service := newService(t, "idempotency-different-operation")
		created := create(t, service, "case-operation", "shared-command-key")
		_, err := service.CompleteSurvey(context.Background(), created.ID, application.CommandMeta{
			Actor: "巡护员", Role: domain.RolePatrol, ExpectedVersion: created.Version, IdempotencyKey: "shared-command-key",
		})
		if err == nil {
			t.Error("同一幂等键用于不同操作时被错误地当作成功重放")
		}
	})

	t.Run("different request", func(t *testing.T) {
		service := newService(t, "idempotency-different-request")
		created := create(t, service, "case-request", "create-key")
		first, err := service.AddSurvey(context.Background(), created.ID, application.AddSurveyCommand{
			CommandMeta: application.CommandMeta{Actor: "巡护员", Role: domain.RolePatrol, ExpectedVersion: created.Version, IdempotencyKey: "survey-key"},
			ID:          "survey-first", Area: domain.AreaCanopy, ConditionCode: "OBS", Severity: domain.SeverityMedium,
			Extent: "局部", Notes: "第一次请求", EvidenceRefs: []string{"照片-1"},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.AddSurvey(context.Background(), created.ID, application.AddSurveyCommand{
			CommandMeta: application.CommandMeta{Actor: "巡护员", Role: domain.RolePatrol, ExpectedVersion: first.Version, IdempotencyKey: "survey-key"},
			ID:          "survey-second", Area: domain.AreaTrunk, ConditionCode: "OBS", Severity: domain.SeverityHigh,
			Extent: "大部", Notes: "不同的第二次请求", EvidenceRefs: []string{"照片-2"},
		})
		if err == nil {
			t.Error("同一操作的不同请求内容被错误地当作成功重放")
		}
	})
}
