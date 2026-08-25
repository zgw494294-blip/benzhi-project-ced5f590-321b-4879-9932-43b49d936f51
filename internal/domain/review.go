package domain

import (
	"strings"
	"time"
)

func (c *RestorationCase) RecordFinding(finding ReviewFinding, now time.Time) error {
	return c.RecordFindings([]ReviewFinding{finding}, now)
}

func (c *RestorationCase) RecordFindings(findings []ReviewFinding, now time.Time) error {
	if err := c.ensureMutable(StatusInReview, StatusRemediation); err != nil {
		return err
	}
	if len(findings) == 0 {
		return NewError(CodeValidation, "复核结论批次不能为空")
	}
	currentMeasures := make(map[string]bool)
	for _, measure := range c.Measures {
		if measure.Revision == c.SubmittedRevision {
			currentMeasures[measure.ID] = true
		}
	}
	reviewed := make(map[string]bool)
	findingIDs := make(map[string]bool)
	for _, existing := range c.Findings {
		findingIDs[existing.ID] = true
		if existing.Decision != "" {
			reviewed[existing.MeasureID] = true
		}
	}
	seen := make(map[string]bool, len(findings))
	for itemIndex := range findings {
		finding := &findings[itemIndex]
		if finding.Decision != DecisionPass && finding.Decision != DecisionReturn {
			return NewError(CodeValidation, "第 %d 项复核结论无效", itemIndex+1)
		}
		if strings.TrimSpace(finding.Reviewer) == "" {
			return NewError(CodeValidation, "第 %d 项复核人不能为空", itemIndex+1)
		}
		if strings.TrimSpace(finding.ID) == "" || findingIDs[finding.ID] {
			return NewError(CodeDuplicate, "第 %d 项复核记录 ID 无效或已存在", itemIndex+1)
		}
		findingIDs[finding.ID] = true
		if seen[finding.MeasureID] {
			return NewError(CodeDuplicate, "批量请求包含重复措施 %s", finding.MeasureID)
		}
		seen[finding.MeasureID] = true
		if !currentMeasures[finding.MeasureID] {
			return NewError(CodeNotFound, "第 %d 项措施 %s 不属于当前送审修订", itemIndex+1, finding.MeasureID)
		}
		if reviewed[finding.MeasureID] {
			return NewError(CodeDuplicate, "措施 %s 已有复核结论", finding.MeasureID)
		}
		finding.Issue = strings.TrimSpace(finding.Issue)
		if finding.Decision == DecisionReturn && finding.Issue == "" {
			return NewError(CodeValidation, "第 %d 项退回措施必须记录具体问题", itemIndex+1)
		}
	}
	for _, finding := range findings {
		finding.CaseID, finding.ReviewedAt = c.ID, now.UTC()
		if finding.Decision == DecisionReturn {
			finding.VerificationDecision = VerificationPending
		} else {
			finding.VerificationDecision = VerificationPass
			closed := now.UTC()
			finding.ClosedAt = &closed
		}
		c.Findings = append(c.Findings, finding)
	}
	c.recalculateReviewState()
	c.bump(now)
	return nil
}

func (c *RestorationCase) recalculateReviewState() {
	currentMeasures := 0
	for _, measure := range c.Measures {
		if measure.Revision == c.SubmittedRevision {
			currentMeasures++
		}
	}
	currentFindings := 0
	hasOpen := false
	for _, finding := range c.Findings {
		belongs := false
		for _, measure := range c.Measures {
			if measure.ID == finding.MeasureID && measure.Revision == c.SubmittedRevision {
				belongs = true
				break
			}
		}
		if !belongs {
			continue
		}
		currentFindings++
		if finding.Decision == DecisionReturn && finding.VerificationDecision != VerificationPass {
			hasOpen = true
		}
	}
	if hasOpen {
		c.Status = StatusRemediation
		return
	}
	if currentMeasures > 0 && currentFindings == currentMeasures {
		c.Status = StatusReviewPassed
	}
}

func (c *RestorationCase) AddRemediation(findingID, evidence string, now time.Time) error {
	if err := c.ensureMutable(StatusRemediation); err != nil {
		return err
	}
	if strings.TrimSpace(evidence) == "" {
		return NewError(CodeValidation, "整改证据不能为空")
	}
	for i := range c.Findings {
		if c.Findings[i].ID == findingID && c.Findings[i].Decision == DecisionReturn {
			c.Findings[i].RemediationEvidence = strings.TrimSpace(evidence)
			c.Findings[i].VerificationDecision = VerificationPending
			c.bump(now)
			return nil
		}
	}
	return NewError(CodeNotFound, "退回复核项不存在")
}

func (c *RestorationCase) VerifyRemediation(findingID, reviewer string, decision VerificationDecision, now time.Time) error {
	if err := c.ensureMutable(StatusRemediation); err != nil {
		return err
	}
	if decision != VerificationPass && decision != VerificationFail {
		return NewError(CodeValidation, "复验结论无效")
	}
	if strings.TrimSpace(reviewer) == "" {
		return NewError(CodeValidation, "复验人不能为空")
	}
	for i := range c.Findings {
		finding := &c.Findings[i]
		if finding.ID != findingID {
			continue
		}
		if finding.Decision != DecisionReturn || strings.TrimSpace(finding.RemediationEvidence) == "" {
			return NewError(CodeValidation, "退回项尚未提交整改证据")
		}
		finding.Reviewer = strings.TrimSpace(reviewer)
		finding.VerificationDecision = decision
		if decision == VerificationPass {
			closed := now.UTC()
			finding.ClosedAt = &closed
		} else {
			finding.ClosedAt = nil
		}
		c.recalculateReviewState()
		c.bump(now)
		return nil
	}
	return NewError(CodeNotFound, "复核项不存在")
}

func (c *RestorationCase) CanFreeze() error {
	if c.Status != StatusReviewPassed {
		return NewError(CodeState, "只有全部复核通过且整改闭环后才能冻结")
	}
	if len(c.Findings) == 0 {
		return NewError(CodeValidation, "缺少复核结论")
	}
	for _, finding := range c.Findings {
		if finding.Decision == DecisionReturn && (finding.VerificationDecision != VerificationPass || finding.ClosedAt == nil) {
			return NewError(CodeState, "仍有未闭环问题")
		}
	}
	return nil
}
