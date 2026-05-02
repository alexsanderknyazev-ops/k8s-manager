package k8s

import (
	"strings"
	"testing"
)

func TestGenerateSessionID_format(t *testing.T) {
	id := GenerateSessionID("default", "nginx", 80, 18080)
	if !strings.HasPrefix(id, "default-nginx-80-18080-") {
		t.Fatal(id)
	}
}
