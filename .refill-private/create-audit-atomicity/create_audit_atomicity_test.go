package createauditatomicity_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/storage"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCreateAuditFailureRollsBackAggregateAndIdempotency(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "desk.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	triggerDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer triggerDB.Close()
	_, err = triggerDB.ExecContext(ctx, `CREATE TRIGGER reject_session_created_audit
		BEFORE INSERT ON audit_events
		WHEN NEW.event_type = 'SESSION_CREATED'
		BEGIN
			SELECT RAISE(ABORT, 'forced audit failure');
		END`)
	if err != nil {
		t.Fatal(err)
	}

	nextID := 0
	service := application.NewServiceWith(store, func() time.Time {
		return time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	}, func() string {
		nextID++
		return fmt.Sprintf("id-%d", nextID)
	})
	cmd := application.CreateSessionCommand{
		Title:          "审计原子性演出",
		Venue:          "实验剧场",
		PerformanceAt:  time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC),
		OperatorID:     "operator-a",
		RuleSetVersion: "R1",
		IdempotencyKey: "create-audit-failure",
	}

	created, createErr := service.CreateSession(ctx, cmd)
	page, listErr := service.ListSessions(ctx, 0, 20)
	if listErr != nil {
		t.Fatal(listErr)
	}
	replayed, replayErr := service.CreateSession(ctx, cmd)

	problems := make([]string, 0, 4)
	if createErr == nil {
		problems = append(problems, "首次创建没有报告被触发器拒绝的审计错误")
	}
	if page.Total != 0 {
		problems = append(problems, fmt.Sprintf("创建返回错误后仍持久化了 %d 个作业", page.Total))
	}
	if replayErr == nil {
		problems = append(problems, "相同 Idempotency-Key 重试命中了已提交的成功快照")
	}
	if created != nil || replayed != nil {
		problems = append(problems, "失败创建或其重试向调用方暴露了作业")
	}
	if len(problems) != 0 {
		t.Fatalf("创建事务边界被审计失败穿透：%s", strings.Join(problems, "；"))
	}
}
