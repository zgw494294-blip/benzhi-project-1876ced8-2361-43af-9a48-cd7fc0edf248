package storage

import (
	"context"
	"path/filepath"
	"rigging-readiness-desk/internal/application"
	"testing"
	"time"
)

func TestPersistenceSurvivesReopenAndIdempotency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desk.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	ctx := context.Background()
	created, err := service.CreateSession(ctx, application.CreateSessionCommand{Title: "持久化演出", Venue: "剧场", PerformanceAt: time.Now().Add(time.Hour), OperatorID: "op", RuleSetVersion: "R1", IdempotencyKey: "create-key"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.CreateSession(ctx, application.CreateSessionCommand{IdempotencyKey: "create-key"})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != created.ID {
		t.Fatal("幂等创建返回了不同作业")
	}
	if _, err = service.ConfirmBaseline(ctx, created.ID, application.BaselineCommand{VersionCommand: application.VersionCommand{ExpectedVersion: created.Version, IdempotencyKey: "base-key"}, BaselineRef: "B1"}); err != nil {
		t.Fatal(err)
	}
	if err = service.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != "BASELINED" || loaded.BaselineRef != "B1" {
		t.Fatalf("重启恢复内容错误: %+v", loaded)
	}
}

func TestBatchAndRemediationPlanCommitOnce(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	defer service.Close()
	ctx := context.Background()
	session, err := service.CreateSession(ctx, application.CreateSessionCommand{Title: "方案演出", Venue: "剧场", PerformanceAt: time.Now().Add(time.Hour), OperatorID: "operator", RuleSetVersion: "R1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err = service.ConfirmBaseline(ctx, session.ID, application.BaselineCommand{VersionCommand: application.VersionCommand{ExpectedVersion: session.Version}, BaselineRef: "B1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err = service.AddLine(ctx, session.ID, application.AddLineCommand{VersionCommand: application.VersionCommand{ExpectedVersion: session.Version}, Code: "L1", RatedLoadGram: 1_000_000, SpanMillimeter: 10_000, MaxMomentNewtonMillimeter: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	session, err = service.AddPoint(ctx, session.ID, application.AddPointCommand{VersionCommand: application.VersionCommand{ExpectedVersion: session.Version}, LineID: session.Lines[0].ID, Code: "P1", HoistRatedLoadGram: 1_000_000, PositionMillimeter: 5_000})
	if err != nil {
		t.Fatal(err)
	}
	batch := application.AddLoadsCommand{VersionCommand: application.VersionCommand{ExpectedVersion: session.Version, IdempotencyKey: "batch-1"}, Loads: []application.AddLoadInput{{LineID: session.Lines[0].ID, PointID: session.Points[0].ID, ComponentCode: "C1", Description: "灯具", WeightGram: 100_000, PositionMillimeter: 0, Quantity: 1, SubmittedBy: "operator"}, {LineID: session.Lines[0].ID, PointID: session.Points[0].ID, ComponentCode: "C2", Description: "景片", WeightGram: 100_000, PositionMillimeter: 0, Quantity: 1, SubmittedBy: "operator"}}}
	result, err := service.AddLoads(ctx, session.ID, batch)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.AddLoads(ctx, session.ID, batch)
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != again.Version || result.Loads[0].ID != again.Loads[0].ID {
		t.Fatal("批量幂等重试没有返回首次结果")
	}
	session, _ = service.GetSession(ctx, session.ID)
	session, err = service.FinalizeModel(ctx, session.ID, application.VersionCommand{ExpectedVersion: session.Version})
	if err != nil {
		t.Fatal(err)
	}
	session, err = service.Calculate(ctx, session.ID, application.VersionCommand{ExpectedVersion: session.Version})
	if err != nil {
		t.Fatal(err)
	}
	beforeVersion := session.Version
	revisions := []application.LoadRevisionInput{{LoadID: session.Loads[0].ID, WeightGram: 100_000, PositionMillimeter: 5_000}, {LoadID: session.Loads[1].ID, WeightGram: 100_000, PositionMillimeter: 5_000}}
	preview, err := service.PreviewLoadPlan(ctx, session.ID, application.PreviewLoadPlanCommand{Revisions: revisions})
	if err != nil {
		t.Fatal(err)
	}
	unchanged, _ := service.GetSession(ctx, session.ID)
	if unchanged.Version != beforeVersion || unchanged.Loads[0].PositionMillimeter != 0 {
		t.Fatal("预演修改了作业")
	}
	applied, err := service.ApplyLoadPlan(ctx, session.ID, application.ApplyLoadPlanCommand{VersionCommand: application.VersionCommand{ExpectedVersion: beforeVersion}, Revisions: revisions, ProposalDigest: preview.ProposalDigest})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Version != beforeVersion+1 || applied.Loads[0].PositionMillimeter != 5_000 || applied.HasOpenFindings() {
		t.Fatal("整改方案未原子应用或未关闭计算阻断项")
	}
}
