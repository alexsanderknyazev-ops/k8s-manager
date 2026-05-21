package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildRESTConfig_outsideClusterUsesKubeconfigPath(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	dir := t.TempDir()
	kc := filepath.Join(dir, "config")
	if err := os.WriteFile(kc, []byte(`apiVersion: v1
kind: Config
clusters: []
contexts: []
users: []
current-context: ""
`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := BuildRESTConfig(kc)
	if err != nil {
		t.Fatalf("expected clientcmd to parse empty config: %v", err)
	}
}
