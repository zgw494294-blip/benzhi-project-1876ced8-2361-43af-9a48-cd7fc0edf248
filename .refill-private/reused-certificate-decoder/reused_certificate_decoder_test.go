package reused_certificate_decoder_test

import (
	"context"
	"path/filepath"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"rigging-readiness-desk/internal/storage"
	"testing"
	"time"
)

func TestRepeatedCertificateVerificationDoesNotReuseExhaustedDecoder(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "rigging.db"))
	if err != nil {
		t.Fatalf("打开测试存储: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 8, 26, 9, 30, 0, 0, time.UTC)
	manifest := &domain.FrozenManifest{Version: 7, FrozenAt: now, Lines: []domain.ManifestLine{}}
	manifest.Digest = domain.DigestManifest(manifest, "rules-v1", "reviewer-2")
	session := &domain.RiggingSession{
		ID:             "session-repeat-verify",
		Title:          "重复凭据校验",
		Venue:          "一号剧场",
		PerformanceAt:  now.Add(24 * time.Hour),
		OperatorID:     "operator-1",
		RuleSetVersion: "rules-v1",
		Status:         domain.StatusReleased,
		Version:        8,
		CreatedAt:      now,
		UpdatedAt:      now,
		Review:         &domain.SafetyReview{ReviewerID: "reviewer-2", Decision: "APPROVE", ReviewedAt: now},
		Frozen:         manifest,
		Certificate: &domain.ReleaseCertificate{
			ID:                 "certificate-repeat-verify",
			SessionID:          "session-repeat-verify",
			FrozenVersion:      manifest.Version,
			ManifestDigest:     manifest.Digest,
			RuleSetVersion:     "rules-v1",
			ApprovedBy:         "reviewer-2",
			IssuedAt:           now,
			VerificationStatus: "VALID",
		},
	}
	event := application.AuditEvent{ID: "audit-created", SessionID: session.ID, Type: "SESSION_CREATED", ActorID: session.OperatorID, Detail: "测试建档", CreatedAt: now}
	if err := store.Create(context.Background(), session, event, nil); err != nil {
		t.Fatalf("准备已放行作业: %v", err)
	}

	service := application.NewService(store)
	for attempt := 1; attempt <= 2; attempt++ {
		result, err := service.Verify(context.Background(), session.Certificate.ID)
		if err != nil {
			t.Fatalf("第 %d 次校验返回错误: %v", attempt, err)
		}
		if !result.Valid {
			t.Fatalf("第 %d 次校验应保持有效，结果为 %#v", attempt, result)
		}
	}
}
