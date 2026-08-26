package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"rigging-readiness-desk/internal/domain"
	"time"
)

func replaceChildren(ctx context.Context, tx *sql.Tx, s *domain.RiggingSession) error {
	for _, table := range []string{"release_certificates", "frozen_manifests", "safety_findings", "line_checks", "suspended_loads", "rigging_points", "rigging_lines"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE session_id = ?", s.ID); err != nil {
			return err
		}
	}
	for _, line := range s.Lines {
		if _, err := tx.ExecContext(ctx, `INSERT INTO rigging_lines(id,session_id,code,rated_load_gram,span_millimeter,max_moment,total_load_gram,utilization_ppm,calculated_moment,safety_margin_ppm) VALUES(?,?,?,?,?,?,?,?,?,?)`, line.ID, s.ID, line.Code, line.RatedLoadGram, line.SpanMillimeter, line.MaxMomentNewtonMillimeter, line.TotalLoadGram, line.UtilizationPPM, line.CalculatedMomentNewtonMillimeter, line.SafetyMarginPPM); err != nil {
			return err
		}
	}
	for _, point := range s.Points {
		if _, err := tx.ExecContext(ctx, `INSERT INTO rigging_points(id,session_id,line_id,code,hoist_rated_load_gram,position_millimeter) VALUES(?,?,?,?,?,?)`, point.ID, s.ID, point.LineID, point.Code, point.HoistRatedLoadGram, point.PositionMillimeter); err != nil {
			return err
		}
	}
	for _, load := range s.Loads {
		if _, err := tx.ExecContext(ctx, `INSERT INTO suspended_loads(id,session_id,line_id,point_id,component_code,description,weight_gram,position_millimeter,quantity,submitted_by,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, load.ID, s.ID, load.LineID, load.PointID, load.ComponentCode, load.Description, load.WeightGram, load.PositionMillimeter, load.Quantity, load.SubmittedBy, load.CreatedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	for _, check := range s.Checks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO line_checks(id,session_id,line_id,kind,passed,measurement,evidence,inspector_id,checked_at) VALUES(?,?,?,?,?,?,?,?,?)`, check.ID, s.ID, check.LineID, string(check.Kind), check.Passed, check.Measurement, check.Evidence, check.InspectorID, check.CheckedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	for _, finding := range s.Findings {
		var closed any
		if finding.ClosedAt != nil {
			closed = finding.ClosedAt.Format(time.RFC3339Nano)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO safety_findings(id,session_id,line_id,source_type,severity,rule_code,description,status,assignee_id,remediation_note,verified_by,closed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, finding.ID, s.ID, finding.LineID, finding.SourceType, finding.Severity, finding.RuleCode, finding.Description, string(finding.Status), finding.AssigneeID, finding.RemediationNote, finding.VerifiedBy, closed); err != nil {
			return err
		}
	}
	if s.Frozen != nil {
		payload, _ := json.Marshal(s.Frozen)
		if _, err := tx.ExecContext(ctx, `INSERT INTO frozen_manifests(session_id,frozen_version,digest,payload) VALUES(?,?,?,?)`, s.ID, s.Frozen.Version, s.Frozen.Digest, payload); err != nil {
			return err
		}
	}
	if c := s.Certificate; c != nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO release_certificates(id,session_id,frozen_version,manifest_digest,rule_set_version,approved_by,issued_at,verification_status) VALUES(?,?,?,?,?,?,?,?)`, c.ID, c.SessionID, c.FrozenVersion, c.ManifestDigest, c.RuleSetVersion, c.ApprovedBy, c.IssuedAt.Format(time.RFC3339Nano), c.VerificationStatus); err != nil {
			return err
		}
	}
	return nil
}
