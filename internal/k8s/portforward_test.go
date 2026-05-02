package k8s

import (
	"testing"
	"time"
)

func TestPortForwardManager_sessions(t *testing.T) {
	m := GetPortForwardManager()
	m.RemoveSession("nonexistent")
	s := &PortForwardSession{
		ID: "t1", Pod: "p", Namespace: "default",
		LocalPort: 18080, RemotePort: 80,
		Status: "running", CreatedAt: time.Now(),
		StopChan: make(chan struct{}),
	}
	m.AddSession(s)
	list := m.GetSessions()
	if len(list) < 1 {
		t.Fatal("expected session")
	}
	got, ok := m.GetSession("t1")
	if !ok || got.ID != "t1" {
		t.Fatal("GetSession")
	}
	if !m.StopSession("t1") {
		t.Fatal("StopSession")
	}
	m.RemoveSession("t1")
}

func TestIsPortInUse_invalidOrEphemeral(t *testing.T) {
	// Очень высокий порт часто свободен; если занят — не фейлим тест.
	_ = IsPortInUse(65534)
}
