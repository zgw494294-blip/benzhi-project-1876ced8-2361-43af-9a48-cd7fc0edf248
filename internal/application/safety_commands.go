package application

import (
	"context"
	"rigging-readiness-desk/internal/domain"
	"strings"
)

func (s *Service) RecordCheck(ctx context.Context, id string, cmd CheckCommand) (*domain.RiggingSession, error) {
	cmd.ActorID = defaultActor(cmd.ActorID, cmd.InspectorID)
	return s.mutation(ctx, id, cmd.VersionCommand, "record-check:"+cmd.LineID+":"+cmd.Kind, "CHECK_RECORDED", "记录现场检查 "+cmd.Kind, func(session *domain.RiggingSession) error {
		return session.RecordCheck(domain.LineCheck{ID: s.newID(), LineID: cmd.LineID, Kind: domain.CheckKind(cmd.Kind), Passed: cmd.Passed, Measurement: cmd.Measurement, Evidence: cmd.Evidence, InspectorID: cmd.InspectorID}, s.now(), s.newID)
	})
}
func (s *Service) CompleteInspection(ctx context.Context, id string, cmd VersionCommand) (*domain.RiggingSession, error) {
	return s.mutation(ctx, id, cmd, "complete-inspection", "INSPECTION_COMPLETED", "确认检查覆盖完整", func(session *domain.RiggingSession) error { return session.CompleteInspection() })
}
func (s *Service) AssignRemediation(ctx context.Context, id string, cmd RemediationCommand) (*domain.RiggingSession, error) {
	cmd.ActorID = defaultActor(cmd.ActorID, cmd.AssigneeID)
	return s.mutation(ctx, id, cmd.VersionCommand, "assign-remediation:"+cmd.FindingID, "REMEDIATION_ASSIGNED", "登记整改责任与说明", func(session *domain.RiggingSession) error {
		for _, finding := range session.Findings {
			if finding.ID == cmd.FindingID && finding.SourceType == "REVIEW" && cmd.ActorID != session.OperatorID {
				return domain.NewError(domain.ErrForbidden, "actorId", "复核退回阻断项必须由作业经办人登记整改证据")
			}
		}
		return session.AssignRemediation(cmd.FindingID, cmd.AssigneeID, cmd.Note)
	})
}
func (s *Service) Review(ctx context.Context, id string, cmd ReviewCommand) (*domain.RiggingSession, error) {
	cmd.ActorID = cmd.ReviewerID
	decision := strings.ToUpper(strings.TrimSpace(cmd.Decision))
	if decision != "APPROVE" && decision != "RETURN" {
		return nil, domain.NewError(domain.ErrValidation, "decision", "决定必须为 APPROVE 或 RETURN")
	}
	eventType := "SAFETY_APPROVED"
	detail := "独立复核批准"
	if decision == "RETURN" {
		eventType = "REVIEW_RETURNED"
		detail = "复核退回：" + strings.TrimSpace(cmd.Reason) + "；类别 " + strings.TrimSpace(cmd.Category) + "；吊杆 " + strings.Join(cmd.AffectedLineIDs, ",")
	}
	return s.mutation(ctx, id, cmd.VersionCommand, "review:"+decision, eventType, detail, func(session *domain.RiggingSession) error {
		if decision == "RETURN" {
			return session.ReturnReview(cmd.ReviewerID, cmd.Reason, cmd.Category, cmd.AffectedLineIDs, session.Version, s.now(), s.newID)
		}
		confirmation, err := BuildReviewConfirmation(session)
		if err != nil {
			return err
		}
		provided := cmd.ConfirmationID
		if provided == "" {
			provided = cmd.ReviewConfirmationID
		}
		if cmd.ConfirmationID != "" && cmd.ReviewConfirmationID != "" && cmd.ConfirmationID != cmd.ReviewConfirmationID {
			return domain.NewError(domain.ErrValidation, "reviewConfirmationId", "两个复核确认标识不一致")
		}
		return session.ApproveConfirmed(cmd.ReviewerID, cmd.Reason, provided, confirmation.ID, session.Version, s.now())
	})
}
