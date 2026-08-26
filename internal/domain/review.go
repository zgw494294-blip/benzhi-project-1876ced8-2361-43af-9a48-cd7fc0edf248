package domain

import (
	"strings"
	"time"
)

var reviewCategories = map[string]string{
	"CALCULATION":      "CALCULATION",
	"LOAD_CALCULATION": "CALCULATION",
	"BRAKE":            "BRAKE",
	"WIRE_ROPE":        "WIRE_ROPE",
	"CONNECTOR":        "CONNECTOR",
	"LIMIT":            "LIMIT",
	"LIMIT_CHECK":      "LIMIT",
	"CLEARANCE":        "CLEARANCE",
}

func NormalizeReviewCategory(value string) (string, bool) {
	category, ok := reviewCategories[strings.ToUpper(strings.TrimSpace(value))]
	return category, ok
}

func (s *RiggingSession) ReturnReview(reviewer, reason, category string, lineIDs []string, reviewedVersion int64, now time.Time, idFactory func() string) error {
	if err := s.requireStatus(StatusInspected); err != nil {
		return err
	}
	if strings.TrimSpace(reviewer) == "" {
		return NewError(ErrValidation, "reviewerId", "复核员不能为空")
	}
	if reviewer == s.OperatorID {
		return NewError(ErrForbidden, "reviewerId", "复核员必须独立于经办人")
	}
	if strings.TrimSpace(reason) == "" {
		return NewError(ErrValidation, "reason", "退回必须填写理由")
	}
	normalized, ok := NormalizeReviewCategory(category)
	if !ok {
		return NewError(ErrValidation, "category", "未知退回问题类别")
	}
	if len(lineIDs) == 0 {
		return NewError(ErrValidation, "affectedLineIds", "退回必须选择至少一根受影响吊杆")
	}
	if s.HasOpenFindings() {
		return NewError(ErrState, "findings", "仍有未关闭阻断项，不能发起新的复核退回")
	}
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(lineIDs))
	for i, lineID := range lineIDs {
		if _, exists := seen[lineID]; exists {
			return NewError(ErrValidation, "affectedLineIds", "受影响吊杆不得重复")
		}
		if _, err := s.FindLine(lineID); err != nil {
			return NewError(ErrValidation, "affectedLineIds", "第 "+itoaDomain(i+1)+" 个吊杆不存在")
		}
		seen[lineID] = struct{}{}
		clean = append(clean, lineID)
		s.Findings = append(s.Findings, SafetyFinding{ID: idFactory(), SessionID: s.ID, LineID: lineID, SourceType: "REVIEW", Severity: "BLOCKING", RuleCode: "REVIEW_" + normalized, Description: strings.TrimSpace(reason), OriginVersion: reviewedVersion, OriginActorID: strings.TrimSpace(reviewer), Status: FindingOpen})
	}
	s.Review = &SafetyReview{ReviewerID: strings.TrimSpace(reviewer), Decision: "RETURN", Reason: strings.TrimSpace(reason), Category: normalized, AffectedLineIDs: clean, ReviewedVersion: reviewedVersion, ReviewedAt: now.UTC()}
	return nil
}

func (s *RiggingSession) ApproveConfirmed(reviewer, reason, confirmationID, expectedConfirmation string, reviewedVersion int64, now time.Time) error {
	if strings.TrimSpace(confirmationID) == "" {
		return NewError(ErrValidation, "confirmationId", "批准必须携带复核确认标识")
	}
	if confirmationID != expectedConfirmation {
		return NewError(ErrConflict, "confirmationId", "复核确认标识已失效，请重新核对")
	}
	if err := s.Approve(reviewer, "APPROVE", reason, now); err != nil {
		return err
	}
	s.Review.ReviewedVersion = reviewedVersion
	s.Review.ConfirmationID = confirmationID
	return nil
}

func (s *RiggingSession) closeReviewCalculationFindings(lineID string, now time.Time) {
	s.closeReviewFindings(lineID, "CALCULATION", "SYSTEM_RECALCULATION", now)
}
func (s *RiggingSession) closeReviewCheckFindings(lineID string, kind CheckKind, verifier string, now time.Time) {
	s.closeReviewFindings(lineID, string(kind), verifier, now)
}
func (s *RiggingSession) closeReviewFindings(lineID, category, verifier string, now time.Time) {
	for i := range s.Findings {
		f := &s.Findings[i]
		if f.LineID == lineID && f.SourceType == "REVIEW" && f.RuleCode == "REVIEW_"+category && f.Status == FindingOpen && f.AssigneeID != "" && f.RemediationNote != "" {
			f.Status = FindingClosed
			f.VerifiedBy = verifier
			closed := now.UTC()
			f.ClosedAt = &closed
		}
	}
}

func itoaDomain(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return "多"
}
