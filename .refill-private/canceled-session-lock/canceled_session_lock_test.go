package canceled_session_lock_test

import (
	"context"
	"encoding/json"
	"errors"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"sync"
	"testing"
	"time"
)

type blockingRepository struct {
	mu               sync.Mutex
	session          *domain.RiggingSession
	firstSaveEntered chan struct{}
	releaseFirstSave chan struct{}
	saveCalls        int
}

type observedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *observedContext) Err() error {
	c.once.Do(func() { close(c.checked) })
	return c.Context.Err()
}

func (r *blockingRepository) Create(context.Context, *domain.RiggingSession, application.AuditEvent, *application.IdempotencyRecord) error {
	return errors.New("unexpected Create call")
}

func (r *blockingRepository) Get(ctx context.Context, id string) (*domain.RiggingSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session == nil || r.session.ID != id {
		return nil, application.RepositoryNotFound()
	}
	return cloneSession(r.session), nil
}

func (r *blockingRepository) GetByCertificate(context.Context, string) (*domain.RiggingSession, error) {
	return nil, application.RepositoryNotFound()
}

func (r *blockingRepository) List(context.Context, int, int) ([]domain.RiggingSession, int, error) {
	return nil, 0, nil
}

func (r *blockingRepository) QuerySessions(context.Context, application.SessionListFilter) ([]domain.RiggingSession, int, application.QueueSummary, error) {
	return nil, 0, application.QueueSummary{}, nil
}

func (r *blockingRepository) Save(ctx context.Context, session *domain.RiggingSession, _ int64, _ application.AuditEvent, _ *application.IdempotencyRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	r.saveCalls++
	call := r.saveCalls
	r.mu.Unlock()
	if call == 1 {
		close(r.firstSaveEntered)
		<-r.releaseFirstSave
	}
	r.mu.Lock()
	r.session = cloneSession(session)
	r.mu.Unlock()
	return nil
}

func (r *blockingRepository) GetIdempotency(context.Context, string, string) (*application.IdempotencyRecord, error) {
	return nil, application.RepositoryNotFound()
}

func (r *blockingRepository) ListAudit(context.Context, string, int, int) ([]application.AuditEvent, int, error) {
	return nil, 0, nil
}

func (r *blockingRepository) Close() error { return nil }

func cloneSession(session *domain.RiggingSession) *domain.RiggingSession {
	data, err := json.Marshal(session)
	if err != nil {
		panic(err)
	}
	var cloned domain.RiggingSession
	if err = json.Unmarshal(data, &cloned); err != nil {
		panic(err)
	}
	return &cloned
}

func TestCanceledMutationDoesNotWaitForSessionLock(t *testing.T) {
	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	session, err := domain.NewSession("session-lock", "锁等待取消复现", "主舞台", now.Add(24*time.Hour), "operator-a", "rules-v1", now)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	repo := &blockingRepository{
		session:          session,
		firstSaveEntered: make(chan struct{}),
		releaseFirstSave: make(chan struct{}),
	}
	service := application.NewServiceWith(repo, func() time.Time { return now }, func() string { return "fixture-id" })

	firstResult := make(chan error, 1)
	go func() {
		_, firstErr := service.ConfirmBaseline(context.Background(), session.ID, application.BaselineCommand{
			VersionCommand: application.VersionCommand{ExpectedVersion: 1, ActorID: "operator-a"},
			BaselineRef:    "baseline-a",
		})
		firstResult <- firstErr
	}()
	<-repo.firstSaveEntered

	cancelBase, cancel := context.WithCancel(context.Background())
	canceled := &observedContext{Context: cancelBase, checked: make(chan struct{})}
	secondResult := make(chan error, 1)
	go func() {
		_, secondErr := service.ConfirmBaseline(canceled, session.ID, application.BaselineCommand{
			VersionCommand: application.VersionCommand{ExpectedVersion: 1, ActorID: "operator-a"},
			BaselineRef:    "baseline-b",
		})
		secondResult <- secondErr
	}()
	<-canceled.checked
	cancel()
	secondErr := <-secondResult
	if !errors.Is(secondErr, context.Canceled) {
		t.Fatalf("canceled mutation must fail before repository access, got %v", secondErr)
	}

	close(repo.releaseFirstSave)
	if firstErr := <-firstResult; firstErr != nil {
		t.Fatalf("first mutation failed: %v", firstErr)
	}
}
