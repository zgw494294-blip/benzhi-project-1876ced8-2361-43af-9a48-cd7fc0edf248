package application

import "sync"

type keyedLocks struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}
type lockEntry struct {
	mu    sync.Mutex
	users int
}

func newKeyedLocks() *keyedLocks { return &keyedLocks{locks: map[string]*lockEntry{}} }
func (k *keyedLocks) Lock(key string) func() {
	k.mu.Lock()
	entry := k.locks[key]
	if entry == nil {
		entry = &lockEntry{}
		k.locks[key] = entry
	}
	entry.users++
	k.mu.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		k.mu.Lock()
		entry.users--
		if entry.users == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}
