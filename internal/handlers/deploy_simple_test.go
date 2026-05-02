package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSimpleDeployHandler_successWithService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
	)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	body := `{"name":"app1","namespace":"default","image":"nginx:alpine","replicas":1,"container_port":80,"create_service":true,"service_port":80}`
	w := httptest.NewRecorder()
	c := ctxPOSTJSON(w, "/api/deploy/simple", body)
	h.SimpleDeployHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestSimpleDeployHandler_badJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxPOSTJSON(w, "/api/deploy/simple", `{`)
	h.SimpleDeployHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSimpleDeployHandler_missingName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	body := `{"name":"","namespace":"default","image":"nginx"}`
	w := httptest.NewRecorder()
	c := ctxPOSTJSON(w, "/api/deploy/simple", body)
	h.SimpleDeployHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSimpleDeployHandler_noClientset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	body := `{"name":"x","namespace":"default","image":"nginx"}`
	w := httptest.NewRecorder()
	c := ctxPOSTJSON(w, "/api/deploy/simple", body)
	h.SimpleDeployHandler(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}
