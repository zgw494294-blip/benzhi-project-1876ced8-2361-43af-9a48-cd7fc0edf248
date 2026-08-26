package domain

type SessionStatus string

const (
	StatusDraft     SessionStatus = "DRAFT"
	StatusBaselined SessionStatus = "BASELINED"
	StatusModeled   SessionStatus = "MODELED"
	StatusInspected SessionStatus = "INSPECTED"
	StatusApproved  SessionStatus = "APPROVED"
	StatusFrozen    SessionStatus = "FROZEN"
	StatusReleased  SessionStatus = "RELEASED"
)

func (s SessionStatus) Mutable() bool { return s != StatusFrozen && s != StatusReleased }
func (s *RiggingSession) requireStatus(allowed ...SessionStatus) error {
	for _, a := range allowed {
		if s.Status == a {
			return nil
		}
	}
	return NewError(ErrState, "status", "当前状态不允许此操作")
}
func (s *RiggingSession) requireMutable() error {
	if !s.Status.Mutable() {
		return NewError(ErrState, "status", "冻结后不得修改业务数据")
	}
	return nil
}
