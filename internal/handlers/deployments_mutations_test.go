package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func deploymentWebYAML(t *testing.T, image string) string {
	t.Helper()
	r1 := int32(1)
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r1,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "c", Image: image}},
				},
			},
		},
	}
	b, err := yaml.Marshal(&dep)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestScaleDeploymentHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/scale/default/web?replicas=2", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "deployment", Value: "web"}}
	h.ScaleDeploymentHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestScaleDeploymentHandler_badReplicas(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(richFakeCluster(t), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/scale/default/web?replicas=bad", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "deployment", Value: "web"}}
	h.ScaleDeploymentHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestRestartDeploymentHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/restart/default/web", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "deployment", Value: "web"}}
	h.RestartDeploymentHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestDeleteDeploymentHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/deployment/default/web", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "deployment", Value: "web"}}
	h.DeleteDeploymentHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestCreateDeploymentHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
	)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	yamlBody := `{"yaml":"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: fresh\n  namespace: default\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: fresh\n  template:\n    metadata:\n      labels:\n        app: fresh\n    spec:\n      containers:\n      - name: c\n        image: nginx\n"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/deployment/yaml", strings.NewReader(yamlBody))
	c.Request.Header.Set("Content-Type", "application/json")
	h.CreateDeploymentHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestUpdateDeploymentYAMLHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	payload, err := json.Marshal(map[string]string{"yaml": deploymentWebYAML(t, "nginx:newtag")})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/deployment/yaml/default/web", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "web"}}
	h.UpdateDeploymentYAMLHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func rollbackFakeCluster(t *testing.T) *fake.Clientset {
	t.Helper()
	r1 := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r1,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "nginx:latest"}}},
			},
		},
	}
	rsLow := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-old", Namespace: "default",
			Labels: map[string]string{"app": "web"},
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": "1",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &r1,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "nginx:rollback"}}},
			},
		},
	}
	rsHigh := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-new", Namespace: "default",
			Labels: map[string]string{"app": "web"},
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": "2",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &r1,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "nginx:current"}}},
			},
		},
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}
	return fake.NewClientset(ns, dep, rsLow, rsHigh)
}

func TestRollbackDeploymentHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := rollbackFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/rollback/default/web", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "deployment", Value: "web"}}
	h.RollbackDeploymentHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}
