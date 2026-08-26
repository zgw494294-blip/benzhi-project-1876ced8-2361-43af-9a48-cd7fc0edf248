package application

import (
	"context"
	"rigging-readiness-desk/internal/domain"
	"strings"
	"time"
)

type Page[T any] struct {
	Items  []T `json:"items"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

type SessionListFilter struct {
	Status          domain.SessionStatus
	Venue           string
	OperatorID      string
	PerformanceFrom *time.Time
	PerformanceTo   *time.Time
	Keyword         string
	DueBefore       *time.Time
	PendingOnly     bool
	Offset          int
	Limit           int
}

type QueueSummary struct {
	StatusCounts            map[domain.SessionStatus]int `json:"statusCounts"`
	OpenFindingCount        int                          `json:"openFindingCount"`
	UpcomingUnreleasedCount int                          `json:"upcomingUnreleasedCount"`
}

type FindingSummary struct {
	OpenCount int            `json:"openCount"`
	BySource  map[string]int `json:"bySource"`
}

type SessionQueueItem struct {
	ID                   string               `json:"id"`
	Title                string               `json:"title"`
	Venue                string               `json:"venue"`
	PerformanceAt        time.Time            `json:"performanceAt"`
	OperatorID           string               `json:"operatorId"`
	Status               domain.SessionStatus `json:"status"`
	Version              int64                `json:"version"`
	UpdatedAt            time.Time            `json:"updatedAt"`
	NextStep             string               `json:"nextStep"`
	MissingPrerequisites []string             `json:"missingPrerequisites"`
	BlockingSummary      FindingSummary       `json:"blockingSummary"`
}

type SessionQueuePage struct {
	Items   []SessionQueueItem `json:"items"`
	Offset  int                `json:"offset"`
	Limit   int                `json:"limit"`
	Total   int                `json:"total"`
	Summary QueueSummary       `json:"summary"`
}

func pageBounds(offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return offset, limit
}
func (s *Service) GetSession(ctx context.Context, id string) (*domain.RiggingSession, error) {
	return s.repo.Get(ctx, id)
}
func (s *Service) ListSessions(ctx context.Context, offset, limit int) (Page[domain.RiggingSession], error) {
	offset, limit = pageBounds(offset, limit)
	items, total, err := s.repo.List(ctx, offset, limit)
	return Page[domain.RiggingSession]{Items: items, Offset: offset, Limit: limit, Total: total}, err
}

func ValidateSessionListFilter(filter SessionListFilter) (SessionListFilter, error) {
	if filter.Status != "" {
		valid := false
		for _, status := range []domain.SessionStatus{domain.StatusDraft, domain.StatusBaselined, domain.StatusModeled, domain.StatusInspected, domain.StatusApproved, domain.StatusFrozen, domain.StatusReleased} {
			if filter.Status == status {
				valid = true
				break
			}
		}
		if !valid {
			return filter, domain.NewError(domain.ErrValidation, "status", "未知作业状态")
		}
	}
	filter.Venue = strings.TrimSpace(filter.Venue)
	filter.OperatorID = strings.TrimSpace(filter.OperatorID)
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	if len([]rune(filter.Keyword)) > 100 {
		return filter, domain.NewError(domain.ErrValidation, "keyword", "标题关键字不得超过 100 个字符")
	}
	if filter.PerformanceFrom != nil && filter.PerformanceTo != nil && filter.PerformanceFrom.After(*filter.PerformanceTo) {
		return filter, domain.NewError(domain.ErrValidation, "performanceFrom", "演出时间起点不得晚于终点")
	}
	if filter.Offset < 0 {
		return filter, domain.NewError(domain.ErrValidation, "offset", "分页偏移不得小于零")
	}
	if filter.Offset > 1_000_000 {
		return filter, domain.NewError(domain.ErrValidation, "offset", "分页偏移超过允许范围")
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return filter, domain.NewError(domain.ErrValidation, "limit", "分页条数必须在 1 到 100 之间")
	}
	if filter.PendingOnly && filter.DueBefore == nil {
		return filter, domain.NewError(domain.ErrValidation, "dueBefore", "筛选临近未放行待办时必须提供截止时间")
	}
	return filter, nil
}

func (s *Service) QuerySessionQueue(ctx context.Context, filter SessionListFilter) (SessionQueuePage, error) {
	filter, err := ValidateSessionListFilter(filter)
	if err != nil {
		return SessionQueuePage{}, err
	}
	sessions, total, summary, err := s.repo.QuerySessions(ctx, filter)
	if err != nil {
		return SessionQueuePage{}, err
	}
	items := make([]SessionQueueItem, 0, len(sessions))
	for i := range sessions {
		items = append(items, buildQueueItem(&sessions[i]))
	}
	if summary.StatusCounts == nil {
		summary.StatusCounts = map[domain.SessionStatus]int{}
	}
	return SessionQueuePage{Items: items, Offset: filter.Offset, Limit: filter.Limit, Total: total, Summary: summary}, nil
}

func buildQueueItem(session *domain.RiggingSession) SessionQueueItem {
	readiness := &ReadinessView{Lines: []LineReadiness{}}
	calculationByLine := map[string]bool{}
	if session.Calculation != nil {
		for _, result := range session.Calculation.Lines {
			calculationByLine[result.LineID] = result.Passed
		}
	}
	for _, line := range session.Lines {
		item := LineReadiness{LineID: line.ID, CalculationPass: calculationByLine[line.ID], ChecksRequired: len(domain.RequiredChecks)}
		for _, check := range session.Checks {
			if check.LineID == line.ID {
				item.ChecksRecorded++
				if check.Passed {
					item.ChecksPassed++
				}
			}
		}
		readiness.Lines = append(readiness.Lines, item)
	}
	for _, finding := range session.Findings {
		if finding.Status == domain.FindingOpen {
			readiness.OpenFindingCount++
		}
	}
	gates := buildGates(session, readiness)
	missing := []string{}
	for _, gate := range gates {
		if !gate.Satisfied {
			missing = append(missing, gate.Explanation)
		}
	}
	next := "作业已放行"
	if len(missing) > 0 {
		next = missing[0]
	}
	bySource := map[string]int{}
	open := 0
	for _, finding := range session.Findings {
		if finding.Status == domain.FindingOpen {
			open++
			bySource[finding.SourceType]++
		}
	}
	return SessionQueueItem{ID: session.ID, Title: session.Title, Venue: session.Venue, PerformanceAt: session.PerformanceAt, OperatorID: session.OperatorID, Status: session.Status, Version: session.Version, UpdatedAt: session.UpdatedAt, NextStep: next, MissingPrerequisites: missing, BlockingSummary: FindingSummary{OpenCount: open, BySource: bySource}}
}
func (s *Service) ListAudit(ctx context.Context, id string, offset, limit int) (Page[AuditEvent], error) {
	offset, limit = pageBounds(offset, limit)
	items, total, err := s.repo.ListAudit(ctx, id, offset, limit)
	return Page[AuditEvent]{Items: items, Offset: offset, Limit: limit, Total: total}, err
}
func (s *Service) OpenFindings(ctx context.Context, id string, offset, limit int) (Page[domain.SafetyFinding], error) {
	session, err := s.repo.Get(ctx, id)
	if err != nil {
		return Page[domain.SafetyFinding]{}, err
	}
	all := []domain.SafetyFinding{}
	for _, finding := range session.Findings {
		if finding.Status == domain.FindingOpen {
			all = append(all, finding)
		}
	}
	offset, limit = pageBounds(offset, limit)
	end := offset + limit
	if offset > len(all) {
		offset = len(all)
	}
	if end > len(all) {
		end = len(all)
	}
	return Page[domain.SafetyFinding]{Items: all[offset:end], Offset: offset, Limit: limit, Total: len(all)}, nil
}
