package domain

import (
	"fmt"
	"strings"
	"time"
)

type SurveyAreaReadiness struct {
	Area      SurveyArea          `json:"area"`
	Effective []SurveyObservation `json:"effective"`
	History   []SurveyObservation `json:"history"`
}

type SurveyPreflight struct {
	Ready        bool                  `json:"ready"`
	Areas        []SurveyAreaReadiness `json:"areas"`
	MissingAreas []SurveyArea          `json:"missingAreas"`
	Blockers     []string              `json:"blockers"`
}

func validArea(area SurveyArea) bool {
	for _, required := range RequiredSurveyAreas {
		if area == required {
			return true
		}
	}
	return false
}

func (c *RestorationCase) AddObservation(observation SurveyObservation, now time.Time) error {
	if err := c.ensureMutable(StatusDraft); err != nil {
		return err
	}
	if strings.TrimSpace(observation.ID) == "" {
		return NewError(CodeValidation, "调查记录 ID 不能为空")
	}
	if !validArea(observation.Area) || !observation.Severity.Valid() {
		return NewError(CodeValidation, "调查区域或严重度无效")
	}
	if strings.TrimSpace(observation.ConditionCode) == "" || strings.TrimSpace(observation.Extent) == "" || strings.TrimSpace(observation.Notes) == "" {
		return NewError(CodeValidation, "调查现象、影响范围和现场说明不能为空")
	}
	if len(observation.EvidenceRefs) == 0 || strings.TrimSpace(observation.ObservedBy) == "" {
		return NewError(CodeValidation, "调查证据和调查人不能为空")
	}
	for _, existing := range c.Surveys {
		if existing.ID == observation.ID {
			return NewError(CodeDuplicate, "调查记录 ID 已存在")
		}
	}
	observation.CaseID = c.ID
	observation.ObservedAt = observation.ObservedAt.UTC()
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = now.UTC()
	}
	c.Surveys = append(c.Surveys, observation)
	c.bump(now)
	return nil
}

func (c *RestorationCase) CorrectObservation(observation SurveyObservation, supersedesID, reason string, now time.Time) error {
	if err := c.ensureMutable(StatusDraft); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if strings.TrimSpace(supersedesID) == "" || reason == "" {
		return NewError(CodeValidation, "被更正记录和更正理由不能为空")
	}
	targetIndex := -1
	for i := range c.Surveys {
		if c.Surveys[i].ID == supersedesID {
			targetIndex = i
		}
		if c.Surveys[i].ID == observation.ID {
			return NewError(CodeDuplicate, "调查记录 ID 已存在")
		}
	}
	if targetIndex < 0 {
		return NewError(CodeNotFound, "被更正调查记录不存在")
	}
	target := c.Surveys[targetIndex]
	if target.Area != observation.Area {
		return NewError(CodeValidation, "更正记录区域 %s 与原记录区域 %s 不一致", observation.Area, target.Area)
	}
	if target.SupersededByID != "" {
		return NewError(CodeState, "调查记录 %s 已被 %s 取代", target.ID, target.SupersededByID)
	}
	for cursor := target; cursor.SupersedesID != ""; {
		if cursor.SupersedesID == observation.ID {
			return NewError(CodeValidation, "调查记录取代关系不能形成循环")
		}
		found := false
		for _, candidate := range c.Surveys {
			if candidate.ID == cursor.SupersedesID {
				cursor, found = candidate, true
				break
			}
		}
		if !found {
			return NewError(CodeValidation, "调查记录 %s 的取代关系不完整", cursor.ID)
		}
	}
	observation.SupersedesID = supersedesID
	observation.CorrectionReason = reason
	if err := c.AddObservation(observation, now); err != nil {
		return err
	}
	c.Surveys[targetIndex].SupersededByID = observation.ID
	return nil
}

func validateObservation(observation SurveyObservation) string {
	if !validArea(observation.Area) {
		return "调查区域无效"
	}
	if !observation.Severity.Valid() {
		return "严重度无效"
	}
	if strings.TrimSpace(observation.ConditionCode) == "" || strings.TrimSpace(observation.Extent) == "" || strings.TrimSpace(observation.Notes) == "" {
		return "结构化现象、影响范围或文本说明缺失"
	}
	if len(observation.EvidenceRefs) == 0 || strings.TrimSpace(observation.ObservedBy) == "" {
		return "证据引用或调查人缺失"
	}
	return ""
}

func (c *RestorationCase) SurveyPreflight() SurveyPreflight {
	result := SurveyPreflight{}
	evidenceOwner := make(map[string]string)
	for _, area := range RequiredSurveyAreas {
		areaResult := SurveyAreaReadiness{Area: area}
		for _, observation := range c.Surveys {
			if observation.Area != area {
				continue
			}
			if observation.SupersededByID == "" {
				areaResult.Effective = append(areaResult.Effective, observation)
			} else {
				areaResult.History = append(areaResult.History, observation)
			}
		}
		if len(areaResult.Effective) == 0 {
			result.MissingAreas = append(result.MissingAreas, area)
			result.Blockers = append(result.Blockers, fmt.Sprintf("%s 区域缺少有效调查记录", area))
		}
		if len(areaResult.Effective) > 1 {
			result.Blockers = append(result.Blockers, fmt.Sprintf("%s 区域存在 %d 条未决有效记录", area, len(areaResult.Effective)))
		}
		for _, observation := range areaResult.Effective {
			if issue := validateObservation(observation); issue != "" {
				result.Blockers = append(result.Blockers, fmt.Sprintf("%s 记录 %s：%s", area, observation.ID, issue))
			}
			local := make(map[string]bool)
			for _, reference := range observation.EvidenceRefs {
				reference = strings.TrimSpace(reference)
				if reference == "" {
					result.Blockers = append(result.Blockers, fmt.Sprintf("%s 记录 %s：证据引用不能为空", area, observation.ID))
					continue
				}
				if local[reference] || evidenceOwner[reference] != "" {
					result.Blockers = append(result.Blockers, fmt.Sprintf("证据引用 %s 重复（记录 %s）", reference, observation.ID))
				} else {
					evidenceOwner[reference] = observation.ID
				}
				local[reference] = true
			}
		}
		result.Areas = append(result.Areas, areaResult)
	}
	result.Ready = len(result.Blockers) == 0
	return result
}

func (c *RestorationCase) CompleteSurvey(now time.Time) error {
	if err := c.ensureMutable(StatusDraft); err != nil {
		return err
	}
	preflight := c.SurveyPreflight()
	if !preflight.Ready {
		return NewError(CodeValidation, "现场快照尚未就绪：%s", strings.Join(preflight.Blockers, "；"))
	}
	c.Status = StatusSurveyComplete
	c.bump(now)
	return nil
}
