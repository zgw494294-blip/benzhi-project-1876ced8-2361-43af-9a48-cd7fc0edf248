package readinesscache_test

import (
	"context"
	"fmt"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/domain"
	"rigging-readiness-desk/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestReadinessViewCacheIsVersionedAndIsolated(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	t.Cleanup(func() { _ = service.Close() })

	ctx := context.Background()
	session, err := service.CreateSession(ctx, application.CreateSessionCommand{
		Title:          "缓存边界复现",
		Venue:          "实验剧场",
		PerformanceAt:  time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC),
		OperatorID:     "operator-cache",
		RuleSetVersion: "R1",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.GetReadiness(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.NextActions) == 0 {
		t.Fatal("初始就绪视图缺少后续动作")
	}
	originalAction := first.NextActions[0]
	first.NextActions[0] = "调用方局部展示文案"

	problems := make([]string, 0, 2)
	again, err := service.GetReadiness(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.NextActions[0] != originalAction {
		problems = append(problems, fmt.Sprintf("缓存返回了被调用方污染的切片 %q", again.NextActions[0]))
	}

	baselined, err := service.ConfirmBaseline(ctx, session.ID, application.BaselineCommand{
		VersionCommand: application.VersionCommand{ExpectedVersion: session.Version},
		BaselineRef:    "BASELINE-2030",
	})
	if err != nil {
		t.Fatal(err)
	}
	afterMutation, err := service.GetReadiness(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterMutation.Version != baselined.Version || afterMutation.Status != domain.StatusBaselined || !gateSatisfied(afterMutation, "BASELINE") {
		problems = append(problems, fmt.Sprintf("状态变更后仍返回 status=%s version=%d baseline=%t，持久化版本为 %d", afterMutation.Status, afterMutation.Version, gateSatisfied(afterMutation, "BASELINE"), baselined.Version))
	}
	if len(problems) > 0 {
		t.Fatalf("TestReadinessViewCacheIsVersionedAndIsolated: %s", strings.Join(problems, "; "))
	}
}

func gateSatisfied(view *application.ReadinessView, code string) bool {
	for _, gate := range view.Gates {
		if gate.Code == code {
			return gate.Satisfied
		}
	}
	return false
}
