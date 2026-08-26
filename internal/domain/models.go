package domain

import "time"

type RiggingSession struct {
	ID             string               `json:"id"`
	Title          string               `json:"title"`
	Venue          string               `json:"venue"`
	PerformanceAt  time.Time            `json:"performanceAt"`
	OperatorID     string               `json:"operatorId"`
	RuleSetVersion string               `json:"ruleSetVersion"`
	Status         SessionStatus        `json:"status"`
	Version        int64                `json:"version"`
	BaselineRef    string               `json:"baselineRef,omitempty"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
	Lines          []RiggingLine        `json:"lines"`
	Points         []RiggingPoint       `json:"points"`
	Loads          []SuspendedLoad      `json:"loads"`
	Checks         []LineCheck          `json:"checks"`
	Findings       []SafetyFinding      `json:"findings"`
	Calculation    *CalculationSnapshot `json:"calculation,omitempty"`
	Review         *SafetyReview        `json:"review,omitempty"`
	Frozen         *FrozenManifest      `json:"frozen,omitempty"`
	Certificate    *ReleaseCertificate  `json:"certificate,omitempty"`
}
type RiggingLine struct {
	ID                               string `json:"id"`
	SessionID                        string `json:"sessionId"`
	Code                             string `json:"code"`
	RatedLoadGram                    int64  `json:"ratedLoadGram"`
	SpanMillimeter                   int64  `json:"spanMillimeter"`
	MaxMomentNewtonMillimeter        int64  `json:"maxMomentNewtonMillimeter"`
	TotalLoadGram                    int64  `json:"totalLoadGram"`
	UtilizationPPM                   int64  `json:"utilizationPpm"`
	CalculatedMomentNewtonMillimeter int64  `json:"calculatedMomentNewtonMillimeter"`
	SafetyMarginPPM                  int64  `json:"safetyMarginPpm"`
}
type RiggingPoint struct {
	ID                 string `json:"id"`
	SessionID          string `json:"sessionId"`
	LineID             string `json:"lineId"`
	Code               string `json:"code"`
	HoistRatedLoadGram int64  `json:"hoistRatedLoadGram"`
	PositionMillimeter int64  `json:"positionMillimeter"`
}
type SuspendedLoad struct {
	ID                 string    `json:"id"`
	SessionID          string    `json:"sessionId"`
	LineID             string    `json:"lineId"`
	PointID            string    `json:"pointId"`
	ComponentCode      string    `json:"componentCode"`
	Description        string    `json:"description"`
	WeightGram         int64     `json:"weightGram"`
	PositionMillimeter int64     `json:"positionMillimeter"`
	Quantity           int64     `json:"quantity"`
	SubmittedBy        string    `json:"submittedBy"`
	CreatedAt          time.Time `json:"createdAt"`
}
type CheckKind string

const (
	CheckBrake     CheckKind = "BRAKE"
	CheckWireRope  CheckKind = "WIRE_ROPE"
	CheckConnector CheckKind = "CONNECTOR"
	CheckLimit     CheckKind = "LIMIT"
	CheckClearance CheckKind = "CLEARANCE"
)

var RequiredChecks = []CheckKind{CheckBrake, CheckWireRope, CheckConnector, CheckLimit, CheckClearance}

type LineCheck struct {
	ID          string    `json:"id"`
	LineID      string    `json:"lineId"`
	Kind        CheckKind `json:"kind"`
	Passed      bool      `json:"passed"`
	Measurement string    `json:"measurement"`
	Evidence    string    `json:"evidence"`
	InspectorID string    `json:"inspectorId"`
	CheckedAt   time.Time `json:"checkedAt"`
}
type FindingStatus string

const (
	FindingOpen   FindingStatus = "OPEN"
	FindingClosed FindingStatus = "CLOSED"
)

type SafetyFinding struct {
	ID              string        `json:"id"`
	SessionID       string        `json:"sessionId"`
	LineID          string        `json:"lineId"`
	SourceType      string        `json:"sourceType"`
	Severity        string        `json:"severity"`
	RuleCode        string        `json:"ruleCode"`
	Description     string        `json:"description"`
	OriginVersion   int64         `json:"originVersion,omitempty"`
	OriginActorID   string        `json:"originActorId,omitempty"`
	Status          FindingStatus `json:"status"`
	AssigneeID      string        `json:"assigneeId,omitempty"`
	RemediationNote string        `json:"remediationNote,omitempty"`
	VerifiedBy      string        `json:"verifiedBy,omitempty"`
	ClosedAt        *time.Time    `json:"closedAt,omitempty"`
}
type LineCalculation struct {
	LineID                 string   `json:"lineId"`
	LineCode               string   `json:"lineCode"`
	TotalLoadGram          int64    `json:"totalLoadGram"`
	EffectiveRatedLoadGram int64    `json:"effectiveRatedLoadGram"`
	UtilizationPPM         int64    `json:"utilizationPpm"`
	MomentNewtonMillimeter int64    `json:"momentNewtonMillimeter"`
	MomentUtilizationPPM   int64    `json:"momentUtilizationPpm"`
	SafetyMarginPPM        int64    `json:"safetyMarginPpm"`
	Passed                 bool     `json:"passed"`
	Reasons                []string `json:"reasons"`
}
type CalculationSnapshot struct {
	RuleSetVersion string            `json:"ruleSetVersion"`
	InputDigest    string            `json:"inputDigest"`
	CalculatedAt   time.Time         `json:"calculatedAt"`
	Lines          []LineCalculation `json:"lines"`
}
type SafetyReview struct {
	ReviewerID      string    `json:"reviewerId"`
	Decision        string    `json:"decision"`
	Reason          string    `json:"reason"`
	Category        string    `json:"category,omitempty"`
	AffectedLineIDs []string  `json:"affectedLineIds,omitempty"`
	ReviewedVersion int64     `json:"reviewedVersion"`
	ConfirmationID  string    `json:"confirmationId,omitempty"`
	ReviewedAt      time.Time `json:"reviewedAt"`
}
type ManifestLine struct {
	Sequence      int                 `json:"sequence"`
	LineCode      string              `json:"lineCode"`
	TotalLoadGram int64               `json:"totalLoadGram"`
	Components    []ManifestComponent `json:"components"`
}
type ManifestComponent struct {
	ComponentCode      string `json:"componentCode"`
	PointCode          string `json:"pointCode"`
	Description        string `json:"description"`
	WeightGram         int64  `json:"weightGram"`
	PositionMillimeter int64  `json:"positionMillimeter"`
	Quantity           int64  `json:"quantity"`
}
type FrozenManifest struct {
	Version  int64          `json:"version"`
	Digest   string         `json:"digest"`
	FrozenAt time.Time      `json:"frozenAt"`
	Lines    []ManifestLine `json:"lines"`
}
type ReleaseCertificate struct {
	ID                 string    `json:"id"`
	SessionID          string    `json:"sessionId"`
	FrozenVersion      int64     `json:"frozenVersion"`
	ManifestDigest     string    `json:"manifestDigest"`
	RuleSetVersion     string    `json:"ruleSetVersion"`
	ApprovedBy         string    `json:"approvedBy"`
	IssuedAt           time.Time `json:"issuedAt"`
	VerificationStatus string    `json:"verificationStatus"`
}
