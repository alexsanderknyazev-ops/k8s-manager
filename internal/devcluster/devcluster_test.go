package devcluster

import (
	"strings"
	"testing"
)

func TestRunStart_manifestDirMissing(t *testing.T) {
	err := RunStart(t.Context(), "/nonexistent/path/to/deploy/dev-cluster")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "manifests dir not found") {
		t.Fatal(err)
	}
}
