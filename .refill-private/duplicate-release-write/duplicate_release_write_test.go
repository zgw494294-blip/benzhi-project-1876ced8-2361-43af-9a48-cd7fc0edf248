package duplicate_release_write_test

import (
	"context"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"rigging-readiness-desk/internal/storage"
	"testing"
	"time"
)

func releasedSession(now time.Time) *domain.RiggingSession {
	s, _ := domain.NewSession("session", "演出", "剧场", now.Add(time.Hour), "operator", "R1", now)
	s.Status = domain.StatusReleased
	s.Version = 9
	s.Review = &domain.SafetyReview{ReviewerID: "reviewer", Decision: "APPROVE", ReviewedVersion: 7, ReviewedAt: now}
	s.Frozen = &domain.FrozenManifest{Version: 8, FrozenAt: now, Lines: []domain.ManifestLine{}}
	s.Frozen.Digest = domain.DigestManifest(s.Frozen, s.RuleSetVersion, s.Review.ReviewerID)
	s.Certificate = &domain.ReleaseCertificate{ID: "certificate", SessionID: s.ID, FrozenVersion: s.Frozen.Version, ManifestDigest: s.Frozen.Digest, RuleSetVersion: s.RuleSetVersion, ApprovedBy: s.Review.ReviewerID, IssuedAt: now, VerificationStatus: "VALID"}
	return s
}

func TestRepeatedReleaseDoesNotCreateWriteOrAudit(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	s := releasedSession(now)
	ctx := context.Background()
	initial := application.AuditEvent{ID: "audit-initial", SessionID: s.ID, Type: "CERTIFICATE_ISSUED", ActorID: "reviewer", Detail: "首次签发", CreatedAt: now}
	if err = store.Create(ctx, s, initial, nil); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	t.Cleanup(func() { _ = service.Close() })

	repeated, err := service.Issue(ctx, s.ID, application.VersionCommand{ExpectedVersion: s.Version, ActorID: "reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	_, total, err := store.ListAudit(ctx, s.ID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Version != s.Version || total != 1 {
		t.Fatalf("重复签发污染了版本或审计：version=%d auditTotal=%d", repeated.Version, total)
	}
}
