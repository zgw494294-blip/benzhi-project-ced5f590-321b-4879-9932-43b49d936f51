package domain

import "time"

type RestorationCase struct {
	ID                string              `json:"id"`
	TreeCode          string              `json:"treeCode"`
	Location          string              `json:"location"`
	ProtectionGrade   string              `json:"protectionGrade"`
	Owner             string              `json:"owner"`
	WorkWindowStart   time.Time           `json:"workWindowStart"`
	WorkWindowEnd     time.Time           `json:"workWindowEnd"`
	Status            CaseStatus          `json:"status"`
	Version           int64               `json:"version"`
	SubmittedRevision int                 `json:"submittedRevision"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
	Surveys           []SurveyObservation `json:"surveys"`
	Risks             []RiskItem          `json:"risks"`
	Measures          []TreatmentMeasure  `json:"measures"`
	Findings          []ReviewFinding     `json:"findings"`
	Frozen            *FrozenManifest     `json:"frozen,omitempty"`
	Permit            *ReleasePermit      `json:"permit,omitempty"`
}

type SurveyObservation struct {
	ID               string     `json:"id"`
	CaseID           string     `json:"caseId"`
	Area             SurveyArea `json:"area"`
	ConditionCode    string     `json:"conditionCode"`
	Severity         Severity   `json:"severity"`
	Extent           string     `json:"extent"`
	Notes            string     `json:"notes"`
	EvidenceRefs     []string   `json:"evidenceRefs"`
	ObservedBy       string     `json:"observedBy"`
	ObservedAt       time.Time  `json:"observedAt"`
	SupersedesID     string     `json:"supersedesId,omitempty"`
	SupersededByID   string     `json:"supersededById,omitempty"`
	CorrectionReason string     `json:"correctionReason,omitempty"`
}

type RiskItem struct {
	ID                  string     `json:"id"`
	CaseID              string     `json:"caseId"`
	SourceObservationID string     `json:"sourceObservationId,omitempty"`
	Category            string     `json:"category"`
	Severity            Severity   `json:"severity"`
	Urgency             Urgency    `json:"urgency"`
	Rationale           string     `json:"rationale"`
	Status              RiskStatus `json:"status"`
	CreatedAt           time.Time  `json:"createdAt"`
}

type TreatmentMeasure struct {
	ID                 string    `json:"id"`
	CaseID             string    `json:"caseId"`
	RiskID             string    `json:"riskId"`
	Revision           int       `json:"revision"`
	Sequence           int       `json:"sequence"`
	Action             string    `json:"action"`
	Prohibitions       string    `json:"prohibitions"`
	AcceptanceCriteria string    `json:"acceptanceCriteria"`
	PreparedBy         string    `json:"preparedBy"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type ReviewFinding struct {
	ID                   string               `json:"id"`
	CaseID               string               `json:"caseId"`
	MeasureID            string               `json:"measureId"`
	Decision             ReviewDecision       `json:"decision"`
	Issue                string               `json:"issue,omitempty"`
	RemediationEvidence  string               `json:"remediationEvidence,omitempty"`
	VerificationDecision VerificationDecision `json:"verificationDecision"`
	Reviewer             string               `json:"reviewer"`
	ReviewedAt           time.Time            `json:"reviewedAt"`
	ClosedAt             *time.Time           `json:"closedAt,omitempty"`
}

type FrozenManifest struct {
	CaseID        string    `json:"caseId"`
	FrozenVersion int64     `json:"frozenVersion"`
	SchemaVersion string    `json:"schemaVersion"`
	CanonicalJSON string    `json:"canonicalJson"`
	ContentDigest string    `json:"contentDigest"`
	FrozenBy      string    `json:"frozenBy"`
	FrozenAt      time.Time `json:"frozenAt"`
}

type ReleasePermit struct {
	ID            string    `json:"id"`
	CaseID        string    `json:"caseId"`
	SerialNumber  int64     `json:"serialNumber"`
	FrozenVersion int64     `json:"frozenVersion"`
	ContentDigest string    `json:"contentDigest"`
	ApprovedBy    string    `json:"approvedBy"`
	IssuedAt      time.Time `json:"issuedAt"`
	SchemaVersion string    `json:"schemaVersion"`
}

type AuditEvent struct {
	ID           string         `json:"id"`
	CaseID       string         `json:"caseId"`
	Type         string         `json:"type"`
	Actor        string         `json:"actor"`
	Role         Role           `json:"role"`
	BeforeStatus CaseStatus     `json:"beforeStatus"`
	AfterStatus  CaseStatus     `json:"afterStatus"`
	Version      int64          `json:"version"`
	OccurredAt   time.Time      `json:"occurredAt"`
	Details      map[string]any `json:"details,omitempty"`
}
