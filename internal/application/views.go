package application

import (
	"sort"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

type RiskWorkItem struct {
	Risk     domain.RiskItem     `json:"risk"`
	Priority domain.RiskPriority `json:"priority"`
	Covered  bool                `json:"covered"`
}

type RiskPriorityStats struct {
	Priority     domain.RiskPriority `json:"priority"`
	RiskCount    int                 `json:"riskCount"`
	CoveredCount int                 `json:"coveredCount"`
}

type RiskWorklist struct {
	Items []RiskWorkItem      `json:"items"`
	Stats []RiskPriorityStats `json:"stats"`
}

type ReviewProgress struct {
	Total      int `json:"total"`
	Reviewed   int `json:"reviewed"`
	Passed     int `json:"passed"`
	Returned   int `json:"returned"`
	Unreviewed int `json:"unreviewed"`
}

type RemediationTodoItem struct {
	Finding domain.ReviewFinding    `json:"finding"`
	Measure domain.TreatmentMeasure `json:"measure"`
}

type RemediationTodos struct {
	AwaitingEvidence []RemediationTodoItem `json:"awaitingEvidence"`
	AwaitingVerify   []RemediationTodoItem `json:"awaitingVerify"`
	VerificationFail []RemediationTodoItem `json:"verificationFail"`
	Closed           []RemediationTodoItem `json:"closed"`
}

func priorityRank(priority domain.RiskPriority) int {
	switch priority {
	case domain.PriorityHigh:
		return 0
	case domain.PriorityMedium:
		return 1
	default:
		return 2
	}
}

func severityRank(severity domain.Severity) int {
	switch severity {
	case domain.SeverityCritical:
		return 0
	case domain.SeverityHigh:
		return 1
	case domain.SeverityMedium:
		return 2
	default:
		return 3
	}
}

func BuildRiskWorklist(restoration *domain.RestorationCase) RiskWorklist {
	result := RiskWorklist{Stats: []RiskPriorityStats{{Priority: domain.PriorityHigh}, {Priority: domain.PriorityMedium}, {Priority: domain.PriorityRoutine}}}
	covered := make(map[string]bool)
	for _, measure := range restoration.Measures {
		covered[measure.RiskID] = true
	}
	for _, risk := range restoration.Risks {
		priority := domain.RiskPriorityFor(risk.Severity, risk.Urgency)
		result.Items = append(result.Items, RiskWorkItem{Risk: risk, Priority: priority, Covered: covered[risk.ID]})
		for i := range result.Stats {
			if result.Stats[i].Priority == priority {
				result.Stats[i].RiskCount++
				if covered[risk.ID] {
					result.Stats[i].CoveredCount++
				}
			}
		}
	}
	sort.SliceStable(result.Items, func(i, j int) bool {
		left, right := result.Items[i], result.Items[j]
		if priorityRank(left.Priority) != priorityRank(right.Priority) {
			return priorityRank(left.Priority) < priorityRank(right.Priority)
		}
		if severityRank(left.Risk.Severity) != severityRank(right.Risk.Severity) {
			return severityRank(left.Risk.Severity) < severityRank(right.Risk.Severity)
		}
		if !left.Risk.CreatedAt.Equal(right.Risk.CreatedAt) {
			return left.Risk.CreatedAt.Before(right.Risk.CreatedAt)
		}
		return left.Risk.ID < right.Risk.ID
	})
	return result
}

func currentRevisionMeasures(restoration *domain.RestorationCase) map[string]domain.TreatmentMeasure {
	result := make(map[string]domain.TreatmentMeasure)
	for _, measure := range restoration.Measures {
		if measure.Revision == restoration.SubmittedRevision {
			result[measure.ID] = measure
		}
	}
	return result
}

func BuildReviewProgress(restoration *domain.RestorationCase) ReviewProgress {
	measures := currentRevisionMeasures(restoration)
	result := ReviewProgress{Total: len(measures)}
	for _, finding := range restoration.Findings {
		if _, found := measures[finding.MeasureID]; !found {
			continue
		}
		result.Reviewed++
		if finding.Decision == domain.DecisionPass {
			result.Passed++
		} else if finding.Decision == domain.DecisionReturn {
			result.Returned++
		}
	}
	result.Unreviewed = result.Total - result.Reviewed
	return result
}

func BuildRemediationTodos(restoration *domain.RestorationCase) RemediationTodos {
	result := RemediationTodos{}
	measures := currentRevisionMeasures(restoration)
	for _, finding := range restoration.Findings {
		measure, found := measures[finding.MeasureID]
		if !found || finding.Decision != domain.DecisionReturn {
			continue
		}
		item := RemediationTodoItem{Finding: finding, Measure: measure}
		switch {
		case finding.VerificationDecision == domain.VerificationPass && finding.ClosedAt != nil:
			result.Closed = append(result.Closed, item)
		case finding.VerificationDecision == domain.VerificationFail:
			result.VerificationFail = append(result.VerificationFail, item)
		case finding.RemediationEvidence == "":
			result.AwaitingEvidence = append(result.AwaitingEvidence, item)
		default:
			result.AwaitingVerify = append(result.AwaitingVerify, item)
		}
	}
	return result
}

type CaseProgress struct {
	StatusLabel          string              `json:"statusLabel"`
	CompletedSurveyAreas int                 `json:"completedSurveyAreas"`
	RequiredSurveyAreas  int                 `json:"requiredSurveyAreas"`
	MissingSurveyAreas   []domain.SurveyArea `json:"missingSurveyAreas"`
	RiskCount            int                 `json:"riskCount"`
	CoveredRiskCount     int                 `json:"coveredRiskCount"`
	UncoveredRiskIDs     []string            `json:"uncoveredRiskIds"`
	CurrentMeasureCount  int                 `json:"currentMeasureCount"`
	ReviewedMeasureCount int                 `json:"reviewedMeasureCount"`
	OpenFindingIDs       []string            `json:"openFindingIds"`
	CanFreeze            bool                `json:"canFreeze"`
	CanApprove           bool                `json:"canApprove"`
	ContentLocked        bool                `json:"contentLocked"`
	NextAction           string              `json:"nextAction"`
}

var statusLabels = map[domain.CaseStatus]string{
	domain.StatusDraft:          "草拟",
	domain.StatusSurveyComplete: "调查完成",
	domain.StatusInReview:       "送审中",
	domain.StatusRemediation:    "整改中",
	domain.StatusReviewPassed:   "复核通过",
	domain.StatusFrozen:         "已冻结",
	domain.StatusReleased:       "已放行",
}

func BuildProgress(restoration *domain.RestorationCase) CaseProgress {
	progress := CaseProgress{
		StatusLabel: statusLabels[restoration.Status], RequiredSurveyAreas: len(domain.RequiredSurveyAreas),
		RiskCount: len(restoration.Risks), CanApprove: restoration.Status == domain.StatusFrozen,
		ContentLocked: restoration.Frozen != nil || restoration.Status == domain.StatusFrozen || restoration.Status == domain.StatusReleased,
	}
	coveredAreas := make(map[domain.SurveyArea]bool)
	for _, observation := range restoration.Surveys {
		if observation.SupersededByID == "" {
			coveredAreas[observation.Area] = true
		}
	}
	progress.CompletedSurveyAreas = len(coveredAreas)
	for _, area := range domain.RequiredSurveyAreas {
		if !coveredAreas[area] {
			progress.MissingSurveyAreas = append(progress.MissingSurveyAreas, area)
		}
	}
	currentMeasureByRisk := make(map[string]bool)
	currentMeasureIDs := make(map[string]bool)
	for _, measure := range restoration.Measures {
		if restoration.SubmittedRevision == 0 || measure.Revision == restoration.SubmittedRevision {
			currentMeasureByRisk[measure.RiskID] = true
			currentMeasureIDs[measure.ID] = true
			progress.CurrentMeasureCount++
		}
	}
	for _, risk := range restoration.Risks {
		if currentMeasureByRisk[risk.ID] {
			progress.CoveredRiskCount++
		} else {
			progress.UncoveredRiskIDs = append(progress.UncoveredRiskIDs, risk.ID)
		}
	}
	for _, finding := range restoration.Findings {
		if !currentMeasureIDs[finding.MeasureID] {
			continue
		}
		progress.ReviewedMeasureCount++
		if finding.Decision == domain.DecisionReturn && finding.VerificationDecision != domain.VerificationPass {
			progress.OpenFindingIDs = append(progress.OpenFindingIDs, finding.ID)
		}
	}
	progress.CanFreeze = restoration.CanFreeze() == nil
	sort.Strings(progress.UncoveredRiskIDs)
	sort.Strings(progress.OpenFindingIDs)
	progress.NextAction = nextAction(restoration, progress)
	return progress
}

func nextAction(restoration *domain.RestorationCase, progress CaseProgress) string {
	switch restoration.Status {
	case domain.StatusDraft:
		if len(progress.MissingSurveyAreas) > 0 {
			return "补齐树冠、主干、根区和周边环境调查后确认现场快照"
		}
		return "确认调查范围完整，形成可追溯现场快照"
	case domain.StatusSurveyComplete:
		if progress.RiskCount == 0 {
			return "依据调查生成风险项"
		}
		if len(progress.UncoveredRiskIDs) > 0 {
			return "为每个开放风险编制当前修订的处置措施"
		}
		return "确认措施顺序、禁止事项和验收标准后提交复核"
	case domain.StatusInReview:
		return "复核人员逐项登记通过或退回结论"
	case domain.StatusRemediation:
		if len(progress.OpenFindingIDs) > 0 {
			return "提交退回项整改证据并完成复验闭环"
		}
		return "补齐当前送审修订的逐项复核结论"
	case domain.StatusReviewPassed:
		return "冻结已通过方案并生成规范化摘要"
	case domain.StatusFrozen:
		return "批准冻结版本并签发不可变开工凭据"
	case domain.StatusReleased:
		return "核验放行凭据摘要并按冻结版本实施"
	default:
		return "检查作业状态"
	}
}
