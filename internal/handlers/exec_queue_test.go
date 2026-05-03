package handlers

import (
	"testing"

	"k8s.io/client-go/tools/remotecommand"
)

func TestSizeQueue_Next(t *testing.T) {
	ch := make(chan *remotecommand.TerminalSize, 2)
	q := &sizeQueue{ch: ch}
	w := &remotecommand.TerminalSize{Width: 80, Height: 24}
	ch <- w
	got := q.Next()
	if got == nil || got.Width != 80 {
		t.Fatal("expected size")
	}
	close(ch)
	if q.Next() != nil {
		t.Fatal("closed channel should yield nil")
	}
}
