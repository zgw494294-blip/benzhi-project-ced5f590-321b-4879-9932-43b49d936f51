package progress_cache_rollback_pollution_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func TestFailedMutationDoesNotPublishProgress(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(filepath.Join(t.TempDir(), "restoration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	service := application.New(repository).WithClock(func() time.Time { return now })
	created, err := service.CreateCase(ctx, application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{Actor: "巡护员", Role: domain.RolePatrol, IdempotencyKey: "create-case"},
		ID:          "case-rollback", TreeCode: "GS-ROLLBACK", Location: "保护区东侧", ProtectionGrade: "一级", Owner: "养护组",
		WorkWindowStart: now.Add(time.Hour), WorkWindowEnd: now.Add(3 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = repository.DB().ExecContext(ctx, `
		CREATE TRIGGER reject_case_update
		BEFORE UPDATE ON restoration_cases
		BEGIN
			SELECT RAISE(ABORT, 'forced update failure');
		END`)
	if err != nil {
		t.Fatal(err)
	}

	_, mutationErr := service.AddSurvey(ctx, created.ID, application.AddSurveyCommand{
		CommandMeta: application.CommandMeta{
			Actor: "巡护员", Role: domain.RolePatrol, ExpectedVersion: created.Version, IdempotencyKey: "survey-that-rolls-back",
		},
		ID: "survey-uncommitted", Area: domain.AreaCanopy, ConditionCode: "DEAD_BRANCH",
		Severity: domain.SeverityMedium, Extent: "树冠东侧", Notes: "发现枯枝",
		EvidenceRefs: []string{"photo://rollback"}, ObservedAt: now,
	})
	if mutationErr == nil {
		t.Fatal("预期 SQLite 触发器拒绝事务更新")
	}

	persisted, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Version != created.Version || len(persisted.Surveys) != 0 {
		t.Fatalf("失败事务不应改变持久化聚合：%+v", persisted)
	}

	details, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if details.Case.Version != created.Version || details.Progress.CompletedSurveyAreas != 0 {
		t.Fatalf("详情响应混入了回滚事务的进度：caseVersion=%d completedSurveyAreas=%d",
			details.Case.Version, details.Progress.CompletedSurveyAreas)
	}
}
