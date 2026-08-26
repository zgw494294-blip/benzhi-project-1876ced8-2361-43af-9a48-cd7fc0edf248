package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"rigging-readiness-desk/internal/domain"
	"sync"
	"time"
)

type Clock func() time.Time
type IDFactory func() string
type Service struct {
	repo        Repository
	now         Clock
	newID       IDFactory
	locks       *keyedLocks
	verifyMu    sync.Mutex
	verifyCalls map[string]*verificationCall
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, newID: randomID, locks: newKeyedLocks(), verifyCalls: map[string]*verificationCall{}}
}
func NewServiceWith(repo Repository, now Clock, ids IDFactory) *Service {
	return &Service{repo: repo, now: now, newID: ids, locks: newKeyedLocks(), verifyCalls: map[string]*verificationCall{}}
}
func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
func (s *Service) Close() error { return s.repo.Close() }
func (s *Service) idempotent(ctx context.Context, key, operation string) (*domain.RiggingSession, bool, error) {
	if key == "" {
		return nil, false, nil
	}
	record, err := s.repo.GetIdempotency(ctx, key, operation)
	if err != nil {
		if errors.Is(err, domainNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var session domain.RiggingSession
	if err := json.Unmarshal(record.Response, &session); err != nil {
		return nil, false, err
	}
	return &session, true, nil
}

var domainNotFound = errors.New("record not found")

func IsRepositoryNotFound(err error) bool { return errors.Is(err, domainNotFound) }
func RepositoryNotFound() error           { return domainNotFound }
func (s *Service) mutation(ctx context.Context, sessionID string, cmd VersionCommand, operation, eventType, detail string, change func(*domain.RiggingSession) error) (*domain.RiggingSession, error) {
	unlock := s.locks.Lock(sessionID)
	defer unlock()
	if cached, ok, err := s.idempotent(ctx, cmd.IdempotencyKey, operation); err != nil || ok {
		return cached, err
	}
	session, err := s.repo.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if cmd.ExpectedVersion <= 0 {
		return nil, domain.NewError(domain.ErrValidation, "expectedVersion", "必须大于零")
	}
	if session.Version != cmd.ExpectedVersion {
		return nil, domain.NewError(domain.ErrConflict, "expectedVersion", "版本已变化，请刷新后重试")
	}
	if err := change(session); err != nil {
		return nil, err
	}
	expected := session.Version
	session.Version++
	session.UpdatedAt = s.now().UTC()
	data, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	event := AuditEvent{ID: s.newID(), SessionID: session.ID, Type: eventType, ActorID: cmd.ActorID, Detail: detail, CreatedAt: s.now().UTC()}
	var idem *IdempotencyRecord
	if cmd.IdempotencyKey != "" {
		idem = &IdempotencyRecord{Key: cmd.IdempotencyKey, Operation: operation, SessionID: session.ID, Response: data, CreatedAt: s.now().UTC()}
	}
	if err := s.repo.Save(ctx, session, expected, event, idem); err != nil {
		return nil, err
	}
	return session, nil
}
