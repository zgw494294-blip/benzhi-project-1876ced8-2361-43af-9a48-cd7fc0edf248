package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"rigging-readiness-desk/internal/domain"
)

type storedSnapshot struct {
	id      string
	status  string
	version int64
	payload []byte
}

func (s *Store) verifyIntegrity(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id,status,version,payload FROM sessions ORDER BY id`)
	if err != nil {
		return err
	}
	snapshots := []storedSnapshot{}
	for rows.Next() {
		var item storedSnapshot
		if err = rows.Scan(&item.id, &item.status, &item.version, &item.payload); err != nil {
			rows.Close()
			return err
		}
		snapshots = append(snapshots, item)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, item := range snapshots {
		var session domain.RiggingSession
		if err = json.Unmarshal(item.payload, &session); err != nil {
			return fmt.Errorf("作业 %s 快照无法解析: %w", item.id, err)
		}
		if session.ID != item.id || string(session.Status) != item.status || session.Version != item.version {
			return fmt.Errorf("作业 %s 快照与索引字段不一致", item.id)
		}
		if err = s.verifyChildCount(ctx, item.id, "rigging_lines", len(session.Lines)); err != nil {
			return err
		}
		if err = s.verifyChildCount(ctx, item.id, "rigging_points", len(session.Points)); err != nil {
			return err
		}
		if err = s.verifyChildCount(ctx, item.id, "suspended_loads", len(session.Loads)); err != nil {
			return err
		}
		if err = s.verifyChildCount(ctx, item.id, "line_checks", len(session.Checks)); err != nil {
			return err
		}
		if err = s.verifyChildCount(ctx, item.id, "safety_findings", len(session.Findings)); err != nil {
			return err
		}
		manifestCount := 0
		if session.Frozen != nil {
			manifestCount = 1
			if session.Review == nil || domain.DigestManifest(session.Frozen, session.RuleSetVersion, session.Review.ReviewerID) != session.Frozen.Digest {
				return fmt.Errorf("作业 %s 的冻结清单摘要不一致", item.id)
			}
		}
		if err = s.verifyChildCount(ctx, item.id, "frozen_manifests", manifestCount); err != nil {
			return err
		}
		certificateCount := 0
		if session.Certificate != nil {
			certificateCount = 1
			if !session.VerifyCertificate() {
				return fmt.Errorf("作业 %s 的启用凭据无法通过摘要校验", item.id)
			}
		}
		if err = s.verifyChildCount(ctx, item.id, "release_certificates", certificateCount); err != nil {
			return err
		}
		var auditCount int
		if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE session_id=?`, item.id).Scan(&auditCount); err != nil {
			return err
		}
		if auditCount == 0 {
			return fmt.Errorf("作业 %s 缺少追加式审计事件", item.id)
		}
	}
	return nil
}

func (s *Store) verifyChildCount(ctx context.Context, sessionID, table string, expected int) error {
	allowed := map[string]bool{"rigging_lines": true, "rigging_points": true, "suspended_loads": true, "line_checks": true, "safety_findings": true, "frozen_manifests": true, "release_certificates": true}
	if !allowed[table] {
		return fmt.Errorf("不支持的一致性检查表 %s", table)
	}
	var actual int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE session_id=?", sessionID).Scan(&actual); err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("作业 %s 的 %s 行数不一致：快照 %d，规范化表 %d", sessionID, table, expected, actual)
	}
	return nil
}
