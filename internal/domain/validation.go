package domain

import (
	"strings"
	"time"
)

func NewSession(id, title, venue string, performanceAt time.Time, operatorID, ruleVersion string, now time.Time) (*RiggingSession, error) {
	if strings.TrimSpace(id) == "" {
		return nil, NewError(ErrValidation, "id", "不能为空")
	}
	if strings.TrimSpace(title) == "" {
		return nil, NewError(ErrValidation, "title", "不能为空")
	}
	if strings.TrimSpace(venue) == "" {
		return nil, NewError(ErrValidation, "venue", "不能为空")
	}
	if performanceAt.IsZero() {
		return nil, NewError(ErrValidation, "performanceAt", "必须提供演出时间")
	}
	if strings.TrimSpace(operatorID) == "" {
		return nil, NewError(ErrValidation, "operatorId", "不能为空")
	}
	if strings.TrimSpace(ruleVersion) == "" {
		return nil, NewError(ErrValidation, "ruleSetVersion", "不能为空")
	}
	return &RiggingSession{ID: id, Title: strings.TrimSpace(title), Venue: strings.TrimSpace(venue), PerformanceAt: performanceAt.UTC(), OperatorID: strings.TrimSpace(operatorID), RuleSetVersion: strings.TrimSpace(ruleVersion), Status: StatusDraft, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC(), Lines: []RiggingLine{}, Points: []RiggingPoint{}, Loads: []SuspendedLoad{}, Checks: []LineCheck{}, Findings: []SafetyFinding{}}, nil
}
func (s *RiggingSession) FindLine(id string) (*RiggingLine, error) {
	for i := range s.Lines {
		if s.Lines[i].ID == id {
			return &s.Lines[i], nil
		}
	}
	return nil, NewError(ErrNotFound, "lineId", "吊杆不存在")
}
func (s *RiggingSession) FindPoint(id string) (*RiggingPoint, error) {
	for i := range s.Points {
		if s.Points[i].ID == id {
			return &s.Points[i], nil
		}
	}
	return nil, NewError(ErrNotFound, "pointId", "吊点不存在")
}
func (s *RiggingSession) validateUniqueLine(line RiggingLine) error {
	for _, e := range s.Lines {
		if e.ID == line.ID {
			return NewError(ErrValidation, "id", "吊杆 ID 已存在")
		}
		if strings.EqualFold(e.Code, line.Code) {
			return NewError(ErrValidation, "code", "吊杆编号已存在")
		}
	}
	return nil
}
func (s *RiggingSession) validateUniqueLoad(load SuspendedLoad) error {
	for _, e := range s.Loads {
		if e.ID == load.ID {
			return NewError(ErrValidation, "id", "载荷 ID 已存在")
		}
		if strings.EqualFold(e.ComponentCode, load.ComponentCode) {
			return NewError(ErrValidation, "componentCode", "构件编号已存在")
		}
	}
	return nil
}
func (s *RiggingSession) validateUniquePoint(point RiggingPoint) error {
	for _, existing := range s.Points {
		if existing.ID == point.ID {
			return NewError(ErrValidation, "id", "吊点 ID 已存在")
		}
		if strings.EqualFold(existing.Code, point.Code) {
			return NewError(ErrValidation, "code", "吊点编号已存在")
		}
	}
	return nil
}
func (s *RiggingSession) HasOpenFindings() bool {
	for _, f := range s.Findings {
		if f.Status == FindingOpen {
			return true
		}
	}
	return false
}
