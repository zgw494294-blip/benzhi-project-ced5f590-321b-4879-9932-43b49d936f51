package domain_test

import (
	"fmt"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

var testNow = time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)

func newTestCase(t *testing.T) *domain.RestorationCase {
	t.Helper()
	restoration, err := domain.NewCase(domain.NewCaseInput{
		ID: "case-1", TreeCode: "GS-001", Location: "古树公园东侧", ProtectionGrade: "一级", Owner: "张三",
		WorkWindowStart: testNow.Add(24 * time.Hour), WorkWindowEnd: testNow.Add(48 * time.Hour), Now: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	return restoration
}

func surveyedCase(t *testing.T) *domain.RestorationCase {
	t.Helper()
	restoration := newTestCase(t)
	for index, area := range domain.RequiredSurveyAreas {
		err := restoration.AddObservation(domain.SurveyObservation{
			ID: fmt.Sprintf("survey-%d", index), Area: area, ConditionCode: "OBS", Severity: domain.SeverityMedium,
			Extent: "局部", Notes: "结构化调查事实", EvidenceRefs: []string{fmt.Sprintf("照片-%d", index+1)}, ObservedBy: "巡护员",
		}, testNow.Add(time.Duration(index+1)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := restoration.CompleteSurvey(testNow.Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := restoration.GenerateRisks(testNow.Add(11*time.Minute), func(observation domain.SurveyObservation) string { return "risk-" + observation.ID }); err != nil {
		t.Fatal(err)
	}
	return restoration
}

func reviewedCase(t *testing.T, returned bool) *domain.RestorationCase {
	t.Helper()
	restoration := surveyedCase(t)
	for index, risk := range restoration.Risks {
		err := restoration.UpsertMeasure(domain.TreatmentMeasure{
			ID: fmt.Sprintf("measure-%d", index), RiskID: risk.ID, Revision: 1, Sequence: index + 1,
			Action: "实施复壮处置", Prohibitions: "禁止损伤健康组织", AcceptanceCriteria: "复测合格", PreparedBy: "编制员",
		}, testNow.Add(time.Duration(20+index)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := restoration.SubmitReview(1, testNow.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for index, measure := range restoration.Measures {
		decision, issue := domain.DecisionPass, ""
		if returned && index == 0 {
			decision, issue = domain.DecisionReturn, "补充根区保护边界"
		}
		err := restoration.RecordFinding(domain.ReviewFinding{
			ID: fmt.Sprintf("finding-%d", index), MeasureID: measure.ID, Decision: decision, Issue: issue, Reviewer: "复核员",
		}, testNow.Add(time.Duration(40+index)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}
	return restoration
}

func TestSurveyCoverageAndRiskReference(t *testing.T) {
	restoration := newTestCase(t)
	if err := restoration.CompleteSurvey(testNow); domain.ErrorCodeOf(err) != domain.CodeValidation {
		t.Fatalf("缺少调查范围应失败，得到 %v", err)
	}
	restoration = surveyedCase(t)
	risk := restoration.Risks[0]
	if err := restoration.UpsertMeasure(domain.TreatmentMeasure{
		ID: "measure-ref", RiskID: risk.ID, Revision: 1, Sequence: 1, Action: "处置", Prohibitions: "禁止扩大范围", AcceptanceCriteria: "验收", PreparedBy: "编制员",
	}, testNow.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := restoration.DeleteRisk(risk.ID, testNow.Add(2*time.Hour)); domain.ErrorCodeOf(err) != domain.CodeState {
		t.Fatalf("已引用风险不应删除，得到 %v", err)
	}
}

func TestReviewRemediationFreezeAndRelease(t *testing.T) {
	restoration := reviewedCase(t, true)
	if restoration.Status != domain.StatusRemediation {
		t.Fatalf("期望整改中，得到 %s", restoration.Status)
	}
	if err := restoration.CanFreeze(); domain.ErrorCodeOf(err) != domain.CodeState {
		t.Fatalf("未闭环时不应冻结，得到 %v", err)
	}
	finding := restoration.Findings[0]
	if err := restoration.AddRemediation(finding.ID, "整改照片 R-01 与复测记录", testNow.Add(50*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := restoration.VerifyRemediation(finding.ID, "复核员", domain.VerificationPass, testNow.Add(51*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if restoration.Status != domain.StatusReviewPassed {
		t.Fatalf("期望复核通过，得到 %s", restoration.Status)
	}
	manifest, err := audit.BuildManifest(restoration, "复核负责人", testNow.Add(52*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := restoration.Freeze(manifest, testNow.Add(52*time.Minute)); err != nil {
		t.Fatal(err)
	}
	permit := domain.ReleasePermit{ID: "permit-1", CaseID: restoration.ID, SerialNumber: 1, FrozenVersion: restoration.Frozen.FrozenVersion, ContentDigest: restoration.Frozen.ContentDigest, ApprovedBy: "负责人", SchemaVersion: audit.SchemaVersion}
	if err := restoration.Release(permit, testNow.Add(53*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !audit.Verify(restoration).Valid {
		t.Fatal("放行凭据应通过完整性核验")
	}
	if err := restoration.AddManualRisk(domain.RiskItem{ID: "late"}, testNow.Add(54*time.Minute)); domain.ErrorCodeOf(err) != domain.CodeImmutable {
		t.Fatalf("冻结内容应不可修改，得到 %v", err)
	}
}

func TestSubmittedRevisionIsLocked(t *testing.T) {
	restoration := reviewedCase(t, false)
	measure := restoration.Measures[0]
	measure.Action = "试图修改"
	if err := restoration.UpsertMeasure(measure, testNow.Add(3*time.Hour)); domain.ErrorCodeOf(err) != domain.CodeState && domain.ErrorCodeOf(err) != domain.CodeImmutable {
		t.Fatalf("已通过送审修订不应修改，得到 %v", err)
	}
}

func TestSurveyCorrectionPreflightKeepsHistory(t *testing.T) {
	restoration := newTestCase(t)
	for index, area := range domain.RequiredSurveyAreas {
		err := restoration.AddObservation(domain.SurveyObservation{
			ID: fmt.Sprintf("survey-original-%d", index), Area: area, ConditionCode: "OBS", Severity: domain.SeverityMedium,
			Extent: "局部", Notes: "原始事实", EvidenceRefs: []string{fmt.Sprintf("证据-%d", index)}, ObservedBy: "巡护员",
		}, testNow.Add(time.Duration(index)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
	}
	beforeVersion := restoration.Version
	err := restoration.CorrectObservation(domain.SurveyObservation{
		ID: "survey-root-corrected", Area: domain.AreaRootZone, ConditionCode: "SOIL", Severity: domain.SeverityHigh,
		Extent: "根盘东侧", Notes: "复测后的根区事实", EvidenceRefs: []string{"证据-根区-复测"}, ObservedBy: "巡护员",
	}, "survey-original-2", "原记录方位填写错误", testNow.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if restoration.Version != beforeVersion+1 || restoration.Surveys[2].SupersededByID != "survey-root-corrected" {
		t.Fatalf("更正应只递增一个版本并标记原记录：%+v", restoration.Surveys[2])
	}
	preflight := restoration.SurveyPreflight()
	if !preflight.Ready || len(preflight.Areas[2].Effective) != 1 || preflight.Areas[2].Effective[0].ID != "survey-root-corrected" || len(preflight.Areas[2].History) != 1 {
		t.Fatalf("预检未正确选择更正记录：%+v", preflight)
	}
	unchangedVersion, unchangedCount := restoration.Version, len(restoration.Surveys)
	err = restoration.CorrectObservation(domain.SurveyObservation{
		ID: "wrong-area", Area: domain.AreaCanopy, ConditionCode: "OBS", Severity: domain.SeverityLow,
		Extent: "局部", Notes: "错误区域", EvidenceRefs: []string{"新证据"}, ObservedBy: "巡护员",
	}, "survey-original-1", "尝试跨区", testNow.Add(11*time.Minute))
	if domain.ErrorCodeOf(err) != domain.CodeValidation || restoration.Version != unchangedVersion || len(restoration.Surveys) != unchangedCount {
		t.Fatalf("跨区域更正应原样拒绝，得到 %v", err)
	}
}

func TestBatchRiskUrgencyAndReviewAreAtomic(t *testing.T) {
	restoration := surveyedCase(t)
	beforeVersion := restoration.Version
	beforeUrgency := restoration.Risks[0].Urgency
	_, err := restoration.AdjustRiskUrgencies([]domain.RiskUrgencyAdjustment{
		{RiskID: restoration.Risks[0].ID, Urgency: domain.UrgencyImmediate, Reason: "现场变化"},
		{RiskID: "unknown", Urgency: domain.UrgencySoon, Reason: "无效项"},
	}, testNow.Add(time.Hour))
	if domain.ErrorCodeOf(err) != domain.CodeNotFound || restoration.Version != beforeVersion || restoration.Risks[0].Urgency != beforeUrgency {
		t.Fatalf("风险批量校验失败时不得部分更新：%v", err)
	}
	changes, err := restoration.AdjustRiskUrgencies([]domain.RiskUrgencyAdjustment{{RiskID: restoration.Risks[0].ID, Urgency: domain.UrgencyImmediate, Reason: "现场变化"}}, testNow.Add(2*time.Hour))
	if err != nil || len(changes) != 1 || restoration.Version != beforeVersion+1 {
		t.Fatalf("合法风险批量调整应只递增一个版本：%v %+v", err, changes)
	}
	for index, risk := range restoration.Risks {
		if err := restoration.UpsertMeasure(domain.TreatmentMeasure{
			ID: fmt.Sprintf("batch-measure-%d", index), RiskID: risk.ID, Revision: 1, Sequence: index + 1,
			Action: "处置", Prohibitions: "禁止损伤", AcceptanceCriteria: "验收通过", PreparedBy: "编制员",
		}, testNow.Add(time.Duration(3+index)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if err := restoration.SubmitReview(1, testNow.Add(8*time.Hour)); err != nil {
		t.Fatal(err)
	}
	beforeVersion = restoration.Version
	err = restoration.RecordFindings([]domain.ReviewFinding{
		{ID: "batch-finding-1", MeasureID: restoration.Measures[0].ID, Decision: domain.DecisionPass, Reviewer: "复核员"},
		{ID: "batch-finding-2", MeasureID: restoration.Measures[1].ID, Decision: domain.DecisionReturn, Reviewer: "复核员"},
	}, testNow.Add(9*time.Hour))
	if domain.ErrorCodeOf(err) != domain.CodeValidation || restoration.Version != beforeVersion || len(restoration.Findings) != 0 {
		t.Fatalf("退回问题缺失时整批不得写入：%v", err)
	}
	err = restoration.RecordFindings([]domain.ReviewFinding{
		{ID: "batch-finding-1", MeasureID: restoration.Measures[0].ID, Decision: domain.DecisionPass, Reviewer: "复核员"},
		{ID: "batch-finding-2", MeasureID: restoration.Measures[1].ID, Decision: domain.DecisionReturn, Issue: "补充边界", Reviewer: "复核员"},
	}, testNow.Add(10*time.Hour))
	if err != nil || restoration.Version != beforeVersion+1 || len(restoration.Findings) != 2 || restoration.Status != domain.StatusRemediation {
		t.Fatalf("合法复核批次应原子保存并进入整改：%v", err)
	}
}

func TestWorkWindowStatusIncludesBoundaries(t *testing.T) {
	restoration := newTestCase(t)
	if restoration.WorkWindowStatus(restoration.WorkWindowStart.Add(-time.Nanosecond)) != domain.WindowNotStarted ||
		restoration.WorkWindowStatus(restoration.WorkWindowStart) != domain.WindowActive ||
		restoration.WorkWindowStatus(restoration.WorkWindowEnd) != domain.WindowActive ||
		restoration.WorkWindowStatus(restoration.WorkWindowEnd.Add(time.Nanosecond)) != domain.WindowExpired {
		t.Fatal("作业窗口边界判定错误")
	}
}
