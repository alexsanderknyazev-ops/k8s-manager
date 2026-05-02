package audit

import (
	"context"
	"sync"
	"time"
)

const maxEntries = 500

type Entry struct {
	Time      time.Time `json:"time"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	Username  string    `json:"username,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
}

// PersistentStore — хранилище аудита (например Postgres). При установке список записей берётся из него.
type PersistentStore interface {
	Append(ctx context.Context, method, path string, status int, username, requestID string)
	Get(ctx context.Context, limit int) []Entry
}

var (
	mu              sync.RWMutex
	entries         []Entry
	persistentStore PersistentStore
)

func SetPersistentStore(s PersistentStore) { persistentStore = s }

func appendMemory(method, path string, status int, username, requestID string) {
	mu.Lock()
	defer mu.Unlock()
	e := Entry{
		Time:      time.Now(),
		Method:    method,
		Path:      path,
		Status:    status,
		Username:  username,
		RequestID: requestID,
	}
	entries = append(entries, e)
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
}

func getFromMemory(limit int) []Entry {
	mu.RLock()
	defer mu.RUnlock()
	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}
	start := len(entries) - limit
	if start < 0 {
		start = 0
	}
	result := make([]Entry, limit)
	copy(result, entries[start:])
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func Append(ctx context.Context, method, path string, status int, username, requestID string) {
	appendMemory(method, path, status, username, requestID)
	if persistentStore != nil {
		persistentStore.Append(ctx, method, path, status, username, requestID)
	}
}

func Get(ctx context.Context, limit int) []Entry {
	if persistentStore != nil {
		if out := persistentStore.Get(ctx, limit); len(out) > 0 {
			return out
		}
	}
	return getFromMemory(limit)
}
