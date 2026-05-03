package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetLogStreamsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/logs/streams")
	h.GetLogStreamsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}
