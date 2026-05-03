package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestIsValidPodName(t *testing.T) {
	if !isValidPodName("nginx-1") {
		t.Fatal("valid name rejected")
	}
	if isValidPodName("") || isValidPodName("Bad") {
		t.Fatal("invalid accepted")
	}
}

func TestIsValidNamespace(t *testing.T) {
	if !isValidNamespace("default") {
		t.Fatal()
	}
	if isValidNamespace("Bad_NS") {
		t.Fatal()
	}
}

func TestCheckPortAvailableHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/portforward/check/65534", nil)
	c.Params = gin.Params{{Key: "port", Value: "65534"}}
	h.CheckPortAvailableHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
}

func TestCheckPortAvailableHandler_badPort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/portforward/check/abc", nil)
	c.Params = gin.Params{{Key: "port", Value: "abc"}}
	h.CheckPortAvailableHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestGetPortForwardSessionsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/portforward/sessions")
	h.GetPortForwardSessionsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
}

func TestStopPortForwardHandler_validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/portforward/", nil)
	c.Params = gin.Params{{Key: "id", Value: ""}}
	h.StopPortForwardHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestStopPortForwardHandler_notFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/portforward/nope", nil)
	c.Params = gin.Params{{Key: "id", Value: "nope"}}
	h.StopPortForwardHandler(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestStartPortForwardHandler_badJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxPOSTJSON(w, "/api/portforward/start", `{`)
	h.StartPortForwardHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestStartPortForwardHandler_invalidPodName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	body := `{"pod":"BAD","namespace":"default","remotePort":80,"localPort":18080}`
	w := httptest.NewRecorder()
	c := ctxPOSTJSON(w, "/api/portforward/start", body)
	h.StartPortForwardHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}
