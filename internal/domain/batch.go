package domain

import (
	"errors"
	"fmt"
	"strings"
)

const MaxLoadBatchSize = 100

func (s *RiggingSession) AddLoads(loads []SuspendedLoad) error {
	if err := s.requireStatus(StatusBaselined); err != nil {
		return err
	}
	if len(loads) == 0 {
		return NewError(ErrValidation, "loads", "批次至少包含一条悬挂构件")
	}
	if len(loads) > MaxLoadBatchSize {
		return NewError(ErrValidation, "loads", fmt.Sprintf("批次不得超过 %d 条", MaxLoadBatchSize))
	}
	codes := map[string]struct{}{}
	rows := map[string]struct{}{}
	working := *s
	working.Loads = append([]SuspendedLoad(nil), s.Loads...)
	for i, load := range loads {
		code := strings.ToLower(strings.TrimSpace(load.ComponentCode))
		row := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%d\x00%d", load.LineID, load.PointID, code, strings.TrimSpace(load.Description), load.WeightGram, load.PositionMillimeter, load.Quantity)
		if _, exists := rows[row]; exists {
			return NewError(ErrValidation, fmt.Sprintf("loads[%d]", i), "批次内存在重复行")
		}
		rows[row] = struct{}{}
		if _, exists := codes[code]; exists && code != "" {
			return NewError(ErrValidation, fmt.Sprintf("loads[%d].componentCode", i), "批次内构件编号重复")
		}
		codes[code] = struct{}{}
		if err := working.AddLoad(load); err != nil {
			var de *DomainError
			if errors.As(err, &de) {
				field := fmt.Sprintf("loads[%d]", i)
				if de.Field != "" {
					field += "." + normalizedLoadField(de.Field)
				}
				return NewError(de.Code, field, de.Msg)
			}
			return err
		}
	}
	s.Loads = working.Loads
	return nil
}

func normalizedLoadField(field string) string {
	if field == "load" {
		return "componentCode"
	}
	return field
}
