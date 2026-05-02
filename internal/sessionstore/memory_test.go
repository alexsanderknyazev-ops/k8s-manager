package sessionstore

import (
	"context"
	"regexp"
	"testing"
)

func TestMemoryStore_CreateSession_returnsHexID(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()
	id, err := m.CreateSession(ctx, "alice", "admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if len(id) != 32 {
		t.Errorf("session ID length: want 32, got %d", len(id))
	}
	if ok := regexp.MustCompile(`^[a-f0-9]+$`).MatchString(id); !ok {
		t.Errorf("session ID must be hex, got %q", id)
	}
}

func TestMemoryStore_CreateGetSession_roundtrip(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()
	id, err := m.CreateSession(ctx, "bob", "viewer")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	username, role, ok := m.GetSession(ctx, id)
	if !ok {
		t.Fatal("GetSession: want ok=true")
	}
	if username != "bob" || role != "viewer" {
		t.Errorf("GetSession: want bob/viewer, got %q/%q", username, role)
	}
}

func TestMemoryStore_GetSession_emptyID_returnsFalse(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()
	_, _, ok := m.GetSession(ctx, "")
	if ok {
		t.Error("GetSession with empty ID: want ok=false")
	}
}

func TestMemoryStore_GetSession_nonexistent_returnsFalse(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()
	_, _, ok := m.GetSession(ctx, "00000000000000000000000000000000")
	if ok {
		t.Error("GetSession with nonexistent ID: want ok=false")
	}
}

func TestMemoryStore_DeleteSession_removesSession(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStore()
	id, err := m.CreateSession(ctx, "carol", "admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	m.DeleteSession(ctx, id)
	_, _, ok := m.GetSession(ctx, id)
	if ok {
		t.Error("GetSession after DeleteSession: want ok=false")
	}
}
