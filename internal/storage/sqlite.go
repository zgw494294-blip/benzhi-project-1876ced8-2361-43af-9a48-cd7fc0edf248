package storage

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"path/filepath"
	"rigging-readiness-desk/internal/domain"
	"sync"
)

type Store struct {
	db       *sql.DB
	cacheMu  sync.RWMutex
	sessions map[string]*domain.RiggingSession
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("数据路径不能为空")
	}
	dsn := path
	if path != ":memory:" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		dsn = "file:" + url.PathEscape(absolute)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, sessions: map[string]*domain.RiggingSession{}}
	ctx := context.Background()
	if _, err = db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, err
	}
	if err = store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err = store.verifyIntegrity(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}
func (s *Store) Close() error { return s.db.Close() }
