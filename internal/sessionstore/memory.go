package sessionstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const defaultMaxAgeSec = 24 * 60 * 60 // 24 hours

type memoryStore struct {
	mu     sync.RWMutex
	sessions map[string]struct {
		username string
		role     string
		expires  time.Time
	}
}

// NewMemoryStore возвращает хранилище сессий в памяти (по умолчанию для одного инстанса).
func NewMemoryStore() Store {
	return &memoryStore{sessions: make(map[string]struct {
		username string
		role     string
		expires  time.Time
	})}
}

func (m *memoryStore) CreateSession(ctx context.Context, username, role string) (sessionID string, err error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	m.mu.Lock()
	m.sessions[id] = struct {
		username string
		role     string
		expires  time.Time
	}{username, role, time.Now().Add(time.Duration(defaultMaxAgeSec) * time.Second)}
	m.mu.Unlock()
	return id, nil
}

func (m *memoryStore) GetSession(ctx context.Context, sessionID string) (username, role string, ok bool) {
	if sessionID == "" {
		return "", "", false
	}
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok || time.Now().After(s.expires) {
		if ok {
			m.mu.Lock()
			delete(m.sessions, sessionID)
			m.mu.Unlock()
		}
		return "", "", false
	}
	return s.username, s.role, true
}

func (m *memoryStore) DeleteSession(ctx context.Context, sessionID string) {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}
