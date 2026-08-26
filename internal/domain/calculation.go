package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const ppm = int64(1_000_000)

type calculationInput struct {
	Rule   string                 `json:"rule"`
	Lines  []calculationLineInput `json:"lines"`
	Points []RiggingPoint         `json:"points"`
	Loads  []SuspendedLoad        `json:"loads"`
}

type calculationLineInput struct {
	ID                        string `json:"id"`
	Code                      string `json:"code"`
	RatedLoadGram             int64  `json:"ratedLoadGram"`
	SpanMillimeter            int64  `json:"spanMillimeter"`
	MaxMomentNewtonMillimeter int64  `json:"maxMomentNewtonMillimeter"`
}

func (s *RiggingSession) Calculate(now time.Time, idFactory func() string) (*CalculationSnapshot, error) {
	if err := s.requireStatus(StatusModeled, StatusInspected); err != nil {
		return nil, err
	}
	lines := make([]calculationLineInput, 0, len(s.Lines))
	for _, line := range s.Lines {
		lines = append(lines, calculationLineInput{ID: line.ID, Code: line.Code, RatedLoadGram: line.RatedLoadGram, SpanMillimeter: line.SpanMillimeter, MaxMomentNewtonMillimeter: line.MaxMomentNewtonMillimeter})
	}
	loads := append([]SuspendedLoad(nil), s.Loads...)
	points := append([]RiggingPoint(nil), s.Points...)
	sort.Slice(lines, func(i, j int) bool { return lines[i].Code < lines[j].Code })
	sort.Slice(points, func(i, j int) bool {
		if points[i].LineID == points[j].LineID {
			return points[i].Code < points[j].Code
		}
		return points[i].LineID < points[j].LineID
	})
	sort.Slice(loads, func(i, j int) bool {
		if loads[i].LineID == loads[j].LineID {
			return loads[i].ComponentCode < loads[j].ComponentCode
		}
		return loads[i].LineID < loads[j].LineID
	})
	b, _ := json.Marshal(calculationInput{Rule: s.RuleSetVersion, Lines: lines, Points: points, Loads: loads})
	sum := sha256.Sum256(b)
	snap := &CalculationSnapshot{RuleSetVersion: s.RuleSetVersion, InputDigest: hex.EncodeToString(sum[:]), CalculatedAt: now.UTC(), Lines: []LineCalculation{}}
	for i := range s.Lines {
		line := &s.Lines[i]
		var total, moment, hoistCapacity int64
		for _, point := range s.Points {
			if point.LineID == line.ID {
				hoistCapacity += point.HoistRatedLoadGram
			}
		}
		effectiveRated := line.RatedLoadGram
		if hoistCapacity < effectiveRated {
			effectiveRated = hoistCapacity
		}
		if effectiveRated <= 0 {
			return nil, NewError(ErrValidation, "points", "吊杆缺少有效提升机额定载荷")
		}
		for _, load := range s.Loads {
			if load.LineID != line.ID {
				continue
			}
			weight := load.WeightGram * load.Quantity
			total += weight
			offset := load.PositionMillimeter - line.SpanMillimeter/2
			if offset < 0 {
				offset = -offset
			}
			moment += weight * offset * 981 / 100_000
		}
		util := total * ppm / effectiveRated
		momentUtil := moment * ppm / line.MaxMomentNewtonMillimeter
		governing := util
		if momentUtil > governing {
			governing = momentUtil
		}
		margin := ppm - governing
		line.TotalLoadGram = total
		line.UtilizationPPM = util
		line.CalculatedMomentNewtonMillimeter = moment
		line.SafetyMarginPPM = margin
		result := LineCalculation{LineID: line.ID, LineCode: line.Code, TotalLoadGram: total, EffectiveRatedLoadGram: effectiveRated, UtilizationPPM: util, MomentNewtonMillimeter: moment, MomentUtilizationPPM: momentUtil, SafetyMarginPPM: margin, Passed: util <= ppm && momentUtil <= ppm}
		if util > ppm {
			reason := fmt.Sprintf("静载利用率 %d ppm 超过 1000000 ppm", util)
			result.Reasons = append(result.Reasons, reason)
			s.upsertCalculationFinding(line.ID, "RATED_LOAD_EXCEEDED", reason, idFactory)
		} else {
			s.closeCalculationFinding(line.ID, "RATED_LOAD_EXCEEDED", now)
		}
		if momentUtil > ppm {
			reason := fmt.Sprintf("偏心力矩利用率 %d ppm 超过 1000000 ppm", momentUtil)
			result.Reasons = append(result.Reasons, reason)
			s.upsertCalculationFinding(line.ID, "MOMENT_EXCEEDED", reason, idFactory)
		} else {
			s.closeCalculationFinding(line.ID, "MOMENT_EXCEEDED", now)
		}
		if result.Passed {
			result.Reasons = []string{fmt.Sprintf("静载低于有效额定边界 %d 克，偏心力矩在规则边界内", effectiveRated)}
		}
		snap.Lines = append(snap.Lines, result)
		s.closeReviewCalculationFindings(line.ID, now)
	}
	s.Calculation = snap
	return snap, nil
}
func (s *RiggingSession) upsertCalculationFinding(lineID, rule, description string, idFactory func() string) {
	for i := range s.Findings {
		if s.Findings[i].LineID == lineID && s.Findings[i].RuleCode == rule && s.Findings[i].Status == FindingOpen {
			s.Findings[i].Description = description
			return
		}
	}
	s.Findings = append(s.Findings, SafetyFinding{ID: idFactory(), SessionID: s.ID, LineID: lineID, SourceType: "CALCULATION", Severity: "BLOCKING", RuleCode: rule, Description: description, Status: FindingOpen})
}
func (s *RiggingSession) closeCalculationFinding(lineID, rule string, now time.Time) {
	for i := range s.Findings {
		f := &s.Findings[i]
		if f.LineID == lineID && f.SourceType == "CALCULATION" && f.RuleCode == rule && f.Status == FindingOpen {
			f.Status = FindingClosed
			f.VerifiedBy = "SYSTEM_RECALCULATION"
			closed := now.UTC()
			f.ClosedAt = &closed
		}
	}
}
