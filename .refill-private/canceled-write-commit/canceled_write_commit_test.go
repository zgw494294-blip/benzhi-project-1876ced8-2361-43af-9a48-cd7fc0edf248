package canceledwritecommit_test

import (
	"context"
	"errors"
	"fmt"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"rigging-readiness-desk/internal/storage"
	"testing"
	"time"
)

func TestCanceledWriteContextDoesNotCommit(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	fixedNow := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	sequence := 0
	var cancelNext context.CancelFunc
	service := application.NewServiceWith(store, func() time.Time {
		if cancelNext != nil {
			cancelNext()
			cancelNext = nil
		}
		return fixedNow
	}, func() string {
		sequence++
		return fmt.Sprintf("id-%d", sequence)
	})

	kept, err := service.CreateSession(context.Background(), application.CreateSessionCommand{
		Title:          "保留作业",
		Venue:          "测试剧场",
		PerformanceAt:  fixedNow.Add(24 * time.Hour),
		OperatorID:     "operator-a",
		RuleSetVersion: "RIG-2026.1",
	})
	if err != nil {
		t.Fatalf("准备作业失败: %v", err)
	}

	createCtx, cancelCreate := context.WithCancel(context.Background())
	cancelNext = cancelCreate
	_, createErr := service.CreateSession(createCtx, application.CreateSessionCommand{
		Title:          "不应提交的作业",
		Venue:          "测试剧场",
		PerformanceAt:  fixedNow.Add(48 * time.Hour),
		OperatorID:     "operator-b",
		RuleSetVersion: "RIG-2026.1",
	})
	if !errors.Is(createErr, context.Canceled) {
		t.Errorf("取消后的 CreateSession 应返回 context.Canceled，实际为 %v", createErr)
	}
	_, total, err := store.List(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("查询作业数失败: %v", err)
	}
	if total != 1 {
		t.Errorf("取消后的 CreateSession 不应提交，实际作业数为 %d", total)
	}

	updateCtx, cancelUpdate := context.WithCancel(context.Background())
	cancelNext = cancelUpdate
	_, updateErr := service.ConfirmBaseline(updateCtx, kept.ID, application.BaselineCommand{
		VersionCommand: application.VersionCommand{ExpectedVersion: kept.Version, ActorID: kept.OperatorID},
		BaselineRef:    "CANCELED-BASELINE",
	})
	if !errors.Is(updateErr, context.Canceled) {
		t.Errorf("取消后的 ConfirmBaseline 应返回 context.Canceled，实际为 %v", updateErr)
	}
	persisted, err := store.Get(context.Background(), kept.ID)
	if err != nil {
		t.Fatalf("读取保留作业失败: %v", err)
	}
	if persisted.Status != domain.StatusDraft || persisted.Version != kept.Version {
		t.Errorf("取消后的 Save 不应提交，实际状态=%s version=%d", persisted.Status, persisted.Version)
	}
}
