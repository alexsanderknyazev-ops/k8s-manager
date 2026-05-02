package k8s

import (
	"testing"

	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFormatBytes_k8s(t *testing.T) {
	if FormatBytes(512) != "512B" {
		t.Fatal(FormatBytes(512))
	}
	if FormatBytes(2048) == "" {
		t.Fatal("empty")
	}
}

func TestMin_k8s(t *testing.T) {
	if Min(3, 7) != 3 || Min(9, 2) != 2 {
		t.Fatal("Min")
	}
}

func TestGetPodMetrics_emptyLists(t *testing.T) {
	cs := fake.NewClientset()
	mc := metricsfake.NewSimpleClientset() //nolint:staticcheck
	got, err := GetPodMetrics(mc, cs, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}

func TestGetPodMetrics_nilMetricsClient(t *testing.T) {
	cs := fake.NewClientset()
	_, err := GetPodMetrics(nil, cs, "default")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestGetNodeMetrics_empty(t *testing.T) {
	cs := fake.NewClientset()
	mc := metricsfake.NewSimpleClientset() //nolint:staticcheck
	nodes, cluster, err := GetNodeMetrics(mc, cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("want 0 nodes metrics, got %d", len(nodes))
	}
	_ = cluster
}

func TestGetSinglePodMetrics_notFound(t *testing.T) {
	cs := fake.NewClientset()
	mc := metricsfake.NewSimpleClientset() //nolint:staticcheck
	_, _, err := GetSinglePodMetrics(mc, cs, "default", "missing")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestGetAllPodsMetrics_noNamespaces(t *testing.T) {
	cs := fake.NewClientset()
	mc := metricsfake.NewSimpleClientset() //nolint:staticcheck
	all, cpu, mem, err := GetAllPodsMetrics(mc, cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 || cpu != 0 || mem != 0 {
		t.Fatalf("want empty totals, got len=%d cpu=%d mem=%d", len(all), cpu, mem)
	}
}
