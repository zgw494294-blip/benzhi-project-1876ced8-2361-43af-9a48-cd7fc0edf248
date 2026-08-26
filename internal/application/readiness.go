package application

import (
	"context"
	"rigging-readiness-desk/internal/domain"
	"sync"
)

type readinessCache struct {
	mu    sync.RWMutex
	views map[string]*ReadinessView
}

func newReadinessCache() *readinessCache {
	return &readinessCache{views: map[string]*ReadinessView{}}
}

func (c *readinessCache) get(sessionID string) (*ReadinessView, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	view, ok := c.views[sessionID]
	return view, ok
}

func (c *readinessCache) put(sessionID string, view *ReadinessView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.views[sessionID] = view
}

type ReadinessGate struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Satisfied   bool   `json:"satisfied"`
	Explanation string `json:"explanation"`
}

type LineReadiness struct {
	LineID          string `json:"lineId"`
	LineCode        string `json:"lineCode"`
	CalculationPass bool   `json:"calculationPass"`
	ChecksRecorded  int    `json:"checksRecorded"`
	ChecksRequired  int    `json:"checksRequired"`
	ChecksPassed    int    `json:"checksPassed"`
	OpenFindings    int    `json:"openFindings"`
}

type ReadinessView struct {
	SessionID        string               `json:"sessionId"`
	Status           domain.SessionStatus `json:"status"`
	Version          int64                `json:"version"`
	LineCount        int                  `json:"lineCount"`
	PointCount       int                  `json:"pointCount"`
	LoadCount        int                  `json:"loadCount"`
	OpenFindingCount int                  `json:"openFindingCount"`
	Lines            []LineReadiness      `json:"lines"`
	Gates            []ReadinessGate      `json:"gates"`
	NextActions      []string             `json:"nextActions"`
}

func (s *Service) GetReadiness(ctx context.Context, id string) (*ReadinessView, error) {
	if view, ok := s.readiness.get(id); ok {
		return view, nil
	}
	session, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	view := &ReadinessView{SessionID: session.ID, Status: session.Status, Version: session.Version, LineCount: len(session.Lines), PointCount: len(session.Points), LoadCount: len(session.Loads), Lines: []LineReadiness{}, Gates: []ReadinessGate{}, NextActions: []string{}}
	calculationByLine := map[string]bool{}
	if session.Calculation != nil {
		for _, result := range session.Calculation.Lines {
			calculationByLine[result.LineID] = result.Passed
		}
	}
	for _, line := range session.Lines {
		item := LineReadiness{LineID: line.ID, LineCode: line.Code, CalculationPass: calculationByLine[line.ID], ChecksRequired: len(domain.RequiredChecks)}
		for _, check := range session.Checks {
			if check.LineID == line.ID {
				item.ChecksRecorded++
				if check.Passed {
					item.ChecksPassed++
				}
			}
		}
		for _, finding := range session.Findings {
			if finding.LineID == line.ID && finding.Status == domain.FindingOpen {
				item.OpenFindings++
				view.OpenFindingCount++
			}
		}
		view.Lines = append(view.Lines, item)
	}
	view.Gates = buildGates(session, view)
	for _, gate := range view.Gates {
		if !gate.Satisfied {
			view.NextActions = append(view.NextActions, gate.Explanation)
		}
	}
	if len(view.NextActions) == 0 {
		view.NextActions = []string{"所有放行关口均已满足，凭据可用于该冻结场次。"}
	}
	s.readiness.put(id, view)
	return view, nil
}

func buildGates(session *domain.RiggingSession, view *ReadinessView) []ReadinessGate {
	modelReady := len(session.Lines) > 0 && len(session.Points) >= len(session.Lines) && len(session.Loads) > 0
	calculationReady := session.Calculation != nil
	checksReady := len(session.Lines) > 0
	for _, line := range view.Lines {
		calculationReady = calculationReady && line.CalculationPass
		checksReady = checksReady && line.ChecksRecorded == line.ChecksRequired && line.ChecksPassed == line.ChecksRequired
	}
	reviewReady := session.Review != nil && session.Review.Decision == "APPROVE" && session.Review.ReviewerID != session.OperatorID
	return []ReadinessGate{
		{Code: "BASELINE", Label: "设备基线", Satisfied: session.BaselineRef != "", Explanation: "确认当前场地设备基线。"},
		{Code: "MODEL", Label: "载荷模型", Satisfied: modelReady, Explanation: "为每根吊杆登记吊点、提升机能力和悬挂构件。"},
		{Code: "CALCULATION", Label: "安全计算", Satisfied: calculationReady, Explanation: "重新计算并消除载荷或力矩超限。"},
		{Code: "INSPECTION", Label: "现场检查", Satisfied: checksReady, Explanation: "完成每根吊杆的五类通过检查。"},
		{Code: "FINDINGS", Label: "整改闭环", Satisfied: view.OpenFindingCount == 0, Explanation: "为全部阻断项登记整改并完成定向复验。"},
		{Code: "REVIEW", Label: "独立复核", Satisfied: reviewReady, Explanation: "由不同于经办人的复核员批准。"},
		{Code: "FREEZE", Label: "清单冻结", Satisfied: session.Frozen != nil, Explanation: "生成并冻结按执行顺序排列的场景清单。"},
		{Code: "CERTIFICATE", Label: "启用凭据", Satisfied: session.Certificate != nil && session.VerifyCertificate(), Explanation: "签发凭据并重算摘要完成校验。"},
	}
}

func (s *Service) GetCalculation(ctx context.Context, id string) (*domain.CalculationSnapshot, error) {
	session, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if session.Calculation == nil {
		return nil, domain.NewError(domain.ErrNotFound, "calculation", "尚未生成计算结果")
	}
	return session.Calculation, nil
}

func (s *Service) GetCertificate(ctx context.Context, certificateID string) (*domain.ReleaseCertificate, error) {
	session, err := s.repo.GetByCertificate(ctx, certificateID)
	if err != nil {
		return nil, err
	}
	if session.Certificate == nil || session.Certificate.ID != certificateID {
		return nil, domain.NewError(domain.ErrNotFound, "certificateId", "凭据不存在")
	}
	copy := *session.Certificate
	if session.VerifyCertificate() {
		copy.VerificationStatus = "VALID"
	} else {
		copy.VerificationStatus = "INVALID"
	}
	return &copy, nil
}
