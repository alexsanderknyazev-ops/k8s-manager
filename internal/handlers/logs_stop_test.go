package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func resetLogStreamsMap(t *testing.T) {
	t.Helper()
	logStreamsMu.Lock()
	defer logStreamsMu.Unlock()
	logStreams = make(map[string]*LogStream)
}

func TestStopLogStreamHandler_notFound(t *testing.T) {
	resetLogStreamsMap(t)
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/logs/stop/nope", nil)
	c.Params = gin.Params{{Key: "id", Value: "nope"}}
	h.StopLogStreamHandler(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestStopLogStreamHandler_ok(t *testing.T) {
	resetLogStreamsMap(t)
	stop := make(chan struct{})
	logStreamsMu.Lock()
	logStreams["sid1"] = &LogStream{ID: "sid1", StopChan: stop}
	logStreamsMu.Unlock()

	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/logs/stop/sid1", nil)
	c.Params = gin.Params{{Key: "id", Value: "sid1"}}
	h.StopLogStreamHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	resetLogStreamsMap(t)
}
