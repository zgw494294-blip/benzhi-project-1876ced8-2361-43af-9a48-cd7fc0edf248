package application

import (
	"context"
	"encoding/json"
	"rigging-readiness-desk/internal/domain"
)

func (s *Service) CreateSession(ctx context.Context, cmd CreateSessionCommand) (*domain.RiggingSession, error) {
	unlock := s.locks.Lock("create:" + cmd.IdempotencyKey)
	defer unlock()
	if cached, ok, err := s.idempotent(ctx, cmd.IdempotencyKey, "create-session"); err != nil || ok {
		return cached, err
	}
	now := s.now().UTC()
	session, err := domain.NewSession(s.newID(), cmd.Title, cmd.Venue, cmd.PerformanceAt, cmd.OperatorID, cmd.RuleSetVersion, now)
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(session)
	event := AuditEvent{ID: s.newID(), SessionID: session.ID, Type: "SESSION_CREATED", ActorID: cmd.OperatorID, Detail: "创建演出吊挂作业", CreatedAt: now}
	var idem *IdempotencyRecord
	if cmd.IdempotencyKey != "" {
		idem = &IdempotencyRecord{Key: cmd.IdempotencyKey, Operation: "create-session", SessionID: session.ID, Response: data, CreatedAt: now}
	}
	if err := s.repo.Create(ctx, session, event, idem); err != nil {
		return nil, err
	}
	s.readiness.invalidate(session.ID)
	return session, nil
}
func (s *Service) ConfirmBaseline(ctx context.Context, id string, cmd BaselineCommand) (*domain.RiggingSession, error) {
	cmd.ActorID = defaultActor(cmd.ActorID, "operator")
	return s.mutation(ctx, id, cmd.VersionCommand, "confirm-baseline", "BASELINE_CONFIRMED", "确认设备基线", func(session *domain.RiggingSession) error { return session.ConfirmBaseline(cmd.BaselineRef) })
}
func defaultActor(actor, fallback string) string {
	if actor == "" {
		return fallback
	}
	return actor
}
