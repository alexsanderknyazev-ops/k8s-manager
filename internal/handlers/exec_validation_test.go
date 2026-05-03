package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes/fake"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/rest"
)

func TestPodExecWSHandler_restNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ws/exec", nil)
	h.PodExecWSHandler(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

func TestPodExecWSHandler_missingNamespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &rest.Config{Host: "http://127.0.0.1:6443"}
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, cfg) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ws/exec?pod=p", nil)
	h.PodExecWSHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}
