package failedmutationcacheleak_test

import (
	"context"
	"path/filepath"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"rigging-readiness-desk/internal/storage"
	"testing"
	"time"
)

func TestFailedReviewDoesNotLeakCachedAggregate(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(filepath.Join(t.TempDir(), "rigging.db"))
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	defer store.Close()

	now := time.Date(2027, 2, 3, 4, 5, 6, 0, time.UTC)
	session := &domain.RiggingSession{
		ID: "session-cache-leak", Title: "缓存所有权复现", Venue: "测试剧场",
		PerformanceAt: now.Add(24 * time.Hour), OperatorID: "operator-a", RuleSetVersion: "rules-v1",
		Status: domain.StatusInspected, Version: 7, CreatedAt: now, UpdatedAt: now,
		Lines: []domain.RiggingLine{{ID: "line-valid", SessionID: "session-cache-leak", Code: "LX-1"}},
	}
	event := application.AuditEvent{ID: "audit-created", SessionID: session.ID, Type: "SESSION_CREATED", ActorID: session.OperatorID, Detail: "测试建档", CreatedAt: now}
	if err = store.Create(ctx, session, event, nil); err != nil {
		t.Fatalf("准备作业: %v", err)
	}

	service := application.NewServiceWith(store, func() time.Time { return now }, func() string { return "finding-from-failed-review" })
	_, err = service.Review(ctx, session.ID, application.ReviewCommand{
		VersionCommand: application.VersionCommand{ExpectedVersion: 7},
		ReviewerID:     "reviewer-b", Decision: "RETURN", Reason: "需要复查",
		Category: "BRAKE", AffectedLineIDs: []string{"line-valid", "line-missing"},
	})
	if err == nil {
		t.Fatal("包含不存在吊杆的复核退回应失败")
	}

	persisted, _, err := store.List(ctx, 0, 10)
	if err != nil {
		t.Fatalf("读取持久化快照: %v", err)
	}
	if len(persisted) != 1 || len(persisted[0].Findings) != 0 || persisted[0].Version != 7 {
		t.Fatalf("失败命令不应写入数据库: %+v", persisted)
	}
	audits, total, err := store.ListAudit(ctx, session.ID, 0, 10)
	if err != nil || total != 1 || len(audits) != 1 {
		t.Fatalf("失败命令不应追加审计: total=%d audits=%d err=%v", total, len(audits), err)
	}

	observed, err := service.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("再次查询作业: %v", err)
	}
	if len(observed.Findings) != 0 || observed.Version != 7 {
		t.Fatalf("失败命令污染了后续查询：version=%d findings=%d", observed.Version, len(observed.Findings))
	}
}
