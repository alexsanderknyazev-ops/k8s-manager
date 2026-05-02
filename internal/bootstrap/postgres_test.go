package bootstrap

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPostgresDSNIfReady_noDeployment(t *testing.T) {
	cs := fake.NewClientset()
	dsn, ready, err := PostgresDSNIfReady(t.Context(), cs, "default", "u", "p", "db")
	if err != nil {
		t.Fatal(err)
	}
	if ready || dsn != "" {
		t.Fatalf("expected not ready, got dsn=%q ready=%v", dsn, ready)
	}
}

func TestPostgresDSNIfReady_ready(t *testing.T) {
	r1 := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: "default"},
		Status:     appsv1.DeploymentStatus{AvailableReplicas: 1},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r1,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "postgres"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "pg", Image: "postgres"}}},
			},
		},
	}
	cs := fake.NewClientset(dep)
	dsn, ready, err := PostgresDSNIfReady(t.Context(), cs, "default", "user", "pass", "mydb")
	if err != nil {
		t.Fatal(err)
	}
	if !ready || dsn == "" {
		t.Fatalf("expected ready dsn, got %q ready=%v", dsn, ready)
	}
	for _, part := range []string{"user", "postgres.default.svc.cluster.local", "mydb"} {
		if !strings.Contains(dsn, part) {
			t.Fatalf("dsn %q missing %q", dsn, part)
		}
	}
}
