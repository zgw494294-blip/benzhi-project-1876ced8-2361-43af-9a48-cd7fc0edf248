package verify_context_isolation_test

import (
	"context"
	"errors"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type blockingRepository struct {
	entered             chan struct{}
	sawCancel           chan struct{}
	allowCanceledReturn chan struct{}
	release             chan struct{}
	session             *domain.RiggingSession
	enterOnce           sync.Once
	cancelOnce          sync.Once
	calls               atomic.Int32
}

func (r *blockingRepository) GetByCertificate(ctx context.Context, _ string) (*domain.RiggingSession, error) {
	if r.calls.Add(1) > 1 {
		_ = ctx.Done()
		return r.session, nil
	}
	r.enterOnce.Do(func() { close(r.entered) })
	select {
	case <-ctx.Done():
		r.cancelOnce.Do(func() { close(r.sawCancel) })
		<-r.allowCanceledReturn
		return nil, ctx.Err()
	case <-r.release:
		return r.session, nil
	}
}

func (r *blockingRepository) Create(context.Context, *domain.RiggingSession, application.AuditEvent, *application.IdempotencyRecord) error {
	panic("unexpected Create")
}
func (r *blockingRepository) Get(context.Context, string) (*domain.RiggingSession, error) {
	panic("unexpected Get")
}
func (r *blockingRepository) List(context.Context, int, int) ([]domain.RiggingSession, int, error) {
	panic("unexpected List")
}
func (r *blockingRepository) QuerySessions(context.Context, application.SessionListFilter) ([]domain.RiggingSession, int, application.QueueSummary, error) {
	panic("unexpected QuerySessions")
}
func (r *blockingRepository) Save(context.Context, *domain.RiggingSession, int64, application.AuditEvent, *application.IdempotencyRecord) error {
	panic("unexpected Save")
}
func (r *blockingRepository) GetIdempotency(context.Context, string, string) (*application.IdempotencyRecord, error) {
	panic("unexpected GetIdempotency")
}
func (r *blockingRepository) ListAudit(context.Context, string, int, int) ([]application.AuditEvent, int, error) {
	panic("unexpected ListAudit")
}
func (r *blockingRepository) Close() error { return nil }

type observingContext struct {
	context.Context
	doneObserved chan struct{}
	once         sync.Once
}

func (c *observingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.doneObserved) })
	return c.Context.Done()
}

type verificationOutcome struct {
	result *application.VerificationResult
	err    error
}

func TestCanceledVerificationLeaderDoesNotPoisonFollower(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	review := &domain.SafetyReview{ReviewerID: "reviewer-b", Decision: "APPROVE", ReviewedAt: now}
	manifest := &domain.FrozenManifest{Version: 7, FrozenAt: now, Lines: []domain.ManifestLine{}}
	manifest.Digest = domain.DigestManifest(manifest, "RIG-2026.1", review.ReviewerID)
	certificate := &domain.ReleaseCertificate{ID: "cert-shared", SessionID: "session-a", FrozenVersion: manifest.Version, ManifestDigest: manifest.Digest, RuleSetVersion: "RIG-2026.1", ApprovedBy: review.ReviewerID, IssuedAt: now, VerificationStatus: "VALID"}
	repository := &blockingRepository{
		entered:             make(chan struct{}),
		sawCancel:           make(chan struct{}),
		allowCanceledReturn: make(chan struct{}),
		release:             make(chan struct{}),
		session:             &domain.RiggingSession{ID: "session-a", RuleSetVersion: "RIG-2026.1", Status: domain.StatusReleased, Review: review, Frozen: manifest, Certificate: certificate},
	}
	service := application.NewService(repository)

	leaderContext, cancelLeader := context.WithCancel(context.Background())
	leaderDone := make(chan verificationOutcome, 1)
	go func() {
		result, err := service.Verify(leaderContext, certificate.ID)
		leaderDone <- verificationOutcome{result: result, err: err}
	}()
	<-repository.entered

	followerContext := &observingContext{Context: context.Background(), doneObserved: make(chan struct{})}
	followerDone := make(chan verificationOutcome, 1)
	go func() {
		result, err := service.Verify(followerContext, certificate.ID)
		followerDone <- verificationOutcome{result: result, err: err}
	}()
	<-followerContext.doneObserved

	cancelLeader()
	select {
	case <-repository.sawCancel:
		close(repository.allowCanceledReturn)
	case outcome := <-leaderDone:
		if !errors.Is(outcome.err, context.Canceled) {
			t.Fatalf("首请求取消后返回错误 = %v", outcome.err)
		}
		close(repository.release)
	}

	follower := <-followerDone
	if follower.err != nil {
		t.Fatalf("健康跟随请求被首请求的 context 污染: %v", follower.err)
	}
	if follower.result == nil || !follower.result.Valid || follower.result.Certificate == nil || follower.result.Certificate.ID != certificate.ID {
		t.Fatalf("健康跟随请求未得到有效凭据: %#v", follower.result)
	}
}
