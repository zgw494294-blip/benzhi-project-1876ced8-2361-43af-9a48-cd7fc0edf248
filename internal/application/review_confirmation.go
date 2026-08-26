package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"rigging-readiness-desk/internal/domain"
	"sort"
)

type ReviewConfirmation struct {
	ID                       string `json:"id"`
	ReviewConfirmationID     string `json:"reviewConfirmationId"`
	SessionID                string `json:"sessionId"`
	Version                  int64  `json:"version"`
	CalculationInputDigest   string `json:"calculationInputDigest"`
	InspectionCoverageDigest string `json:"inspectionCoverageDigest"`
	RemediationClosureDigest string `json:"remediationClosureDigest"`
}

func (s *Service) GetReviewConfirmation(ctx context.Context, id string) (*ReviewConfirmation, error) {
	session, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return BuildReviewConfirmation(session)
}

func BuildReviewConfirmation(session *domain.RiggingSession) (*ReviewConfirmation, error) {
	if session.Status != domain.StatusInspected {
		return nil, domain.NewError(domain.ErrState, "status", "只有完成检查的作业可生成复核确认标识")
	}
	if session.Calculation == nil {
		return nil, domain.NewError(domain.ErrState, "calculation", "载荷计算不存在")
	}
	if session.HasOpenFindings() {
		return nil, domain.NewError(domain.ErrState, "findings", "仍有未关闭阻断项")
	}
	checks := append([]domain.LineCheck(nil), session.Checks...)
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].LineID == checks[j].LineID {
			return checks[i].Kind < checks[j].Kind
		}
		return checks[i].LineID < checks[j].LineID
	})
	checkData, _ := json.Marshal(checks)
	checkHash := sha256.Sum256(checkData)
	findings := append([]domain.SafetyFinding(nil), session.Findings...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	findingData, _ := json.Marshal(findings)
	findingHash := sha256.Sum256(findingData)
	confirmation := &ReviewConfirmation{SessionID: session.ID, Version: session.Version, CalculationInputDigest: session.Calculation.InputDigest, InspectionCoverageDigest: hex.EncodeToString(checkHash[:]), RemediationClosureDigest: hex.EncodeToString(findingHash[:])}
	payload, _ := json.Marshal(struct {
		SessionID   string `json:"sessionId"`
		Version     int64  `json:"version"`
		Calculation string `json:"calculation"`
		Inspection  string `json:"inspection"`
		Remediation string `json:"remediation"`
	}{confirmation.SessionID, confirmation.Version, confirmation.CalculationInputDigest, confirmation.InspectionCoverageDigest, confirmation.RemediationClosureDigest})
	sum := sha256.Sum256(payload)
	confirmation.ID = hex.EncodeToString(sum[:])
	confirmation.ReviewConfirmationID = confirmation.ID
	return confirmation, nil
}
