package concurrentpreviewworkspace_test

import (
	"context"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"sync/atomic"
	"testing"
	"time"
)

type previewRepository struct {
	sessions map[string]*domain.RiggingSession
}

func (r *previewRepository) Get(_ context.Context, id string) (*domain.RiggingSession, error) {
	return r.sessions[id], nil
}

func (*previewRepository) Create(context.Context, *domain.RiggingSession, application.AuditEvent, *application.IdempotencyRecord) error {
	panic("unexpected Create")
}
func (*previewRepository) GetByCertificate(context.Context, string) (*domain.RiggingSession, error) {
	panic("unexpected GetByCertificate")
}
func (*previewRepository) List(context.Context, int, int) ([]domain.RiggingSession, int, error) {
	panic("unexpected List")
}
func (*previewRepository) QuerySessions(context.Context, application.SessionListFilter) ([]domain.RiggingSession, int, application.QueueSummary, error) {
	panic("unexpected QuerySessions")
}
func (*previewRepository) Save(context.Context, *domain.RiggingSession, int64, application.AuditEvent, *application.IdempotencyRecord) error {
	panic("unexpected Save")
}
func (*previewRepository) GetIdempotency(context.Context, string, string) (*application.IdempotencyRecord, error) {
	panic("unexpected GetIdempotency")
}
func (*previewRepository) ListAudit(context.Context, string, int, int) ([]application.AuditEvent, int, error) {
	panic("unexpected ListAudit")
}
func (*previewRepository) Close() error { return nil }

func previewSession(id, suffix string) *domain.RiggingSession {
	lineID := "line-" + suffix
	loadID := "load-" + suffix
	return &domain.RiggingSession{
		ID:             id,
		RuleSetVersion: "RIG-2026.1",
		Status:         domain.StatusModeled,
		Version:        7,
		Lines: []domain.RiggingLine{{
			ID:                               lineID,
			Code:                             "LX-" + suffix,
			RatedLoadGram:                    1000,
			SpanMillimeter:                   100,
			MaxMomentNewtonMillimeter:        1000000,
			TotalLoadGram:                    1200,
			UtilizationPPM:                   1200000,
			CalculatedMomentNewtonMillimeter: 0,
			SafetyMarginPPM:                  -200000,
		}},
		Points: []domain.RiggingPoint{{ID: "point-" + suffix, LineID: lineID, Code: "P-" + suffix, HoistRatedLoadGram: 1000, PositionMillimeter: 50}},
		Loads:  []domain.SuspendedLoad{{ID: loadID, LineID: lineID, PointID: "point-" + suffix, ComponentCode: "LOAD-" + suffix, Description: "测试载荷", WeightGram: 1200, PositionMillimeter: 50, Quantity: 1, SubmittedBy: "operator"}},
		Findings: []domain.SafetyFinding{{
			ID: "finding-" + suffix, LineID: lineID, SourceType: "CALCULATION", RuleCode: "RATED_LOAD_EXCEEDED", Status: domain.FindingOpen,
		}},
		Calculation: &domain.CalculationSnapshot{RuleSetVersion: "RIG-2026.1", InputDigest: "before-" + suffix, Lines: []domain.LineCalculation{{LineID: lineID, LineCode: "LX-" + suffix, TotalLoadGram: 1200, UtilizationPPM: 1200000, Passed: false}}},
	}
}

func TestConcurrentPreviewWorkspacesRemainIsolated(t *testing.T) {
	firstAtCalculation := make(chan struct{})
	releaseFirst := make(chan struct{})
	var clockCalls atomic.Int32
	fixed := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		if clockCalls.Add(1) == 1 {
			close(firstAtCalculation)
			<-releaseFirst
		}
		return fixed
	}
	repo := &previewRepository{sessions: map[string]*domain.RiggingSession{
		"session-a": previewSession("session-a", "A"),
		"session-b": previewSession("session-b", "B"),
	}}
	service := application.NewServiceWith(repo, clock, func() string { return "preview-id" })
	type outcome struct {
		preview *application.LoadPlanPreview
		err     error
	}
	firstDone := make(chan outcome, 1)
	go func() {
		preview, err := service.PreviewLoadPlan(context.Background(), "session-a", application.PreviewLoadPlanCommand{Revisions: []application.LoadRevisionInput{{LoadID: "load-A", WeightGram: 500, PositionMillimeter: 50}}})
		firstDone <- outcome{preview: preview, err: err}
	}()
	<-firstAtCalculation

	secondDone := make(chan outcome, 1)
	go func() {
		preview, err := service.PreviewLoadPlan(context.Background(), "session-b", application.PreviewLoadPlanCommand{Revisions: []application.LoadRevisionInput{{LoadID: "load-B", WeightGram: 400, PositionMillimeter: 50}}})
		secondDone <- outcome{preview: preview, err: err}
	}()
	second := <-secondDone
	if second.err != nil {
		t.Fatalf("第二个预演失败: %v", second.err)
	}
	close(releaseFirst)
	first := <-firstDone
	if first.err != nil {
		t.Fatalf("第一个预演失败: %v", first.err)
	}
	if len(first.preview.Impacts) != 1 || first.preview.Impacts[0].LineID != "line-A" || first.preview.Impacts[0].LineCode != "LX-A" {
		t.Fatalf("第一个作业的预演被并发作业污染: %#v", first.preview.Impacts)
	}
}
