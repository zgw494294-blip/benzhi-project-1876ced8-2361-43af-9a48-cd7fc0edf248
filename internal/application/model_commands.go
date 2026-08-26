package application

import (
	"context"
	"encoding/json"
	"rigging-readiness-desk/internal/domain"
)

func (s *Service) AddLine(ctx context.Context, id string, cmd AddLineCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd.VersionCommand, "add-line", "LINE_ADDED", "登记吊杆 "+cmd.Code, func(session *domain.RiggingSession) error {
		return session.AddLine(domain.RiggingLine{ID: s.newID(), Code: cmd.Code, RatedLoadGram: cmd.RatedLoadGram, SpanMillimeter: cmd.SpanMillimeter, MaxMomentNewtonMillimeter: cmd.MaxMomentNewtonMillimeter})
	})
}
func (s *Service) AddPoint(ctx context.Context, id string, cmd AddPointCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd.VersionCommand, "add-point", "POINT_ADDED", "登记吊点 "+cmd.Code, func(session *domain.RiggingSession) error {
		return session.AddPoint(domain.RiggingPoint{ID: s.newID(), LineID: cmd.LineID, Code: cmd.Code, HoistRatedLoadGram: cmd.HoistRatedLoadGram, PositionMillimeter: cmd.PositionMillimeter})
	})
}
func (s *Service) AddLoad(ctx context.Context, id string, cmd AddLoadCommand) (*domain.RiggingSession, error) {
	cmd.ActorID = defaultActor(cmd.ActorID, cmd.SubmittedBy)
	return s.mutation(ctx, id, cmd.VersionCommand, "add-load", "LOAD_ADDED", "登记悬挂构件 "+cmd.ComponentCode, func(session *domain.RiggingSession) error {
		return session.AddLoad(domain.SuspendedLoad{ID: s.newID(), LineID: cmd.LineID, PointID: cmd.PointID, ComponentCode: cmd.ComponentCode, Description: cmd.Description, WeightGram: cmd.WeightGram, PositionMillimeter: cmd.PositionMillimeter, Quantity: cmd.Quantity, SubmittedBy: cmd.SubmittedBy, CreatedAt: s.now().UTC()})
	})
}

type BatchLoadItem struct {
	Index         int    `json:"index"`
	ID            string `json:"id"`
	ComponentCode string `json:"componentCode"`
}
type BatchLoadResult struct {
	SessionID    string          `json:"sessionId"`
	Version      int64           `json:"version"`
	SuccessCount int             `json:"successCount"`
	Loads        []BatchLoadItem `json:"loads"`
}

func (s *Service) AddLoads(ctx context.Context, id string, cmd AddLoadsCommand) (*BatchLoadResult, error) {
	const operation = "add-loads-batch"
	if cmd.ActorID == "" && len(cmd.Loads) > 0 {
		cmd.ActorID = cmd.Loads[0].SubmittedBy
	}
	unlock, err := s.locks.Lock(ctx, id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if cmd.IdempotencyKey != "" {
		record, err := s.repo.GetIdempotency(ctx, cmd.IdempotencyKey, operation)
		if err == nil {
			if record.SessionID != id {
				return nil, domain.NewError(domain.ErrConflict, "Idempotency-Key", "幂等键已用于其他作业")
			}
			var result BatchLoadResult
			if err = json.Unmarshal(record.Response, &result); err != nil {
				return nil, err
			}
			return &result, nil
		}
		if !IsRepositoryNotFound(err) {
			return nil, err
		}
	}
	session, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.ExpectedVersion <= 0 {
		return nil, domain.NewError(domain.ErrValidation, "expectedVersion", "必须大于零")
	}
	if session.Version != cmd.ExpectedVersion {
		return nil, domain.NewError(domain.ErrConflict, "expectedVersion", "版本已变化，请刷新后重试")
	}
	loads := make([]domain.SuspendedLoad, 0, len(cmd.Loads))
	items := make([]BatchLoadItem, 0, len(cmd.Loads))
	for i, input := range cmd.Loads {
		loadID := s.newID()
		loads = append(loads, domain.SuspendedLoad{ID: loadID, LineID: input.LineID, PointID: input.PointID, ComponentCode: input.ComponentCode, Description: input.Description, WeightGram: input.WeightGram, PositionMillimeter: input.PositionMillimeter, Quantity: input.Quantity, SubmittedBy: input.SubmittedBy, CreatedAt: s.now().UTC()})
		items = append(items, BatchLoadItem{Index: i, ID: loadID, ComponentCode: input.ComponentCode})
	}
	if err = session.AddLoads(loads); err != nil {
		return nil, err
	}
	expected := session.Version
	session.Version++
	session.UpdatedAt = s.now().UTC()
	result := &BatchLoadResult{SessionID: id, Version: session.Version, SuccessCount: len(items), Loads: items}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	event := AuditEvent{ID: s.newID(), SessionID: id, Type: "LOAD_BATCH_ADDED", ActorID: cmd.ActorID, Detail: "批量登记悬挂构件", CreatedAt: s.now().UTC()}
	var idem *IdempotencyRecord
	if cmd.IdempotencyKey != "" {
		idem = &IdempotencyRecord{Key: cmd.IdempotencyKey, Operation: operation, SessionID: id, Response: data, CreatedAt: s.now().UTC()}
	}
	if err = s.repo.Save(ctx, session, expected, event, idem); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Service) FinalizeModel(ctx context.Context, id string, cmd VersionCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd, "finalize-model", "MODEL_FINALIZED", "完成载荷建模", func(session *domain.RiggingSession) error { return session.FinalizeModel() })
}
func (s *Service) Calculate(ctx context.Context, id string, cmd VersionCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd, "calculate", "LOAD_CALCULATED", "执行载荷与力矩计算", func(session *domain.RiggingSession) error { _, err := session.Calculate(s.now(), s.newID); return err })
}
func (s *Service) ReviseLoad(ctx context.Context, id string, cmd ReviseLoadCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd.VersionCommand, "revise-load", "LOAD_REVISED", "整改中修订载荷", func(session *domain.RiggingSession) error {
		return session.ReviseLoad(cmd.LoadID, cmd.WeightGram, cmd.PositionMillimeter)
	})
}
