package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type mockPermStore struct {
	allow map[string]bool
}

func (m *mockPermStore) HasPermission(ctx context.Context, subject, namespace, resource, verb string) bool {
	key := subject + "|" + namespace + "|" + resource + "|" + verb
	return m.allow[key]
}

func TestMiddleware_AllowsWhenPermissionExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetPermissionStore(&mockPermStore{
		allow: map[string]bool{
			"alice@example.com|default|pods|read": true,
		},
	})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "alice@example.com")
		c.Next()
	})
	r.Use(Middleware())
	r.GET("/api/pods", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/api/pods?namespace=default", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestMiddleware_DeniesWhenPermissionMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetPermissionStore(&mockPermStore{allow: map[string]bool{}})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "bob@example.com")
		c.Next()
	})
	r.Use(Middleware())
	r.POST("/api/deployment", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodPost, "/api/deployment?namespace=default", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestMiddleware_UsesNamespaceParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetPermissionStore(&mockPermStore{
		allow: map[string]bool{
			"alice@example.com|team-a|pods|read": true,
		},
	})
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "alice@example.com")
		c.Next()
	})
	r.Use(Middleware())
	r.GET("/api/pod/details/:namespace/:pod", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/api/pod/details/team-a/nginx", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}
