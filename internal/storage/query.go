package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"strings"
	"time"
)

const timeFormat = time.RFC3339Nano

func (s *Store) Get(ctx context.Context, id string) (*domain.RiggingSession, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM sessions WHERE id = ?", id).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, domain.NewError(domain.ErrNotFound, "sessionId", "作业不存在")
	}
	if err != nil {
		return nil, err
	}
	var session domain.RiggingSession
	if err = json.Unmarshal(payload, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *Store) GetByCertificate(ctx context.Context, certificateID string) (*domain.RiggingSession, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT s.payload FROM sessions s JOIN release_certificates c ON c.session_id=s.id WHERE c.id=?`, certificateID).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, domain.NewError(domain.ErrNotFound, "certificateId", "凭据不存在")
	}
	if err != nil {
		return nil, err
	}
	var session domain.RiggingSession
	if err = json.Unmarshal(payload, &session); err != nil {
		return nil, err
	}
	return &session, nil
}
func (s *Store) List(ctx context.Context, offset, limit int) ([]domain.RiggingSession, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM sessions ORDER BY updated_at DESC,id LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []domain.RiggingSession{}
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			return nil, 0, err
		}
		var session domain.RiggingSession
		if err = json.Unmarshal(payload, &session); err != nil {
			return nil, 0, err
		}
		items = append(items, session)
	}
	return items, total, rows.Err()
}

func (s *Store) QuerySessions(ctx context.Context, filter application.SessionListFilter) ([]domain.RiggingSession, int, application.QueueSummary, error) {
	where, args := sessionFilterSQL(filter)
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions s"+where, args...).Scan(&total); err != nil {
		return nil, 0, application.QueueSummary{}, err
	}
	summary := application.QueueSummary{StatusCounts: map[domain.SessionStatus]int{domain.StatusDraft: 0, domain.StatusBaselined: 0, domain.StatusModeled: 0, domain.StatusInspected: 0, domain.StatusApproved: 0, domain.StatusFrozen: 0, domain.StatusReleased: 0}}
	statusRows, err := s.db.QueryContext(ctx, "SELECT s.status,COUNT(*) FROM sessions s"+where+" GROUP BY s.status", args...)
	if err != nil {
		return nil, 0, summary, err
	}
	for statusRows.Next() {
		var status domain.SessionStatus
		var count int
		if err = statusRows.Scan(&status, &count); err != nil {
			statusRows.Close()
			return nil, 0, summary, err
		}
		summary.StatusCounts[status] = count
	}
	if err = statusRows.Close(); err != nil {
		return nil, 0, summary, err
	}
	openQuery := "SELECT COUNT(*) FROM safety_findings f JOIN sessions s ON s.id=f.session_id" + where + filterSuffix(where, "f.status='OPEN'")
	if err = s.db.QueryRowContext(ctx, openQuery, args...).Scan(&summary.OpenFindingCount); err != nil {
		return nil, 0, summary, err
	}
	if filter.DueBefore != nil {
		upcomingQuery := "SELECT COUNT(*) FROM sessions s" + where + filterSuffix(where, "s.performance_at<=? AND s.status<>'RELEASED'")
		upcomingArgs := append(append([]any{}, args...), filter.DueBefore.UTC().Format(timeFormat))
		if err = s.db.QueryRowContext(ctx, upcomingQuery, upcomingArgs...).Scan(&summary.UpcomingUnreleasedCount); err != nil {
			return nil, 0, summary, err
		}
	}
	pageArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	rows, err := s.db.QueryContext(ctx, "SELECT s.payload FROM sessions s"+where+" ORDER BY s.performance_at ASC,s.updated_at DESC,s.id ASC LIMIT ? OFFSET ?", pageArgs...)
	if err != nil {
		return nil, 0, summary, err
	}
	defer rows.Close()
	items := []domain.RiggingSession{}
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			return nil, 0, summary, err
		}
		var session domain.RiggingSession
		if err = json.Unmarshal(payload, &session); err != nil {
			return nil, 0, summary, err
		}
		items = append(items, session)
	}
	return items, total, summary, rows.Err()
}

func sessionFilterSQL(filter application.SessionListFilter) (string, []any) {
	clauses := []string{}
	args := []any{}
	if filter.Status != "" {
		clauses = append(clauses, "s.status=?")
		args = append(args, string(filter.Status))
	}
	if filter.Venue != "" {
		clauses = append(clauses, "s.venue=? COLLATE NOCASE")
		args = append(args, filter.Venue)
	}
	if filter.OperatorID != "" {
		clauses = append(clauses, "s.operator_id=? COLLATE NOCASE")
		args = append(args, filter.OperatorID)
	}
	if filter.PerformanceFrom != nil {
		clauses = append(clauses, "s.performance_at>=?")
		args = append(args, filter.PerformanceFrom.UTC().Format(timeFormat))
	}
	if filter.PerformanceTo != nil {
		clauses = append(clauses, "s.performance_at<=?")
		args = append(args, filter.PerformanceTo.UTC().Format(timeFormat))
	}
	if filter.Keyword != "" {
		clauses = append(clauses, "instr(lower(s.title),lower(?))>0")
		args = append(args, filter.Keyword)
	}
	if filter.PendingOnly {
		clauses = append(clauses, "s.performance_at<=?", "s.status<>'RELEASED'")
		args = append(args, filter.DueBefore.UTC().Format(timeFormat))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func filterSuffix(where, clause string) string {
	if where == "" {
		return " WHERE " + clause
	}
	return " AND " + clause
}
func (s *Store) GetIdempotency(ctx context.Context, key, operation string) (*application.IdempotencyRecord, error) {
	if key == "" {
		return nil, application.RepositoryNotFound()
	}
	var record application.IdempotencyRecord
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT key,operation,session_id,response,created_at FROM idempotency_records WHERE key=? AND operation=?`, key, operation).Scan(&record.Key, &record.Operation, &record.SessionID, &record.Response, &created)
	if err == sql.ErrNoRows {
		return nil, application.RepositoryNotFound()
	}
	if err != nil {
		return nil, err
	}
	record.CreatedAt, err = time.Parse(timeFormat, created)
	return &record, err
}
func (s *Store) ListAudit(ctx context.Context, id string, offset, limit int) ([]application.AuditEvent, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE session_id=?", id).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,event_type,actor_id,detail,created_at FROM audit_events WHERE session_id=? ORDER BY sequence DESC LIMIT ? OFFSET ?`, id, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []application.AuditEvent{}
	for rows.Next() {
		var event application.AuditEvent
		var created string
		if err = rows.Scan(&event.ID, &event.SessionID, &event.Type, &event.ActorID, &event.Detail, &created); err != nil {
			return nil, 0, err
		}
		event.CreatedAt, err = time.Parse(timeFormat, created)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, event)
	}
	return items, total, rows.Err()
}
