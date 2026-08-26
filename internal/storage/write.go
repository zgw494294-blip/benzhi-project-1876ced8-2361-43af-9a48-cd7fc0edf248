package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
)

func (s *Store) Create(ctx context.Context, session *domain.RiggingSession, event application.AuditEvent, idem *application.IdempotencyRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sessions(id,title,venue,performance_at,operator_id,rule_set_version,status,version,created_at,updated_at,payload) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, session.ID, session.Title, session.Venue, session.PerformanceAt.Format(timeFormat), session.OperatorID, session.RuleSetVersion, string(session.Status), session.Version, session.CreatedAt.Format(timeFormat), session.UpdatedAt.Format(timeFormat), payload)
	if err != nil {
		return fmt.Errorf("创建作业: %w", err)
	}
	if err = replaceChildren(ctx, tx, session); err != nil {
		return err
	}
	if err = insertAudit(ctx, tx, event); err != nil {
		return err
	}
	if err = insertIdempotency(ctx, tx, idem); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.rememberSession(session)
	return nil
}
func (s *Store) Save(ctx context.Context, session *domain.RiggingSession, expected int64, event application.AuditEvent, idem *application.IdempotencyRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE sessions SET title=?,venue=?,performance_at=?,operator_id=?,rule_set_version=?,status=?,version=?,updated_at=?,payload=? WHERE id=? AND version=?`, session.Title, session.Venue, session.PerformanceAt.Format(timeFormat), session.OperatorID, session.RuleSetVersion, string(session.Status), session.Version, session.UpdatedAt.Format(timeFormat), payload, session.ID, expected)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return domain.NewError(domain.ErrConflict, "expectedVersion", "数据库中的作业版本已变化")
	}
	if err = replaceChildren(ctx, tx, session); err != nil {
		return err
	}
	if err = insertAudit(ctx, tx, event); err != nil {
		return err
	}
	if err = insertIdempotency(ctx, tx, idem); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.rememberSession(session)
	return nil
}
func insertAudit(ctx context.Context, tx *sql.Tx, event application.AuditEvent) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,session_id,event_type,actor_id,detail,created_at) VALUES(?,?,?,?,?,?)`, event.ID, event.SessionID, event.Type, event.ActorID, event.Detail, event.CreatedAt.Format(timeFormat))
	return err
}
func insertIdempotency(ctx context.Context, tx *sql.Tx, record *application.IdempotencyRecord) error {
	if record == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO idempotency_records(key,operation,session_id,response,created_at) VALUES(?,?,?,?,?)`, record.Key, record.Operation, record.SessionID, record.Response, record.CreatedAt.Format(timeFormat))
	return err
}
