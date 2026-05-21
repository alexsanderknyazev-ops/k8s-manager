package devcluster

import (
	"fmt"
	"strings"
	"testing"
)

func TestDevClusterSkipMarket(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"no", false},
		{"1", true},
		{"TRUE", true},
		{" yes ", true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.env), func(t *testing.T) {
			t.Setenv("DEV_CLUSTER_SKIP_MARKET", tc.env)
			if got := devClusterSkipMarket(); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestRunStart_manifestDirMissing(t *testing.T) {
	err := RunStart(t.Context(), "/nonexistent/path/to/deploy/dev-cluster")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "manifests dir not found") {
		t.Fatal(err)
	}
}
