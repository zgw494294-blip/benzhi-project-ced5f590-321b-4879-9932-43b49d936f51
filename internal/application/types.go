package application

import (
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

type CommandMeta struct {
	Actor           string      `json:"actor"`
	Role            domain.Role `json:"role"`
	ExpectedVersion int64       `json:"expectedVersion"`
	IdempotencyKey  string      `json:"idempotencyKey"`
}

type CreateCaseCommand struct {
	CommandMeta
	ID              string    `json:"id"`
	TreeCode        string    `json:"treeCode"`
	Location        string    `json:"location"`
	ProtectionGrade string    `json:"protectionGrade"`
	Owner           string    `json:"owner"`
	WorkWindowStart time.Time `json:"workWindowStart"`
	WorkWindowEnd   time.Time `json:"workWindowEnd"`
}

type AddSurveyCommand struct {
	CommandMeta
	ID            string            `json:"id"`
	Area          domain.SurveyArea `json:"area"`
	ConditionCode string            `json:"conditionCode"`
	Severity      domain.Severity   `json:"severity"`
	Extent        string            `json:"extent"`
	Notes         string            `json:"notes"`
	EvidenceRefs  []string          `json:"evidenceRefs"`
	ObservedAt    time.Time         `json:"observedAt"`
}

type CorrectSurveyCommand struct {
	CommandMeta
	ID               string            `json:"id"`
	SupersedesID     string            `json:"supersedesId"`
	CorrectionReason string            `json:"correctionReason"`
	Area             domain.SurveyArea `json:"area"`
	ConditionCode    string            `json:"conditionCode"`
	Severity         domain.Severity   `json:"severity"`
	Extent           string            `json:"extent"`
	Notes            string            `json:"notes"`
	EvidenceRefs     []string          `json:"evidenceRefs"`
	ObservedAt       time.Time         `json:"observedAt"`
}

type AddRiskCommand struct {
	CommandMeta
	ID        string          `json:"id"`
	Category  string          `json:"category"`
	Severity  domain.Severity `json:"severity"`
	Urgency   domain.Urgency  `json:"urgency"`
	Rationale string          `json:"rationale"`
}

type BatchRiskUrgencyCommand struct {
	CommandMeta
	Adjustments []domain.RiskUrgencyAdjustment `json:"adjustments"`
}

type UpsertMeasureCommand struct {
	CommandMeta
	ID                 string `json:"id"`
	RiskID             string `json:"riskId"`
	Revision           int    `json:"revision"`
	Sequence           int    `json:"sequence"`
	Action             string `json:"action"`
	Prohibitions       string `json:"prohibitions"`
	AcceptanceCriteria string `json:"acceptanceCriteria"`
}

type ReviewCommand struct {
	CommandMeta
	ID        string                `json:"id"`
	MeasureID string                `json:"measureId"`
	Decision  domain.ReviewDecision `json:"decision"`
	Issue     string                `json:"issue"`
}

type BatchReviewItem struct {
	ID        string                `json:"id"`
	MeasureID string                `json:"measureId"`
	Decision  domain.ReviewDecision `json:"decision"`
	Issue     string                `json:"issue"`
}

type BatchReviewCommand struct {
	CommandMeta
	Items []BatchReviewItem `json:"items"`
}

type RemediationCommand struct {
	CommandMeta
	FindingID string `json:"findingId"`
	Evidence  string `json:"evidence"`
}

type VerificationCommand struct {
	CommandMeta
	FindingID string                      `json:"findingId"`
	Decision  domain.VerificationDecision `json:"decision"`
}

// deleteRiskPayload 和 submitReviewPayload 仅用于计算幂等请求摘要，分别捕获
// 路径参数 riskId 与 revision，确保同一幂等键下请求内容不同时能被识别为冲突。
type deleteRiskPayload struct {
	CommandMeta
	RiskID string `json:"riskId"`
}

type submitReviewPayload struct {
	CommandMeta
	Revision int `json:"revision"`
}

type CaseDetails struct {
	Case             *domain.RestorationCase  `json:"case"`
	Progress         CaseProgress             `json:"progress"`
	AuditTrail       []domain.AuditEvent      `json:"auditTrail"`
	AuditIntegrity   audit.TrailVerification  `json:"auditIntegrity"`
	ManifestSummary  audit.ManifestSummary    `json:"manifestSummary"`
	Verification     audit.VerificationResult `json:"verification"`
	SurveyPreflight  domain.SurveyPreflight   `json:"surveyPreflight"`
	RiskWorklist     RiskWorklist             `json:"riskWorklist"`
	ReviewProgress   ReviewProgress           `json:"reviewProgress"`
	RemediationTodos RemediationTodos         `json:"remediationTodos"`
}
