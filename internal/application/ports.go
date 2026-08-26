package application

import (
	"context"
	"rigging-readiness-desk/internal/domain"
	"time"
)

type AuditEvent struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actorId"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}
type IdempotencyRecord struct {
	Key       string
	Operation string
	SessionID string
	Response  []byte
	CreatedAt time.Time
}
type Repository interface {
	Create(context.Context, *domain.RiggingSession, AuditEvent, *IdempotencyRecord) error
	Get(context.Context, string) (*domain.RiggingSession, error)
	GetByCertificate(context.Context, string) (*domain.RiggingSession, error)
	List(context.Context, int, int) ([]domain.RiggingSession, int, error)
	QuerySessions(context.Context, SessionListFilter) ([]domain.RiggingSession, int, QueueSummary, error)
	Save(context.Context, *domain.RiggingSession, int64, AuditEvent, *IdempotencyRecord) error
	GetIdempotency(context.Context, string, string) (*IdempotencyRecord, error)
	ListAudit(context.Context, string, int, int) ([]AuditEvent, int, error)
	Close() error
}
