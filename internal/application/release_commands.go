package application

import (
	"context"
	"rigging-readiness-desk/internal/domain"
)

func (s *Service) Freeze(ctx context.Context, id string, cmd VersionCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd, "freeze", "MANIFEST_FROZEN", "冻结场景吊挂清单", func(session *domain.RiggingSession) error { _, err := session.BuildManifest(s.now()); return err })
}
func (s *Service) Issue(ctx context.Context, id string, cmd VersionCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd, "issue", "CERTIFICATE_ISSUED", "签发吊挂启用凭据", func(session *domain.RiggingSession) error {
		_, err := session.IssueCertificate(s.newID(), s.now())
		return err
	})
}

type VerificationResult struct {
	Valid            bool                       `json:"valid"`
	Certificate      *domain.ReleaseCertificate `json:"certificate,omitempty"`
	RecomputedDigest string                     `json:"recomputedDigest,omitempty"`
	Reason           string                     `json:"reason"`
}

func (s *Service) Verify(ctx context.Context, certificateID string) (*VerificationResult, error) {
	session, err := s.repo.GetByCertificate(ctx, certificateID)
	if err != nil {
		return nil, err
	}
	valid := session.VerifyCertificate()
	digest := ""
	if session.Frozen != nil && session.Review != nil {
		digest = domain.DigestManifest(session.Frozen, session.RuleSetVersion, session.Review.ReviewerID)
	}
	reason := "摘要、冻结版本与凭据一致"
	if !valid {
		reason = "凭据或冻结材料摘要不一致"
	}
	return &VerificationResult{Valid: valid, Certificate: session.Certificate, RecomputedDigest: digest, Reason: reason}, nil
}
