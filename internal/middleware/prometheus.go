package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	reqCount   = make(map[string]int64)
	reqCountMu sync.Mutex
	rbacDeny  int64
)

func reqKey(method, path, status string) string {
	return method + "|" + path + "|" + status
}

// PrometheusMetrics считает запросы по method, path, status и пишет в reqCount.
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		key := reqKey(method, path, status)
		reqCountMu.Lock()
		reqCount[key]++
		if c.Writer.Header().Get("X-RBAC-Deny") == "1" {
			rbacDeny++
		}
		reqCountMu.Unlock()
		_ = start
	}
}

// MetricsHandler отдаёт метрики в формате Prometheus (счётчики запросов).
func MetricsHandler(c *gin.Context) {
	reqCountMu.Lock()
	defer reqCountMu.Unlock()
	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	parts := []string{}
	for key, count := range reqCount {
		seg := strings.SplitN(key, "|", 3)
		if len(seg) != 3 {
			continue
		}
		parts = append(parts, fmt.Sprintf("http_requests_total{method=%q,path=%q,status=%q} %d", seg[0], seg[1], seg[2], count))
	}
	for _, p := range parts {
		_, _ = c.Writer.Write([]byte(p + "\n"))
	}
	_, _ = fmt.Fprintf(c.Writer, "rbac_denied_total %d\n", rbacDeny)
	if len(parts) == 0 {
		_, _ = c.Writer.Write([]byte("# No requests yet\n"))
	}
	c.Status(http.StatusOK)
}
