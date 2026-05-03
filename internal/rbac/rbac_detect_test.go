package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDetectVerb(t *testing.T) {
	if detectVerb(http.MethodGet) != "read" {
		t.Fatal("GET -> read")
	}
	if detectVerb(http.MethodPost) != "write" {
		t.Fatal("POST -> write")
	}
	if detectVerb(http.MethodHead) != "read" {
		t.Fatal("HEAD -> read")
	}
}

func TestDetectResource_table(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/permissions", "permissions"},
		{"/api/pods", "pods"},
		{"/api/pod/foo/bar", "pods"},
		{"/api/deployments", "deployments"},
		{"/api/services", "services"},
		{"/api/configmaps/x", "configmaps"},
		{"/api/secrets/x", "secrets"},
		{"/api/ingress/foo", "ingresses"},
		{"/api/hpa", "hpa"},
		{"/api/statefulset", "statefulsets"},
		{"/api/daemonset", "daemonsets"},
		{"/api/cronjob", "jobs"},
		{"/api/nodes", "nodes"},
		{"/api/namespaces", "namespaces"},
		{"/api/unknown", "cluster"},
	}
	for _, tt := range tests {
		if got := detectResource(tt.path); got != tt.want {
			t.Errorf("detectResource(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestMiddleware_skipsNonAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetPermissionStore(&mockPermStore{allow: map[string]bool{}})
	SetLegacyAdminBypass(false)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "x")
		c.Next()
	})
	r.Use(Middleware())
	r.GET("/ui/foo", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ui/foo", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}
