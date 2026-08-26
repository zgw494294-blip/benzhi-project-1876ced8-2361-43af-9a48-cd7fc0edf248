package domain

import (
	"strings"
	"time"
)

func (s *RiggingSession) RecordCheck(check LineCheck, now time.Time, idFactory func() string) error {
	if err := s.requireStatus(StatusModeled, StatusInspected); err != nil {
		return err
	}
	if _, err := s.FindLine(check.LineID); err != nil {
		return err
	}
	valid := false
	for _, kind := range RequiredChecks {
		if check.Kind == kind {
			valid = true
			break
		}
	}
	if !valid {
		return NewError(ErrValidation, "kind", "未知检查项目")
	}
	if strings.TrimSpace(check.InspectorID) == "" || strings.TrimSpace(check.Evidence) == "" {
		return NewError(ErrValidation, "check", "检查人和文字证据不能为空")
	}
	check.CheckedAt = now.UTC()
	if check.ID == "" {
		check.ID = idFactory()
	}
	replaced := false
	for i := range s.Checks {
		if s.Checks[i].LineID == check.LineID && s.Checks[i].Kind == check.Kind {
			s.Checks[i] = check
			replaced = true
			break
		}
	}
	if !replaced {
		s.Checks = append(s.Checks, check)
	}
	rule := "CHECK_" + string(check.Kind)
	if !check.Passed {
		s.upsertInspectionFinding(check.LineID, rule, "现场检查未通过："+string(check.Kind), idFactory)
	} else {
		s.closeInspectionFinding(check.LineID, rule, check.InspectorID, now)
		s.closeReviewCheckFindings(check.LineID, check.Kind, check.InspectorID, now)
	}
	return nil
}
func (s *RiggingSession) upsertInspectionFinding(lineID, rule, description string, idFactory func() string) {
	for i := range s.Findings {
		if s.Findings[i].LineID == lineID && s.Findings[i].RuleCode == rule && s.Findings[i].Status == FindingOpen {
			s.Findings[i].Description = description
			return
		}
	}
	s.Findings = append(s.Findings, SafetyFinding{ID: idFactory(), SessionID: s.ID, LineID: lineID, SourceType: "INSPECTION", Severity: "BLOCKING", RuleCode: rule, Description: description, Status: FindingOpen})
}
func (s *RiggingSession) closeInspectionFinding(lineID, rule, verified string, now time.Time) {
	for i := range s.Findings {
		f := &s.Findings[i]
		if f.LineID == lineID && f.RuleCode == rule && f.Status == FindingOpen {
			f.Status = FindingClosed
			f.VerifiedBy = verified
			closed := now.UTC()
			f.ClosedAt = &closed
		}
	}
}
func (s *RiggingSession) CompleteInspection() error {
	if err := s.requireStatus(StatusModeled); err != nil {
		return err
	}
	if s.Calculation == nil {
		return NewError(ErrState, "calculation", "必须先完成载荷计算")
	}
	for _, line := range s.Lines {
		for _, kind := range RequiredChecks {
			found := false
			for _, check := range s.Checks {
				if check.LineID == line.ID && check.Kind == kind {
					found = true
					break
				}
			}
			if !found {
				return NewError(ErrValidation, "checks", "检查集不完整")
			}
		}
	}
	s.Status = StatusInspected
	return nil
}
