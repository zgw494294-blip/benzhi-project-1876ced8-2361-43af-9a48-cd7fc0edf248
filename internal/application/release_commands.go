package application

import (
	"context"
	"encoding/json"
	"rigging-readiness-desk/internal/domain"
)

func (s *Service) Freeze(ctx context.Context, id string, cmd VersionCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd, "freeze", "MANIFEST_FROZEN", "冻结场景吊挂清单", func(session *domain.RiggingSession) error { _, err := session.BuildManifest(s.now()); return err })
}
func (s *Service) Issue(ctx context.Context, id string, cmd VersionCommand) (*domain.RiggingSession, error) {
	const operation = "issue"
	unlock := s.locks.Lock(id)
	defer unlock()
	if cached, ok, err := s.idempotent(ctx, cmd.IdempotencyKey, operation); err != nil || ok {
		return cached, err
	}
	session, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.ExpectedVersion <= 0 {
		return nil, domain.NewError(domain.ErrValidation, "expectedVersion", "必须大于零")
	}
	if session.Version != cmd.ExpectedVersion {
		return nil, domain.NewError(domain.ErrConflict, "expectedVersion", "版本已变化，请刷新后重试")
	}
	// 幂等再签发：作业已凭据化且凭据有效时，返回原凭据，不递增版本、不重复写入规范化数据、不新增审计。
	if session.Status == domain.StatusReleased && session.Certificate != nil {
		return session, nil
	}
	if _, err := session.IssueCertificate(s.newID(), s.now()); err != nil {
		return nil, err
	}
	expected := session.Version
	session.Version++
	session.UpdatedAt = s.now().UTC()
	data, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	event := AuditEvent{ID: s.newID(), SessionID: session.ID, Type: "CERTIFICATE_ISSUED", ActorID: cmd.ActorID, Detail: "签发吊挂启用凭据", CreatedAt: s.now().UTC()}
	var idem *IdempotencyRecord
	if cmd.IdempotencyKey != "" {
		idem = &IdempotencyRecord{Key: cmd.IdempotencyKey, Operation: operation, SessionID: session.ID, Response: data, CreatedAt: s.now().UTC()}
	}
	if err := s.repo.Save(ctx, session, expected, event, idem); err != nil {
		return nil, err
	}
	return session, nil
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
