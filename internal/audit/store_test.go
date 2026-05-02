package audit

import (
	"context"
	"testing"
	"time"
)

func TestAppendGet_inMemory(t *testing.T) {
	// Сбрасываем persistentStore, чтобы тест использовал только память
	old := persistentStore
	SetPersistentStore(nil)
	defer SetPersistentStore(old)

	ctx := context.Background()
	Append(ctx, "GET", "/api/pods", 200, "admin", "req-1")
	Append(ctx, "POST", "/api/login", 200, "", "req-2")

	got := Get(ctx, 10)
	if len(got) < 2 {
		t.Fatalf("Get: want at least 2 entries, got %d", len(got))
	}
	// Новые записи первыми (reverse order)
	if got[0].Method != "POST" || got[0].Path != "/api/login" {
		t.Errorf("first entry: want POST /api/login, got %s %s", got[0].Method, got[0].Path)
	}
	if got[1].Method != "GET" || got[1].Path != "/api/pods" {
		t.Errorf("second entry: want GET /api/pods, got %s %s", got[1].Method, got[1].Path)
	}
	if got[0].Status != 200 || got[1].Status != 200 {
		t.Errorf("status: want 200, got %d %d", got[0].Status, got[1].Status)
	}
}

func TestGet_withPersistentStore_returnsStoreData(t *testing.T) {
	old := persistentStore
	defer SetPersistentStore(old)

	mock := &mockPersistentStore{
		entries: []Entry{
			{Time: time.Now(), Method: "GET", Path: "/api/health", Status: 200, Username: "", RequestID: "x"},
		},
	}
	SetPersistentStore(mock)

	ctx := context.Background()
	got := Get(ctx, 5)
	if len(got) != 1 {
		t.Fatalf("Get with persistent store: want 1 entry, got %d", len(got))
	}
	if got[0].Path != "/api/health" || got[0].Status != 200 {
		t.Errorf("entry: want path=/api/health status=200, got path=%s status=%d", got[0].Path, got[0].Status)
	}
}

func TestGet_withPersistentStoreEmpty_fallsBackToMemory(t *testing.T) {
	old := persistentStore
	defer SetPersistentStore(old)

	// Mock возвращает пустой слайс — тогда берём из памяти
	SetPersistentStore(&mockPersistentStore{entries: nil})

	ctx := context.Background()
	Append(ctx, "GET", "/fallback", 200, "u", "r")
	got := Get(ctx, 10)
	// PersistentStore.Get возвращает nil/empty, поэтому используется memory; там должна быть наша запись
	if len(got) < 1 {
		t.Fatalf("Get (fallback to memory): want at least 1 entry, got %d", len(got))
	}
	found := false
	for _, e := range got {
		if e.Path == "/fallback" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Get: expected /fallback entry from memory fallback")
	}
}

type mockPersistentStore struct {
	entries []Entry
}

func (m *mockPersistentStore) Append(ctx context.Context, method, path string, status int, username, requestID string) {}
func (m *mockPersistentStore) Get(ctx context.Context, limit int) []Entry {
	return m.entries
}
