package storage

import (
	"context"
	"database/sql"
	"fmt"
)

const currentSchemaVersion = 2

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_meta (schema_version INTEGER NOT NULL)"); err != nil {
		return err
	}
	var version int
	err := s.db.QueryRowContext(ctx, "SELECT schema_version FROM schema_meta LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows {
		if _, err = s.db.ExecContext(ctx, "INSERT INTO schema_meta(schema_version) VALUES (0)"); err != nil {
			return err
		}
		version = 0
	} else if err != nil {
		return err
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("数据库 schemaVersion %d 高于程序支持版本 %d", version, currentSchemaVersion)
	}
	if version == 0 {
		return s.migrateV1(ctx)
	}
	if version == 1 {
		return s.migrateV2(ctx)
	}
	return nil
}
func (s *Store) migrateV1(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE sessions (id TEXT PRIMARY KEY,title TEXT NOT NULL,venue TEXT NOT NULL,performance_at TEXT NOT NULL,operator_id TEXT NOT NULL,rule_set_version TEXT NOT NULL,status TEXT NOT NULL,version INTEGER NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,payload BLOB NOT NULL)`,
		`CREATE TABLE rigging_lines (id TEXT PRIMARY KEY,session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,code TEXT NOT NULL,rated_load_gram INTEGER NOT NULL,span_millimeter INTEGER NOT NULL,max_moment INTEGER NOT NULL,total_load_gram INTEGER NOT NULL,utilization_ppm INTEGER NOT NULL,calculated_moment INTEGER NOT NULL,safety_margin_ppm INTEGER NOT NULL,UNIQUE(session_id,code))`,
		`CREATE TABLE rigging_points (id TEXT PRIMARY KEY,session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,line_id TEXT NOT NULL REFERENCES rigging_lines(id) ON DELETE CASCADE,code TEXT NOT NULL,hoist_rated_load_gram INTEGER NOT NULL,position_millimeter INTEGER NOT NULL,UNIQUE(session_id,code))`,
		`CREATE TABLE suspended_loads (id TEXT PRIMARY KEY,session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,line_id TEXT NOT NULL REFERENCES rigging_lines(id) ON DELETE CASCADE,point_id TEXT NOT NULL REFERENCES rigging_points(id),component_code TEXT NOT NULL,description TEXT NOT NULL,weight_gram INTEGER NOT NULL,position_millimeter INTEGER NOT NULL,quantity INTEGER NOT NULL,submitted_by TEXT NOT NULL,created_at TEXT NOT NULL,UNIQUE(session_id,component_code))`,
		`CREATE TABLE line_checks (id TEXT PRIMARY KEY,session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,line_id TEXT NOT NULL REFERENCES rigging_lines(id) ON DELETE CASCADE,kind TEXT NOT NULL,passed INTEGER NOT NULL,measurement TEXT NOT NULL,evidence TEXT NOT NULL,inspector_id TEXT NOT NULL,checked_at TEXT NOT NULL,UNIQUE(line_id,kind))`,
		`CREATE TABLE safety_findings (id TEXT PRIMARY KEY,session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,line_id TEXT NOT NULL,source_type TEXT NOT NULL,severity TEXT NOT NULL,rule_code TEXT NOT NULL,description TEXT NOT NULL,status TEXT NOT NULL,assignee_id TEXT NOT NULL,remediation_note TEXT NOT NULL,verified_by TEXT NOT NULL,closed_at TEXT)`,
		`CREATE TABLE frozen_manifests (session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,frozen_version INTEGER NOT NULL,digest TEXT NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(session_id,frozen_version))`,
		`CREATE TABLE release_certificates (id TEXT PRIMARY KEY,session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,frozen_version INTEGER NOT NULL,manifest_digest TEXT NOT NULL,rule_set_version TEXT NOT NULL,approved_by TEXT NOT NULL,issued_at TEXT NOT NULL,verification_status TEXT NOT NULL,UNIQUE(session_id,frozen_version))`,
		`CREATE TABLE audit_events (sequence INTEGER PRIMARY KEY AUTOINCREMENT,id TEXT NOT NULL UNIQUE,session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,event_type TEXT NOT NULL,actor_id TEXT NOT NULL,detail TEXT NOT NULL,created_at TEXT NOT NULL)`,
		`CREATE TABLE idempotency_records (key TEXT NOT NULL,operation TEXT NOT NULL,session_id TEXT NOT NULL,response BLOB NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(key,operation))`,
		`CREATE INDEX idx_sessions_updated ON sessions(updated_at DESC)`,
		`CREATE INDEX idx_sessions_queue ON sessions(performance_at,updated_at DESC,id)`,
		`CREATE INDEX idx_sessions_status_performance ON sessions(status,performance_at,updated_at DESC,id)`,
		`CREATE INDEX idx_audit_session ON audit_events(session_id,sequence DESC)`,
	}
	for _, statement := range statements {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE schema_meta SET schema_version = ?", currentSchemaVersion); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) migrateV2(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{`CREATE INDEX IF NOT EXISTS idx_sessions_queue ON sessions(performance_at,updated_at DESC,id)`, `CREATE INDEX IF NOT EXISTS idx_sessions_status_performance ON sessions(status,performance_at,updated_at DESC,id)`} {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE schema_meta SET schema_version = ?", currentSchemaVersion); err != nil {
		return err
	}
	return tx.Commit()
}
