package idempotency_key_race_test

import (
	"context"
	"errors"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"rigging-readiness-desk/internal/storage"
	"sync"
	"testing"
	"time"
)

type barrierRepository struct {
	base    application.Repository
	mu      sync.Mutex
	waiters int
	ready   chan struct{}
	release chan struct{}
}

func (r *barrierRepository) Create(ctx context.Context, s *domain.RiggingSession, e application.AuditEvent, i *application.IdempotencyRecord) error {
	return r.base.Create(ctx, s, e, i)
}
func (r *barrierRepository) Get(ctx context.Context, id string) (*domain.RiggingSession, error) {
	return r.base.Get(ctx, id)
}
func (r *barrierRepository) GetByCertificate(ctx context.Context, id string) (*domain.RiggingSession, error) {
	return r.base.GetByCertificate(ctx, id)
}
func (r *barrierRepository) List(ctx context.Context, offset, limit int) ([]domain.RiggingSession, int, error) {
	return r.base.List(ctx, offset, limit)
}
func (r *barrierRepository) QuerySessions(ctx context.Context, f application.SessionListFilter) ([]domain.RiggingSession, int, application.QueueSummary, error) {
	return r.base.QuerySessions(ctx, f)
}
func (r *barrierRepository) Save(ctx context.Context, s *domain.RiggingSession, v int64, e application.AuditEvent, i *application.IdempotencyRecord) error {
	return r.base.Save(ctx, s, v, e, i)
}
func (r *barrierRepository) GetIdempotency(ctx context.Context, key, operation string) (*application.IdempotencyRecord, error) {
	record, err := r.base.GetIdempotency(ctx, key, operation)
	if key != "shared-key" || operation != "confirm-baseline" || !application.IsRepositoryNotFound(err) {
		return record, err
	}
	r.mu.Lock()
	r.waiters++
	if r.waiters == 2 {
		close(r.ready)
	}
	r.mu.Unlock()
	<-r.release
	return record, err
}
func (r *barrierRepository) ListAudit(ctx context.Context, id string, offset, limit int) ([]application.AuditEvent, int, error) {
	return r.base.ListAudit(ctx, id, offset, limit)
}
func (r *barrierRepository) Close() error { return r.base.Close() }

func TestConcurrentCrossSessionIdempotencyConflictIsDomainError(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	repo := &barrierRepository{base: store, ready: make(chan struct{}), release: make(chan struct{})}
	service := application.NewService(repo)
	t.Cleanup(func() { _ = service.Close() })
	ctx := context.Background()
	a, err := service.CreateSession(ctx, application.CreateSessionCommand{Title: "甲", Venue: "剧场", PerformanceAt: time.Now().Add(time.Hour), OperatorID: "op-a", RuleSetVersion: "R1"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := service.CreateSession(ctx, application.CreateSessionCommand{Title: "乙", Venue: "剧场", PerformanceAt: time.Now().Add(time.Hour), OperatorID: "op-b", RuleSetVersion: "R1"})
	if err != nil {
		t.Fatal(err)
	}

	type callResult struct {
		session *domain.RiggingSession
		err     error
	}
	resultsCh := make(chan callResult, 2)
	for _, session := range []*domain.RiggingSession{a, b} {
		session := session
		go func() {
			_, callErr := service.ConfirmBaseline(ctx, session.ID, application.BaselineCommand{VersionCommand: application.VersionCommand{ExpectedVersion: session.Version, IdempotencyKey: "shared-key"}, BaselineRef: "B1"})
			resultsCh <- callResult{session: session, err: callErr}
		}()
	}
	<-repo.ready
	close(repo.release)

	results := []callResult{<-resultsCh, <-resultsCh}
	successes := 0
	conflicts := 0
	var conflicted *domain.RiggingSession
	for _, result := range results {
		if result.err == nil {
			successes++
			continue
		}
		var de *domain.DomainError
		if errors.As(result.err, &de) && de.Code == domain.ErrConflict {
			conflicts++
			conflicted = result.session
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("并发复用跨作业幂等键应得到一次成功和一次可识别冲突，实际 success=%d conflict=%d errors=%v", successes, conflicts, results)
	}
	_, retryErr := service.ConfirmBaseline(ctx, conflicted.ID, application.BaselineCommand{VersionCommand: application.VersionCommand{ExpectedVersion: conflicted.Version, IdempotencyKey: "shared-key"}, BaselineRef: "B1"})
	var retryDomainErr *domain.DomainError
	if !errors.As(retryErr, &retryDomainErr) || retryDomainErr.Code != domain.ErrConflict {
		t.Fatalf("冲突作业重试时幂等缓存返回了另一作业的响应：err=%v", retryErr)
	}
}
