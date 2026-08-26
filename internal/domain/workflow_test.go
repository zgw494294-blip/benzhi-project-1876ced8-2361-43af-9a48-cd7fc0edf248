package domain

import (
	"testing"
	"time"
)

func TestCalculationDeterministicAndBlocksOverload(t *testing.T) {
	now := time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC)
	session, err := NewSession("s1", "演出", "剧场", now.Add(time.Hour), "operator", "R1", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = session.ConfirmBaseline("B1"); err != nil {
		t.Fatal(err)
	}
	if err = session.AddLine(RiggingLine{ID: "l1", Code: "LX-1", RatedLoadGram: 100000, SpanMillimeter: 10000, MaxMomentNewtonMillimeter: 100000000}); err != nil {
		t.Fatal(err)
	}
	if err = session.AddPoint(RiggingPoint{ID: "p1", LineID: "l1", Code: "LX-1-P1", HoistRatedLoadGram: 100000, PositionMillimeter: 5000}); err != nil {
		t.Fatal(err)
	}
	if err = session.AddLoad(SuspendedLoad{ID: "w1", LineID: "l1", PointID: "p1", ComponentCode: "C1", Description: "景片", WeightGram: 120000, PositionMillimeter: 5000, Quantity: 1, SubmittedBy: "operator", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err = session.FinalizeModel(); err != nil {
		t.Fatal(err)
	}
	counter := 0
	ids := func() string { counter++; return "f" + string(rune('0'+counter)) }
	first, err := session.Calculate(now, ids)
	if err != nil {
		t.Fatal(err)
	}
	second, err := session.Calculate(now.Add(time.Minute), ids)
	if err != nil {
		t.Fatal(err)
	}
	if first.InputDigest != second.InputDigest {
		t.Fatalf("相同输入摘要不稳定: %s != %s", first.InputDigest, second.InputDigest)
	}
	if !session.HasOpenFindings() {
		t.Fatal("超载未产生阻断项")
	}
	if first.Lines[0].Passed {
		t.Fatal("超载计算不应通过")
	}
}
func TestIndependentReviewAndFrozenImmutability(t *testing.T) {
	now := time.Now().UTC()
	s, _ := NewSession("s", "演出", "场地", now.Add(time.Hour), "operator", "R1", now)
	s.Status = StatusInspected
	s.Calculation = &CalculationSnapshot{}
	if err := s.Approve("operator", "APPROVE", "", now); err == nil {
		t.Fatal("经办人不应自审")
	}
	if err := s.Approve("reviewer", "APPROVE", "核对完成", now); err != nil {
		t.Fatal(err)
	}
	s.Lines = []RiggingLine{{ID: "l", Code: "LX", TotalLoadGram: 100}}
	s.Review = &SafetyReview{ReviewerID: "reviewer", Decision: "APPROVE"}
	if _, err := s.BuildManifest(now); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLine(RiggingLine{}); err == nil {
		t.Fatal("冻结后修改应被拒绝")
	}
}

func TestBatchLoadsAreAtomicAndLocateInvalidRow(t *testing.T) {
	now := time.Now().UTC()
	s, _ := NewSession("batch", "演出", "场地", now.Add(time.Hour), "operator", "R1", now)
	_ = s.ConfirmBaseline("B1")
	_ = s.AddLine(RiggingLine{ID: "l1", Code: "L1", RatedLoadGram: 1000, SpanMillimeter: 1000, MaxMomentNewtonMillimeter: 100000})
	_ = s.AddLine(RiggingLine{ID: "l2", Code: "L2", RatedLoadGram: 1000, SpanMillimeter: 1000, MaxMomentNewtonMillimeter: 100000})
	_ = s.AddPoint(RiggingPoint{ID: "p1", LineID: "l1", Code: "P1", HoistRatedLoadGram: 1000, PositionMillimeter: 500})
	_ = s.AddPoint(RiggingPoint{ID: "p2", LineID: "l2", Code: "P2", HoistRatedLoadGram: 1000, PositionMillimeter: 500})
	err := s.AddLoads([]SuspendedLoad{
		{ID: "w1", LineID: "l1", PointID: "p1", ComponentCode: "C1", Description: "灯具", WeightGram: 100, PositionMillimeter: 500, Quantity: 1, SubmittedBy: "operator", CreatedAt: now},
		{ID: "w2", LineID: "l1", PointID: "p2", ComponentCode: "C2", Description: "景片", WeightGram: 100, PositionMillimeter: 500, Quantity: 1, SubmittedBy: "operator", CreatedAt: now},
	})
	if err == nil || len(s.Loads) != 0 {
		t.Fatalf("无效批次留下了部分构件: err=%v loads=%d", err, len(s.Loads))
	}
	if got := err.Error(); got != "loads[1].pointId: 载荷吊点不属于指定吊杆" {
		t.Fatalf("错误未定位到第二行 pointId: %s", got)
	}
}

func TestReviewReturnRequiresRemediationAndTargetedRecheck(t *testing.T) {
	now := time.Now().UTC()
	s, _ := NewSession("review", "演出", "场地", now.Add(time.Hour), "operator", "R1", now)
	s.Status = StatusInspected
	s.Lines = []RiggingLine{{ID: "l1", Code: "L1"}}
	if err := s.ReturnReview("reviewer", "限位证据不足", "LIMIT_CHECK", []string{"l1"}, 8, now, func() string { return "f1" }); err != nil {
		t.Fatal(err)
	}
	if !s.HasOpenFindings() || s.Findings[0].OriginVersion != 8 {
		t.Fatal("退回未记录版本并生成开放阻断项")
	}
	if err := s.AssignRemediation("f1", "operator", "补充限位动作视频"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCheck(LineCheck{LineID: "l1", Kind: CheckLimit, Passed: true, Evidence: "复验视频", InspectorID: "inspector"}, now.Add(time.Minute), func() string { return "c1" }); err != nil {
		t.Fatal(err)
	}
	if s.HasOpenFindings() {
		t.Fatal("完成对应限位复验后复核阻断项仍未关闭")
	}
}
