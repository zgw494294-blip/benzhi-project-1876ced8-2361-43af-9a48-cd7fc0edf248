package application

import "time"

type CreateSessionCommand struct {
	Title          string    `json:"title"`
	Venue          string    `json:"venue"`
	PerformanceAt  time.Time `json:"performanceAt"`
	OperatorID     string    `json:"operatorId"`
	RuleSetVersion string    `json:"ruleSetVersion"`
	IdempotencyKey string    `json:"-"`
}
type VersionCommand struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"-"`
	ActorID         string `json:"actorId,omitempty"`
}
type BaselineCommand struct {
	VersionCommand
	BaselineRef string `json:"baselineRef"`
}
type AddLineCommand struct {
	VersionCommand
	Code                      string `json:"code"`
	RatedLoadGram             int64  `json:"ratedLoadGram"`
	SpanMillimeter            int64  `json:"spanMillimeter"`
	MaxMomentNewtonMillimeter int64  `json:"maxMomentNewtonMillimeter"`
}
type AddPointCommand struct {
	VersionCommand
	LineID             string `json:"lineId"`
	Code               string `json:"code"`
	HoistRatedLoadGram int64  `json:"hoistRatedLoadGram"`
	PositionMillimeter int64  `json:"positionMillimeter"`
}
type AddLoadCommand struct {
	VersionCommand
	LineID             string `json:"lineId"`
	PointID            string `json:"pointId"`
	ComponentCode      string `json:"componentCode"`
	Description        string `json:"description"`
	WeightGram         int64  `json:"weightGram"`
	PositionMillimeter int64  `json:"positionMillimeter"`
	Quantity           int64  `json:"quantity"`
	SubmittedBy        string `json:"submittedBy"`
}
type AddLoadInput struct {
	LineID             string `json:"lineId"`
	PointID            string `json:"pointId"`
	ComponentCode      string `json:"componentCode"`
	Description        string `json:"description"`
	WeightGram         int64  `json:"weightGram"`
	PositionMillimeter int64  `json:"positionMillimeter"`
	Quantity           int64  `json:"quantity"`
	SubmittedBy        string `json:"submittedBy"`
}
type AddLoadsCommand struct {
	VersionCommand
	Loads []AddLoadInput `json:"loads"`
}
type CheckCommand struct {
	VersionCommand
	LineID      string `json:"lineId"`
	Kind        string `json:"kind"`
	Passed      bool   `json:"passed"`
	Measurement string `json:"measurement"`
	Evidence    string `json:"evidence"`
	InspectorID string `json:"inspectorId"`
}
type RemediationCommand struct {
	VersionCommand
	FindingID  string `json:"findingId"`
	AssigneeID string `json:"assigneeId"`
	Note       string `json:"note"`
}
type ReviseLoadCommand struct {
	VersionCommand
	LoadID             string `json:"loadId"`
	WeightGram         int64  `json:"weightGram"`
	PositionMillimeter int64  `json:"positionMillimeter"`
}
type LoadRevisionInput struct {
	LoadID             string `json:"loadId"`
	WeightGram         int64  `json:"weightGram"`
	PositionMillimeter int64  `json:"positionMillimeter"`
}
type PreviewLoadPlanCommand struct {
	Revisions []LoadRevisionInput `json:"revisions"`
}
type ApplyLoadPlanCommand struct {
	VersionCommand
	Revisions      []LoadRevisionInput `json:"revisions"`
	ProposalDigest string              `json:"proposalDigest"`
}
type ReviewCommand struct {
	VersionCommand
	ReviewerID           string   `json:"reviewerId"`
	Decision             string   `json:"decision"`
	Reason               string   `json:"reason"`
	Category             string   `json:"category,omitempty"`
	AffectedLineIDs      []string `json:"affectedLineIds,omitempty"`
	ConfirmationID       string   `json:"confirmationId,omitempty"`
	ReviewConfirmationID string   `json:"reviewConfirmationId,omitempty"`
}
