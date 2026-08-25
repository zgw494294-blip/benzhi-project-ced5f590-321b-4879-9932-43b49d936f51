package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func urgencyForSeverity(severity Severity) Urgency {
	switch severity {
	case SeverityCritical:
		return UrgencyImmediate
	case SeverityHigh:
		return UrgencySoon
	default:
		return UrgencyRoutine
	}
}

func (c *RestorationCase) GenerateRisks(now time.Time, idFor func(SurveyObservation) string) (int, error) {
	if err := c.ensureMutable(StatusSurveyComplete); err != nil {
		return 0, err
	}
	existing := make(map[string]bool)
	for _, risk := range c.Risks {
		existing[risk.SourceObservationID] = true
	}
	created := 0
	for _, observation := range c.Surveys {
		if observation.SupersededByID != "" {
			continue
		}
		if existing[observation.ID] {
			continue
		}
		category := string(observation.Area) + ":" + observation.ConditionCode
		c.Risks = append(c.Risks, RiskItem{
			ID: idFor(observation), CaseID: c.ID, SourceObservationID: observation.ID,
			Category: category, Severity: observation.Severity, Urgency: urgencyForSeverity(observation.Severity),
			Rationale: fmt.Sprintf("依据%s区域调查：%s；影响范围：%s", observation.Area, observation.Notes, observation.Extent),
			Status:    RiskOpen, CreatedAt: now.UTC(),
		})
		created++
	}
	if created == 0 {
		return 0, NewError(CodeDuplicate, "没有可生成的新风险")
	}
	c.bump(now)
	return created, nil
}

func RiskPriorityFor(severity Severity, urgency Urgency) RiskPriority {
	if severity == SeverityCritical && urgency != UrgencyRoutine || severity == SeverityHigh && urgency == UrgencyImmediate {
		return PriorityHigh
	}
	if severity == SeverityCritical || severity == SeverityHigh || urgency == UrgencyImmediate || severity == SeverityMedium && urgency == UrgencySoon {
		return PriorityMedium
	}
	return PriorityRoutine
}

type RiskUrgencyAdjustment struct {
	RiskID  string  `json:"riskId"`
	Urgency Urgency `json:"urgency"`
	Reason  string  `json:"reason"`
}

type RiskUrgencyChange struct {
	RiskID string  `json:"riskId"`
	Before Urgency `json:"before"`
	After  Urgency `json:"after"`
	Reason string  `json:"reason"`
}

func (c *RestorationCase) AdjustRiskUrgencies(adjustments []RiskUrgencyAdjustment, now time.Time) ([]RiskUrgencyChange, error) {
	if err := c.ensureMutable(StatusSurveyComplete); err != nil {
		return nil, err
	}
	if len(adjustments) == 0 {
		return nil, NewError(CodeValidation, "风险紧迫性调整不能为空")
	}
	indexes := make(map[string]int, len(c.Risks))
	for i := range c.Risks {
		indexes[c.Risks[i].ID] = i
	}
	seen := make(map[string]bool, len(adjustments))
	changes := make([]RiskUrgencyChange, 0, len(adjustments))
	for itemIndex, adjustment := range adjustments {
		adjustment.RiskID = strings.TrimSpace(adjustment.RiskID)
		adjustment.Reason = strings.TrimSpace(adjustment.Reason)
		if adjustment.RiskID == "" || adjustment.Reason == "" {
			return nil, NewError(CodeValidation, "第 %d 项风险编号和调整理由不能为空", itemIndex+1)
		}
		if seen[adjustment.RiskID] {
			return nil, NewError(CodeDuplicate, "批量请求包含重复风险 %s", adjustment.RiskID)
		}
		seen[adjustment.RiskID] = true
		index, found := indexes[adjustment.RiskID]
		if !found {
			return nil, NewError(CodeNotFound, "第 %d 项风险 %s 不存在", itemIndex+1, adjustment.RiskID)
		}
		if !adjustment.Urgency.Valid() {
			return nil, NewError(CodeValidation, "第 %d 项风险 %s 的紧迫性无效", itemIndex+1, adjustment.RiskID)
		}
		changes = append(changes, RiskUrgencyChange{RiskID: adjustment.RiskID, Before: c.Risks[index].Urgency, After: adjustment.Urgency, Reason: adjustment.Reason})
	}
	for _, change := range changes {
		c.Risks[indexes[change.RiskID]].Urgency = change.After
	}
	c.bump(now)
	return changes, nil
}

func (c *RestorationCase) AddManualRisk(risk RiskItem, now time.Time) error {
	if err := c.ensureMutable(StatusSurveyComplete); err != nil {
		return err
	}
	if strings.TrimSpace(risk.Category) == "" || strings.TrimSpace(risk.Rationale) == "" || !risk.Severity.Valid() {
		return NewError(CodeValidation, "人工风险的类别、严重度和理由不能为空")
	}
	if !risk.Urgency.Valid() {
		return NewError(CodeValidation, "风险紧迫性无效")
	}
	for _, existing := range c.Risks {
		if existing.ID == risk.ID {
			return NewError(CodeDuplicate, "风险 ID 已存在")
		}
	}
	risk.CaseID, risk.Status, risk.CreatedAt = c.ID, RiskOpen, now.UTC()
	c.Risks = append(c.Risks, risk)
	c.bump(now)
	return nil
}

func (c *RestorationCase) DeleteRisk(id string, now time.Time) error {
	if err := c.ensureMutable(StatusSurveyComplete); err != nil {
		return err
	}
	for _, measure := range c.Measures {
		if measure.RiskID == id {
			return NewError(CodeState, "已被方案引用的风险不能删除")
		}
	}
	for i := range c.Risks {
		if c.Risks[i].ID == id {
			c.Risks = append(c.Risks[:i], c.Risks[i+1:]...)
			c.bump(now)
			return nil
		}
	}
	return NewError(CodeNotFound, "风险不存在")
}

func (c *RestorationCase) UpsertMeasure(measure TreatmentMeasure, now time.Time) error {
	if err := c.ensureMutable(StatusSurveyComplete, StatusRemediation); err != nil {
		return err
	}
	if measure.Sequence < 1 || strings.TrimSpace(measure.Action) == "" || strings.TrimSpace(measure.Prohibitions) == "" || strings.TrimSpace(measure.AcceptanceCriteria) == "" || strings.TrimSpace(measure.PreparedBy) == "" {
		return NewError(CodeValidation, "措施、禁止事项、验收标准、顺序和编制人必须完整")
	}
	riskIndex := -1
	for i := range c.Risks {
		if c.Risks[i].ID == measure.RiskID {
			riskIndex = i
			break
		}
	}
	if riskIndex < 0 {
		return NewError(CodeNotFound, "关联风险不存在")
	}
	if c.SubmittedRevision > 0 && measure.Revision <= c.SubmittedRevision {
		return NewError(CodeImmutable, "已送审修订不可修改，请使用新修订号")
	}
	measure.CaseID, measure.UpdatedAt = c.ID, now.UTC()
	for i := range c.Measures {
		if c.Measures[i].ID == measure.ID {
			c.Measures[i] = measure
			c.Risks[riskIndex].Status = RiskCovered
			c.bump(now)
			return nil
		}
	}
	c.Measures = append(c.Measures, measure)
	c.Risks[riskIndex].Status = RiskCovered
	c.bump(now)
	return nil
}

func (c *RestorationCase) SubmitReview(revision int, now time.Time) error {
	if err := c.ensureMutable(StatusSurveyComplete, StatusRemediation); err != nil {
		return err
	}
	if revision <= c.SubmittedRevision {
		return NewError(CodeValidation, "送审修订号必须递增")
	}
	if len(c.Risks) == 0 {
		return NewError(CodeValidation, "没有风险项，不能送审")
	}
	covered := make(map[string]bool)
	sequences := make(map[int]bool)
	for _, measure := range c.Measures {
		if measure.Revision == revision {
			covered[measure.RiskID] = true
			if sequences[measure.Sequence] {
				return NewError(CodeValidation, "实施顺序不能重复")
			}
			sequences[measure.Sequence] = true
		}
	}
	for _, risk := range c.Risks {
		if !covered[risk.ID] {
			return NewError(CodeValidation, "风险 %s 未被当前修订措施覆盖", risk.ID)
		}
	}
	c.SubmittedRevision = revision
	c.Status = StatusInReview
	sort.SliceStable(c.Measures, func(i, j int) bool { return c.Measures[i].Sequence < c.Measures[j].Sequence })
	c.bump(now)
	return nil
}
