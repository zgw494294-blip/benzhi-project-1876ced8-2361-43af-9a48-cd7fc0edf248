package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"rigging-readiness-desk/internal/domain"
	"sort"
	"strings"
)

type LoadPlanImpact struct {
	LineID   string                 `json:"lineId"`
	LineCode string                 `json:"lineCode"`
	Before   domain.LineCalculation `json:"before"`
	After    domain.LineCalculation `json:"after"`
}
type FindingReference struct {
	LineID   string `json:"lineId"`
	RuleCode string `json:"ruleCode"`
}
type LoadPlanPreview struct {
	SessionID         string             `json:"sessionId"`
	Version           int64              `json:"version"`
	RuleSetVersion    string             `json:"ruleSetVersion"`
	ProposalDigest    string             `json:"proposalDigest"`
	Impacts           []LoadPlanImpact   `json:"impacts"`
	ExpectedClosed    []FindingReference `json:"expectedClosed"`
	ExpectedPreserved []FindingReference `json:"expectedPreserved"`
	ExpectedNew       []FindingReference `json:"expectedNew"`
	Applicable        bool               `json:"applicable"`
}

func (s *Service) PreviewLoadPlan(ctx context.Context, id string, cmd PreviewLoadPlanCommand) (*LoadPlanPreview, error) {
	session, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.previewLoadPlan(session, cmd.Revisions)
}

func (s *Service) ApplyLoadPlan(ctx context.Context, id string, cmd ApplyLoadPlanCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd.VersionCommand, "apply-load-plan", "REMEDIATION_PLAN_APPLIED", "原子应用整改载荷方案", func(session *domain.RiggingSession) error {
		preview, err := s.previewLoadPlan(session, cmd.Revisions)
		if err != nil {
			return err
		}
		if strings.TrimSpace(cmd.ProposalDigest) == "" {
			return domain.NewError(domain.ErrValidation, "proposalDigest", "必须回传预演方案摘要")
		}
		if cmd.ProposalDigest != preview.ProposalDigest {
			return domain.NewError(domain.ErrConflict, "proposalDigest", "方案内容、作业版本或规则版本已变化，请重新预演")
		}
		if len(preview.ExpectedNew) > 0 {
			return domain.NewError(domain.ErrState, "revisions", "方案会新增计算阻断项，不能应用")
		}
		for _, revision := range cmd.Revisions {
			if err = session.ReviseLoad(revision.LoadID, revision.WeightGram, revision.PositionMillimeter); err != nil {
				return err
			}
		}
		_, err = session.Calculate(s.now(), s.newID)
		return err
	})
}

func (s *Service) previewLoadPlan(session *domain.RiggingSession, revisions []LoadRevisionInput) (*LoadPlanPreview, error) {
	if session.Status != domain.StatusModeled && session.Status != domain.StatusInspected {
		return nil, domain.NewError(domain.ErrState, "status", "当前状态不允许预演整改载荷方案")
	}
	if session.Calculation == nil {
		return nil, domain.NewError(domain.ErrState, "calculation", "必须先完成载荷计算")
	}
	if len(revisions) == 0 {
		return nil, domain.NewError(domain.ErrValidation, "revisions", "至少修订一个悬挂构件")
	}
	if len(revisions) > domain.MaxLoadBatchSize {
		return nil, domain.NewError(domain.ErrValidation, "revisions", "方案修订条数超过上限")
	}
	normalized := append([]LoadRevisionInput(nil), revisions...)
	seen := map[string]struct{}{}
	affected := map[string]struct{}{}
	for i, revision := range normalized {
		if strings.TrimSpace(revision.LoadID) == "" {
			return nil, domain.NewError(domain.ErrValidation, fmt.Sprintf("revisions[%d].loadId", i), "构件标识不能为空")
		}
		if _, exists := seen[revision.LoadID]; exists {
			return nil, domain.NewError(domain.ErrValidation, fmt.Sprintf("revisions[%d].loadId", i), "方案内目标构件重复")
		}
		seen[revision.LoadID] = struct{}{}
		found := false
		for _, load := range session.Loads {
			if load.ID == revision.LoadID {
				found = true
				affected[load.LineID] = struct{}{}
				if !hasOpenCalculationFinding(session, load.LineID) {
					return nil, domain.NewError(domain.ErrState, fmt.Sprintf("revisions[%d].loadId", i), "构件所属吊杆没有尚未关闭的计算阻断项")
				}
				break
			}
		}
		if !found {
			return nil, domain.NewError(domain.ErrNotFound, fmt.Sprintf("revisions[%d].loadId", i), "构件不存在")
		}
	}
	copySession, err := s.reusablePreviewSession(session)
	if err != nil {
		return nil, err
	}
	for i, revision := range revisions {
		if err = copySession.ReviseLoad(revision.LoadID, revision.WeightGram, revision.PositionMillimeter); err != nil {
			return nil, indexedRevisionError(err, i)
		}
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].LoadID < normalized[j].LoadID })
	if _, err = copySession.Calculate(s.now(), func() string { return "preview-finding" }); err != nil {
		return nil, err
	}
	digest := loadPlanDigest(session, normalized)
	beforeLines := calculationMap(session.Calculation)
	afterLines := calculationMap(copySession.Calculation)
	lineIDs := make([]string, 0, len(affected))
	for lineID := range affected {
		lineIDs = append(lineIDs, lineID)
	}
	sort.Strings(lineIDs)
	impacts := make([]LoadPlanImpact, 0, len(lineIDs))
	for _, lineID := range lineIDs {
		impacts = append(impacts, LoadPlanImpact{LineID: lineID, LineCode: afterLines[lineID].LineCode, Before: beforeLines[lineID], After: afterLines[lineID]})
	}
	closed, preserved, added := compareCalculationFindings(session, copySession)
	return &LoadPlanPreview{SessionID: session.ID, Version: session.Version, RuleSetVersion: session.RuleSetVersion, ProposalDigest: digest, Impacts: impacts, ExpectedClosed: closed, ExpectedPreserved: preserved, ExpectedNew: added, Applicable: len(added) == 0}, nil
}

func (s *Service) reusablePreviewSession(session *domain.RiggingSession) (*domain.RiggingSession, error) {
	copySession, err := cloneSession(session)
	if err != nil {
		return nil, err
	}
	if s.previewWorkspace == nil {
		s.previewWorkspace = copySession
	} else {
		*s.previewWorkspace = *copySession
	}
	return s.previewWorkspace, nil
}

func cloneSession(session *domain.RiggingSession) (*domain.RiggingSession, error) {
	data, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	var copy domain.RiggingSession
	if err = json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}
	return &copy, nil
}

func loadPlanDigest(session *domain.RiggingSession, revisions []LoadRevisionInput) string {
	payload := struct {
		SessionID string              `json:"sessionId"`
		Version   int64               `json:"version"`
		Rule      string              `json:"ruleSetVersion"`
		Revisions []LoadRevisionInput `json:"revisions"`
	}{session.ID, session.Version, session.RuleSetVersion, revisions}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hasOpenCalculationFinding(session *domain.RiggingSession, lineID string) bool {
	for _, finding := range session.Findings {
		if finding.LineID == lineID && finding.SourceType == "CALCULATION" && finding.Status == domain.FindingOpen {
			return true
		}
	}
	return false
}
func calculationMap(snapshot *domain.CalculationSnapshot) map[string]domain.LineCalculation {
	result := map[string]domain.LineCalculation{}
	for _, line := range snapshot.Lines {
		result[line.LineID] = line
	}
	return result
}
func openCalculationSet(session *domain.RiggingSession) map[string]FindingReference {
	result := map[string]FindingReference{}
	for _, f := range session.Findings {
		if f.SourceType == "CALCULATION" && f.Status == domain.FindingOpen {
			key := f.LineID + "\x00" + f.RuleCode
			result[key] = FindingReference{LineID: f.LineID, RuleCode: f.RuleCode}
		}
	}
	return result
}
func compareCalculationFindings(before, after *domain.RiggingSession) ([]FindingReference, []FindingReference, []FindingReference) {
	a, b := openCalculationSet(before), openCalculationSet(after)
	closed, kept, added := []FindingReference{}, []FindingReference{}, []FindingReference{}
	for key, value := range a {
		if _, ok := b[key]; ok {
			kept = append(kept, value)
		} else {
			closed = append(closed, value)
		}
	}
	for key, value := range b {
		if _, ok := a[key]; !ok {
			added = append(added, value)
		}
	}
	sortRefs := func(values []FindingReference) {
		sort.Slice(values, func(i, j int) bool {
			if values[i].LineID == values[j].LineID {
				return values[i].RuleCode < values[j].RuleCode
			}
			return values[i].LineID < values[j].LineID
		})
	}
	sortRefs(closed)
	sortRefs(kept)
	sortRefs(added)
	return closed, kept, added
}
func indexedRevisionError(err error, index int) error {
	var de *domain.DomainError
	if errors.As(err, &de) {
		field := fmt.Sprintf("revisions[%d]", index)
		if de.Field != "" {
			field += "." + de.Field
		}
		return domain.NewError(de.Code, field, de.Msg)
	}
	return err
}
