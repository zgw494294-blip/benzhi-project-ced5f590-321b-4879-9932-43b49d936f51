package application

import (
	"context"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func TestAuthorizationAndReadableProgress(t *testing.T) {
	repository, err := store.Open("file:apptest?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	service := New(repository).WithClock(func() time.Time { return now })
	wrongRole := CreateCaseCommand{CommandMeta: CommandMeta{Actor: "编制员", Role: domain.RolePlanner, IdempotencyKey: "wrong"}, ID: "case", TreeCode: "GS", Location: "位置", ProtectionGrade: "一级", Owner: "责任人", WorkWindowStart: now.Add(time.Hour), WorkWindowEnd: now.Add(2 * time.Hour)}
	if _, err := service.CreateCase(context.Background(), wrongRole); domain.ErrorCodeOf(err) != domain.CodeForbidden {
		t.Fatalf("无权角色应被拒绝，得到 %v", err)
	}
	wrongRole.Role, wrongRole.IdempotencyKey = domain.RolePatrol, "create"
	restoration, err := service.CreateCase(context.Background(), wrongRole)
	if err != nil {
		t.Fatal(err)
	}
	if restoration.Status != domain.StatusDraft {
		t.Fatalf("新建状态错误：%s", restoration.Status)
	}
	details, err := service.Get(context.Background(), restoration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Progress.MissingSurveyAreas) != 4 || details.Progress.NextAction == "" {
		t.Fatalf("可读进度视图不完整：%+v", details.Progress)
	}
	stale := CommandMeta{Actor: "巡护员", Role: domain.RolePatrol, ExpectedVersion: 0, IdempotencyKey: "stale"}
	if _, err := service.CompleteSurvey(context.Background(), restoration.ID, stale); domain.ErrorCodeOf(err) != domain.CodeConflict {
		t.Fatalf("应用层应透传并发冲突，得到 %v", err)
	}
}

func TestRiskWorklistSortingAndRemediationGroups(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	restoration := &domain.RestorationCase{
		SubmittedRevision: 2,
		Risks: []domain.RiskItem{
			{ID: "risk-routine", Severity: domain.SeverityLow, Urgency: domain.UrgencyRoutine, CreatedAt: now},
			{ID: "risk-high", Severity: domain.SeverityCritical, Urgency: domain.UrgencyImmediate, CreatedAt: now.Add(time.Minute)},
			{ID: "risk-medium", Severity: domain.SeverityHigh, Urgency: domain.UrgencySoon, CreatedAt: now.Add(2 * time.Minute)},
		},
		Measures: []domain.TreatmentMeasure{
			{ID: "measure-high", RiskID: "risk-high", Revision: 2, Sequence: 1, Action: "高风险措施"},
			{ID: "measure-medium", RiskID: "risk-medium", Revision: 2, Sequence: 2, Action: "中风险措施"},
		},
		Findings: []domain.ReviewFinding{
			{ID: "finding-evidence", MeasureID: "measure-high", Decision: domain.DecisionReturn, Issue: "补充证据", VerificationDecision: domain.VerificationPending},
			{ID: "finding-failed", MeasureID: "measure-medium", Decision: domain.DecisionReturn, Issue: "重新整改", RemediationEvidence: "证据", VerificationDecision: domain.VerificationFail},
		},
	}
	worklist := BuildRiskWorklist(restoration)
	if len(worklist.Items) != 3 || worklist.Items[0].Risk.ID != "risk-high" || worklist.Items[1].Risk.ID != "risk-medium" || worklist.Items[2].Risk.ID != "risk-routine" {
		t.Fatalf("风险工作清单排序错误：%+v", worklist.Items)
	}
	if worklist.Stats[0].RiskCount != 1 || worklist.Stats[0].CoveredCount != 1 || worklist.Stats[1].RiskCount != 1 || worklist.Stats[2].RiskCount != 1 {
		t.Fatalf("风险分级与覆盖统计错误：%+v", worklist.Stats)
	}
	todos := BuildRemediationTodos(restoration)
	if len(todos.AwaitingEvidence) != 1 || len(todos.VerificationFail) != 1 || len(todos.AwaitingVerify) != 0 || len(todos.Closed) != 0 {
		t.Fatalf("整改待办分组错误：%+v", todos)
	}
}
