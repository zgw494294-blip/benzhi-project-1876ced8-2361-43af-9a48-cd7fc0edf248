package domain

import (
	"strings"
	"time"
)

func (s *RiggingSession) ConfirmBaseline(reference string) error {
	if err := s.requireStatus(StatusDraft); err != nil {
		return err
	}
	if strings.TrimSpace(reference) == "" {
		return NewError(ErrValidation, "baselineRef", "设备基线引用不能为空")
	}
	s.BaselineRef = strings.TrimSpace(reference)
	s.Status = StatusBaselined
	return nil
}
func (s *RiggingSession) AddLine(line RiggingLine) error {
	if err := s.requireStatus(StatusBaselined); err != nil {
		return err
	}
	if strings.TrimSpace(line.ID) == "" || strings.TrimSpace(line.Code) == "" {
		return NewError(ErrValidation, "line", "ID 和编号不能为空")
	}
	if line.RatedLoadGram <= 0 || line.SpanMillimeter <= 0 || line.MaxMomentNewtonMillimeter <= 0 {
		return NewError(ErrValidation, "capacity", "额定载荷、跨度和最大力矩必须大于零")
	}
	if err := s.validateUniqueLine(line); err != nil {
		return err
	}
	line.SessionID = s.ID
	line.Code = strings.TrimSpace(line.Code)
	s.Lines = append(s.Lines, line)
	return nil
}
func (s *RiggingSession) AddPoint(point RiggingPoint) error {
	if err := s.requireStatus(StatusBaselined); err != nil {
		return err
	}
	line, err := s.FindLine(point.LineID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(point.ID) == "" || strings.TrimSpace(point.Code) == "" {
		return NewError(ErrValidation, "point", "吊点 ID 和编号不能为空")
	}
	if point.HoistRatedLoadGram <= 0 {
		return NewError(ErrValidation, "hoistRatedLoadGram", "提升机额定载荷必须大于零")
	}
	if point.PositionMillimeter < 0 || point.PositionMillimeter > line.SpanMillimeter {
		return NewError(ErrValidation, "positionMillimeter", "吊点位置必须位于吊杆跨度内")
	}
	if err = s.validateUniquePoint(point); err != nil {
		return err
	}
	point.SessionID = s.ID
	point.Code = strings.TrimSpace(point.Code)
	s.Points = append(s.Points, point)
	return nil
}
func (s *RiggingSession) AddLoad(load SuspendedLoad) error {
	if err := s.requireStatus(StatusBaselined); err != nil {
		return err
	}
	line, err := s.FindLine(load.LineID)
	if err != nil {
		return err
	}
	point, err := s.FindPoint(load.PointID)
	if err != nil {
		return err
	}
	if point.LineID != line.ID {
		return NewError(ErrValidation, "pointId", "载荷吊点不属于指定吊杆")
	}
	if strings.TrimSpace(load.ID) == "" {
		return NewError(ErrValidation, "id", "构件 ID 不能为空")
	}
	if strings.TrimSpace(load.ComponentCode) == "" {
		return NewError(ErrValidation, "componentCode", "构件编号不能为空")
	}
	if strings.TrimSpace(load.Description) == "" {
		return NewError(ErrValidation, "description", "构件说明不能为空")
	}
	if load.WeightGram <= 0 {
		return NewError(ErrValidation, "weightGram", "重量必须大于零")
	}
	if load.Quantity <= 0 {
		return NewError(ErrValidation, "quantity", "数量必须大于零")
	}
	if load.PositionMillimeter < 0 || load.PositionMillimeter > line.SpanMillimeter {
		return NewError(ErrValidation, "positionMillimeter", "构件位置必须位于吊杆跨度内")
	}
	if strings.TrimSpace(load.SubmittedBy) == "" {
		return NewError(ErrValidation, "submittedBy", "提交人不能为空")
	}
	if err := s.validateUniqueLoad(load); err != nil {
		return err
	}
	load.SessionID = s.ID
	load.ComponentCode = strings.TrimSpace(load.ComponentCode)
	load.CreatedAt = load.CreatedAt.UTC()
	s.Loads = append(s.Loads, load)
	return nil
}
func (s *RiggingSession) FinalizeModel() error {
	if err := s.requireStatus(StatusBaselined); err != nil {
		return err
	}
	if len(s.Lines) == 0 {
		return NewError(ErrValidation, "lines", "至少登记一根吊杆")
	}
	for _, line := range s.Lines {
		pointFound := false
		for _, point := range s.Points {
			if point.LineID == line.ID {
				pointFound = true
				break
			}
		}
		if !pointFound {
			return NewError(ErrValidation, "points", "每根吊杆至少登记一个吊点及提升机能力")
		}
		found := false
		for _, load := range s.Loads {
			if load.LineID == line.ID {
				found = true
				break
			}
		}
		if !found {
			return NewError(ErrValidation, "loads", "每根吊杆至少登记一个悬挂构件")
		}
	}
	s.Status = StatusModeled
	return nil
}
func (s *RiggingSession) Approve(reviewer, decision, reason string, now time.Time) error {
	if err := s.requireStatus(StatusInspected); err != nil {
		return err
	}
	if strings.TrimSpace(reviewer) == "" {
		return NewError(ErrValidation, "reviewerId", "复核员不能为空")
	}
	if reviewer == s.OperatorID {
		return NewError(ErrForbidden, "reviewerId", "复核员必须独立于经办人")
	}
	if s.HasOpenFindings() {
		return NewError(ErrState, "findings", "仍有未关闭阻断项")
	}
	decision = strings.ToUpper(strings.TrimSpace(decision))
	if decision != "APPROVE" && decision != "RETURN" {
		return NewError(ErrValidation, "decision", "决定必须为 APPROVE 或 RETURN")
	}
	if decision == "RETURN" && strings.TrimSpace(reason) == "" {
		return NewError(ErrValidation, "reason", "退回必须填写理由")
	}
	s.Review = &SafetyReview{ReviewerID: reviewer, Decision: decision, Reason: strings.TrimSpace(reason), ReviewedAt: now.UTC()}
	if decision == "APPROVE" {
		s.Status = StatusApproved
	}
	return nil
}
