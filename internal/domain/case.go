package domain

import (
	"strings"
	"time"
)

type NewCaseInput struct {
	ID              string
	TreeCode        string
	Location        string
	ProtectionGrade string
	Owner           string
	WorkWindowStart time.Time
	WorkWindowEnd   time.Time
	Now             time.Time
}

func NewCase(input NewCaseInput) (*RestorationCase, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, NewError(CodeValidation, "作业 ID 不能为空")
	}
	if strings.TrimSpace(input.TreeCode) == "" || strings.TrimSpace(input.Location) == "" {
		return nil, NewError(CodeValidation, "古树编号和位置不能为空")
	}
	if strings.TrimSpace(input.ProtectionGrade) == "" || strings.TrimSpace(input.Owner) == "" {
		return nil, NewError(CodeValidation, "保护等级和责任人不能为空")
	}
	if input.WorkWindowStart.IsZero() || input.WorkWindowEnd.IsZero() || !input.WorkWindowEnd.After(input.WorkWindowStart) {
		return nil, NewError(CodeValidation, "计划作业结束时间必须晚于开始时间")
	}
	now := input.Now.UTC()
	return &RestorationCase{
		ID: input.ID, TreeCode: strings.TrimSpace(input.TreeCode), Location: strings.TrimSpace(input.Location),
		ProtectionGrade: strings.TrimSpace(input.ProtectionGrade), Owner: strings.TrimSpace(input.Owner),
		WorkWindowStart: input.WorkWindowStart.UTC(), WorkWindowEnd: input.WorkWindowEnd.UTC(),
		Status: StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (c *RestorationCase) ensureMutable(allowed ...CaseStatus) error {
	if c.Frozen != nil || c.Status == StatusFrozen || c.Status == StatusReleased {
		return NewError(CodeImmutable, "冻结版本及放行内容不可修改")
	}
	for _, status := range allowed {
		if c.Status == status {
			return nil
		}
	}
	return NewError(CodeState, "当前状态 %s 不允许此操作", c.Status)
}

func (c *RestorationCase) bump(now time.Time) {
	c.Version++
	c.UpdatedAt = now.UTC()
}

func (c *RestorationCase) ValidateLinks() error {
	riskIDs := make(map[string]bool, len(c.Risks))
	measureIDs := make(map[string]bool, len(c.Measures))
	for _, risk := range c.Risks {
		if risk.CaseID != c.ID {
			return NewError(CodeValidation, "风险 %s 不属于当前作业", risk.ID)
		}
		riskIDs[risk.ID] = true
	}
	surveyIDs := make(map[string]bool, len(c.Surveys))
	for _, survey := range c.Surveys {
		if survey.CaseID != c.ID {
			return NewError(CodeValidation, "调查记录 %s 不属于当前作业", survey.ID)
		}
		surveyIDs[survey.ID] = true
	}
	for _, survey := range c.Surveys {
		if survey.SupersedesID != "" && !surveyIDs[survey.SupersedesID] {
			return NewError(CodeValidation, "调查记录 %s 取代了不存在的记录", survey.ID)
		}
		if survey.SupersededByID != "" && !surveyIDs[survey.SupersededByID] {
			return NewError(CodeValidation, "调查记录 %s 的取代记录不存在", survey.ID)
		}
	}
	for _, measure := range c.Measures {
		if measure.CaseID != c.ID || !riskIDs[measure.RiskID] {
			return NewError(CodeValidation, "措施 %s 关联了无效风险", measure.ID)
		}
		measureIDs[measure.ID] = true
	}
	for _, finding := range c.Findings {
		if finding.CaseID != c.ID || !measureIDs[finding.MeasureID] {
			return NewError(CodeValidation, "复核记录 %s 关联了无效措施", finding.ID)
		}
	}
	return nil
}

func (c *RestorationCase) WorkWindowStatus(now time.Time) WorkWindowStatus {
	now = now.UTC()
	if now.Before(c.WorkWindowStart) {
		return WindowNotStarted
	}
	if now.After(c.WorkWindowEnd) {
		return WindowExpired
	}
	return WindowActive
}

func (c *RestorationCase) Clone() *RestorationCase {
	copyCase := *c
	copyCase.Surveys = append([]SurveyObservation(nil), c.Surveys...)
	for i := range copyCase.Surveys {
		copyCase.Surveys[i].EvidenceRefs = append([]string(nil), c.Surveys[i].EvidenceRefs...)
	}
	copyCase.Risks = append([]RiskItem(nil), c.Risks...)
	copyCase.Measures = append([]TreatmentMeasure(nil), c.Measures...)
	copyCase.Findings = append([]ReviewFinding(nil), c.Findings...)
	for i := range copyCase.Findings {
		if c.Findings[i].ClosedAt != nil {
			value := *c.Findings[i].ClosedAt
			copyCase.Findings[i].ClosedAt = &value
		}
	}
	if c.Frozen != nil {
		value := *c.Frozen
		copyCase.Frozen = &value
	}
	if c.Permit != nil {
		value := *c.Permit
		copyCase.Permit = &value
	}
	return &copyCase
}
