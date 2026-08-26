package domain

import "strings"

func (s *RiggingSession) AssignRemediation(findingID, assignee, note string) error {
	if err := s.requireStatus(StatusModeled, StatusInspected); err != nil {
		return err
	}
	if strings.TrimSpace(assignee) == "" || strings.TrimSpace(note) == "" {
		return NewError(ErrValidation, "remediation", "责任人和整改说明不能为空")
	}
	for i := range s.Findings {
		f := &s.Findings[i]
		if f.ID == findingID {
			if f.Status != FindingOpen {
				return NewError(ErrState, "finding", "阻断项已经关闭")
			}
			f.AssigneeID = strings.TrimSpace(assignee)
			f.RemediationNote = strings.TrimSpace(note)
			return nil
		}
	}
	return NewError(ErrNotFound, "findingId", "阻断项不存在")
}
func (s *RiggingSession) ReviseLoad(loadID string, weightGram, positionMillimeter int64) error {
	if err := s.requireStatus(StatusModeled, StatusInspected); err != nil {
		return err
	}
	if weightGram <= 0 {
		return NewError(ErrValidation, "weightGram", "重量必须大于零")
	}
	for i := range s.Loads {
		load := &s.Loads[i]
		if load.ID == loadID {
			line, err := s.FindLine(load.LineID)
			if err != nil {
				return err
			}
			if positionMillimeter < 0 || positionMillimeter > line.SpanMillimeter {
				return NewError(ErrValidation, "positionMillimeter", "位置必须位于吊杆跨度内")
			}
			load.WeightGram = weightGram
			load.PositionMillimeter = positionMillimeter
			s.Calculation = nil
			return nil
		}
	}
	return NewError(ErrNotFound, "loadId", "构件不存在")
}
