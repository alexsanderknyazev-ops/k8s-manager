package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s-manager/internal/audit"

	"github.com/gin-gonic/gin"
)

func TestReadOnly_blocksMutatingAPIWhenGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ReadOnly(true))
	r.POST("/api/pods", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/pods", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestReadOnly_viewerRole_blocksMutatingAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "viewer")
		c.Next()
	})
	r.Use(ReadOnly(false))
	r.DELETE("/api/pod/default/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodDelete, "/api/pod/default/x", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestReadOnly_allowsGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ReadOnly(true))
	r.GET("/api/pods", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/pods", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestAudit_logsAfterRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	audit.SetPersistentStore(nil)

	ctx := context.Background()
	r := gin.New()
	r.Use(RequestID())
	r.Use(func(c *gin.Context) {
		c.Set("username", "alice")
		c.Next()
	})
	r.Use(Audit())
	path := "/api/audit-mw-" + t.Name()
	r.GET(path, func(c *gin.Context) { c.Status(http.StatusTeapot) })

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status %d", rec.Code)
	}
	found := false
	for _, e := range audit.Get(ctx, 50) {
		if strings.Contains(e.Path, path) && e.Username == "alice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected audit entry for path", path)
	}
}

func TestSendWebhook_invalidURL_noPanic(t *testing.T) {
	sendWebhook("://bad", map[string]any{"x": 1})
}

func TestSendWebhook_unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	sendWebhook(srv.URL, map[string]any{"event": "test"})
}
