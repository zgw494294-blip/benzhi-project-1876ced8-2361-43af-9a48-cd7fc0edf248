package storage

import "context"

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
