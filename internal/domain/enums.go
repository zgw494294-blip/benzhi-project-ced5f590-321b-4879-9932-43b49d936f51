package domain

type CaseStatus string

const (
	StatusDraft          CaseStatus = "DRAFT"
	StatusSurveyComplete CaseStatus = "SURVEY_COMPLETE"
	StatusInReview       CaseStatus = "IN_REVIEW"
	StatusRemediation    CaseStatus = "REMEDIATION"
	StatusReviewPassed   CaseStatus = "REVIEW_PASSED"
	StatusFrozen         CaseStatus = "FROZEN"
	StatusReleased       CaseStatus = "RELEASED"
)

func (s CaseStatus) Editable() bool {
	return s == StatusDraft || s == StatusSurveyComplete || s == StatusRemediation
}

type SurveyArea string

const (
	AreaCanopy      SurveyArea = "CANOPY"
	AreaTrunk       SurveyArea = "TRUNK"
	AreaRootZone    SurveyArea = "ROOT_ZONE"
	AreaEnvironment SurveyArea = "ENVIRONMENT"
)

var RequiredSurveyAreas = []SurveyArea{AreaCanopy, AreaTrunk, AreaRootZone, AreaEnvironment}

type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

func (s Severity) Valid() bool {
	return s == SeverityLow || s == SeverityMedium || s == SeverityHigh || s == SeverityCritical
}

type Urgency string

const (
	UrgencyRoutine   Urgency = "ROUTINE"
	UrgencySoon      Urgency = "SOON"
	UrgencyImmediate Urgency = "IMMEDIATE"
)

func (u Urgency) Valid() bool {
	return u == UrgencyRoutine || u == UrgencySoon || u == UrgencyImmediate
}

type RiskPriority string

const (
	PriorityHigh    RiskPriority = "HIGH"
	PriorityMedium  RiskPriority = "MEDIUM"
	PriorityRoutine RiskPriority = "ROUTINE"
)

type WorkWindowStatus string

const (
	WindowNotStarted WorkWindowStatus = "NOT_STARTED"
	WindowActive     WorkWindowStatus = "ACTIVE"
	WindowExpired    WorkWindowStatus = "EXPIRED"
)

type RiskStatus string

const (
	RiskOpen    RiskStatus = "OPEN"
	RiskCovered RiskStatus = "COVERED"
)

type ReviewDecision string

const (
	DecisionPass   ReviewDecision = "PASS"
	DecisionReturn ReviewDecision = "RETURN"
)

type VerificationDecision string

const (
	VerificationPending VerificationDecision = "PENDING"
	VerificationPass    VerificationDecision = "PASS"
	VerificationFail    VerificationDecision = "FAIL"
)

type Role string

const (
	RolePatrol   Role = "PATROL"
	RolePlanner  Role = "PLANNER"
	RoleReviewer Role = "REVIEWER"
)
