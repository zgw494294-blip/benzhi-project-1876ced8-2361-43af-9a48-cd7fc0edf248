package storage

import "rigging-readiness-desk/internal/domain"

func (s *Store) cachedSession(id string) *domain.RiggingSession {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return s.sessions[id]
}

func (s *Store) rememberSession(session *domain.RiggingSession) {
	s.cacheMu.Lock()
	s.sessions[session.ID] = session
	s.cacheMu.Unlock()
}
