package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

type TrailVerification struct {
	Valid        bool      `json:"valid"`
	EventCount   int       `json:"eventCount"`
	FirstVersion int64     `json:"firstVersion,omitempty"`
	LastVersion  int64     `json:"lastVersion,omitempty"`
	FirstAt      time.Time `json:"firstAt,omitempty"`
	LastAt       time.Time `json:"lastAt,omitempty"`
	EventDigests []string  `json:"eventDigests"`
	Issues       []string  `json:"issues"`
}

type eventEnvelope struct {
	ID           string            `json:"id"`
	CaseID       string            `json:"caseId"`
	Type         string            `json:"type"`
	Actor        string            `json:"actor"`
	Role         domain.Role       `json:"role"`
	BeforeStatus domain.CaseStatus `json:"beforeStatus"`
	AfterStatus  domain.CaseStatus `json:"afterStatus"`
	Version      int64             `json:"version"`
	OccurredAt   time.Time         `json:"occurredAt"`
	Details      map[string]any    `json:"details,omitempty"`
}

func EventDigest(event domain.AuditEvent) (string, error) {
	payload := eventEnvelope{
		ID: event.ID, CaseID: event.CaseID, Type: event.Type, Actor: event.Actor, Role: event.Role,
		BeforeStatus: event.BeforeStatus, AfterStatus: event.AfterStatus, Version: event.Version,
		OccurredAt: event.OccurredAt.UTC(), Details: event.Details,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func VerifyTrail(restoration *domain.RestorationCase, events []domain.AuditEvent) TrailVerification {
	result := TrailVerification{EventCount: len(events), EventDigests: make([]string, 0, len(events))}
	if restoration == nil {
		result.Issues = append(result.Issues, "缺少作业聚合")
		return result
	}
	if len(events) == 0 {
		result.Issues = append(result.Issues, "审计轨迹为空")
		return result
	}
	result.FirstVersion, result.LastVersion = events[0].Version, events[len(events)-1].Version
	result.FirstAt, result.LastAt = events[0].OccurredAt, events[len(events)-1].OccurredAt
	seenIDs := make(map[string]bool, len(events))
	seenTypes := make(map[string]bool)
	var previous *domain.AuditEvent
	for index := range events {
		event := &events[index]
		seenTypes[event.Type] = true
		if event.ID == "" || seenIDs[event.ID] {
			result.Issues = append(result.Issues, fmt.Sprintf("第 %d 条审计事件 ID 缺失或重复", index+1))
		}
		seenIDs[event.ID] = true
		if event.CaseID != restoration.ID {
			result.Issues = append(result.Issues, fmt.Sprintf("事件 %s 不属于当前作业", event.ID))
		}
		if event.Actor == "" || event.Role == "" {
			result.Issues = append(result.Issues, fmt.Sprintf("事件 %s 缺少操作者或角色", event.ID))
		}
		if event.OccurredAt.IsZero() {
			result.Issues = append(result.Issues, fmt.Sprintf("事件 %s 缺少发生时间", event.ID))
		}
		if previous != nil {
			if event.Version != previous.Version+1 {
				result.Issues = append(result.Issues, fmt.Sprintf("版本序列在 %d 与 %d 之间不连续", previous.Version, event.Version))
			}
			if event.BeforeStatus != previous.AfterStatus {
				result.Issues = append(result.Issues, fmt.Sprintf("事件 %s 的前置状态与上一事件不连续", event.ID))
			}
			if event.OccurredAt.Before(previous.OccurredAt) {
				result.Issues = append(result.Issues, fmt.Sprintf("事件 %s 的发生时间逆序", event.ID))
			}
		}
		digest, err := EventDigest(*event)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("事件 %s 无法计算摘要", event.ID))
		} else {
			result.EventDigests = append(result.EventDigests, digest)
		}
		previous = event
	}
	if events[0].Type != "CASE_CREATED" || events[0].Version != 1 {
		result.Issues = append(result.Issues, "审计轨迹必须从版本 1 的建档事件开始")
	}
	if result.LastVersion != restoration.Version {
		result.Issues = append(result.Issues, "审计末版本与作业聚合版本不一致")
	}
	if events[len(events)-1].AfterStatus != restoration.Status {
		result.Issues = append(result.Issues, "审计末状态与作业当前状态不一致")
	}
	requireEvent := func(eventType string, needed bool) {
		if needed && !seenTypes[eventType] {
			result.Issues = append(result.Issues, "缺少必要审计事件 "+eventType)
		}
	}
	requireEvent("SURVEY_COMPLETED", restoration.Status != domain.StatusDraft)
	requireEvent("REVIEW_SUBMITTED", restoration.SubmittedRevision > 0)
	requireEvent("VERSION_FROZEN", restoration.Frozen != nil)
	requireEvent("PERMIT_ISSUED", restoration.Permit != nil)
	result.Valid = len(result.Issues) == 0
	return result
}

type ManifestSummary struct {
	Available        bool           `json:"available"`
	SchemaVersion    string         `json:"schemaVersion,omitempty"`
	FrozenVersion    int64          `json:"frozenVersion,omitempty"`
	ContentDigest    string         `json:"contentDigest,omitempty"`
	SurveyCount      int            `json:"surveyCount"`
	SurveyAreaCounts map[string]int `json:"surveyAreaCounts"`
	RiskCount        int            `json:"riskCount"`
	MeasureCount     int            `json:"measureCount"`
	FindingCount     int            `json:"findingCount"`
	MeasureSequence  []int          `json:"measureSequence"`
	ParseIssue       string         `json:"parseIssue,omitempty"`
}

func InspectManifest(restoration *domain.RestorationCase) ManifestSummary {
	result := ManifestSummary{SurveyAreaCounts: make(map[string]int)}
	if restoration == nil || restoration.Frozen == nil {
		return result
	}
	result.Available = true
	result.SchemaVersion = restoration.Frozen.SchemaVersion
	result.FrozenVersion = restoration.Frozen.FrozenVersion
	result.ContentDigest = restoration.Frozen.ContentDigest
	var payload canonicalCase
	if err := json.Unmarshal([]byte(restoration.Frozen.CanonicalJSON), &payload); err != nil {
		result.ParseIssue = "冻结清单 JSON 无法解析"
		return result
	}
	if payload.CaseID != restoration.ID {
		result.ParseIssue = "冻结清单作业 ID 不匹配"
		return result
	}
	result.SurveyCount = len(payload.Surveys)
	result.RiskCount = len(payload.Risks)
	result.MeasureCount = len(payload.Measures)
	result.FindingCount = len(payload.Findings)
	for _, observation := range payload.Surveys {
		result.SurveyAreaCounts[string(observation.Area)]++
	}
	for _, measure := range payload.Measures {
		result.MeasureSequence = append(result.MeasureSequence, measure.Sequence)
	}
	sort.Ints(result.MeasureSequence)
	return result
}
