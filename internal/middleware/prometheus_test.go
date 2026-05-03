package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func resetPrometheusState() {
	reqCountMu.Lock()
	defer reqCountMu.Unlock()
	reqCount = make(map[string]int64)
	rbacDeny = 0
}

func TestReqKey_roundTrip(t *testing.T) {
	k := reqKey("GET", "/api/x", "200")
	if k != "GET|/api/x|200" {
		t.Fatal(k)
	}
}

func TestMetricsHandler_emptyState(t *testing.T) {
	resetPrometheusState()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	MetricsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "# No requests yet") {
		t.Fatal(body)
	}
	if !strings.Contains(body, "rbac_denied_total 0") {
		t.Fatal(body)
	}
}

func TestPrometheusMetrics_countsAndRBACDeny(t *testing.T) {
	resetPrometheusState()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PrometheusMetrics())
	r.GET("/test-path", func(c *gin.Context) {
		c.Writer.Header().Set("X-RBAC-Deny", "1")
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	MetricsHandler(c2)
	body := w2.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",path="/test-path",status="200"}`) || !strings.Contains(body, " 1") {
		t.Fatal(body)
	}
	if !strings.Contains(body, "rbac_denied_total 1") {
		t.Fatal(body)
	}
	resetPrometheusState()
}
