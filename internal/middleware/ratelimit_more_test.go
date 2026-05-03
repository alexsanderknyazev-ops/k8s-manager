package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestCleanup_removesStaleLimiter(t *testing.T) {
	mu := sync.RWMutex{}
	store := map[string]*ipLimiter{
		"1.2.3.4": {lastSeen: time.Now().Add(-10 * time.Minute)},
	}
	cleanup(&mu, store, 5*time.Minute)
	mu.RLock()
	defer mu.RUnlock()
	if len(store) != 0 {
		t.Fatal("expected stale ip removed")
	}
}

func TestGetLimiter_returnsNilWhenPerMinZero(t *testing.T) {
	mu := sync.RWMutex{}
	store := map[string]*ipLimiter{}
	l := getLimiter(&mu, store, "10.0.0.1", 0)
	if l != nil {
		t.Fatal("expected nil limiter")
	}
}

func TestRateLimitAPI_manyRequestsEventuallyLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimitAPI(2))
	r.GET("/api/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	ip := "198.51.100.7"
	var lastCode int
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		lastCode = rec.Code
		if lastCode == http.StatusTooManyRequests {
			break
		}
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 at least once, last=%d", lastCode)
	}
}
