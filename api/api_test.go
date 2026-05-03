package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s-manager/internal/auth"
	"k8s-manager/internal/config"
	"k8s-manager/internal/store"

	"github.com/gin-gonic/gin"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

type mockPermManager struct {
	allow map[string]bool
}

func (m *mockPermManager) key(subject, namespace, resource, verb string) string {
	return subject + "|" + namespace + "|" + resource + "|" + verb
}
func (m *mockPermManager) ListPermissions(ctx context.Context, subject, namespace string) ([]store.Permission, error) {
	return nil, nil
}
func (m *mockPermManager) GrantPermission(ctx context.Context, subject, namespace, resource, verb, grantedBy string) error {
	if m.allow == nil {
		m.allow = map[string]bool{}
	}
	m.allow[m.key(subject, namespace, resource, verb)] = true
	return nil
}
func (m *mockPermManager) RevokePermission(ctx context.Context, subject, namespace, resource, verb string) error {
	delete(m.allow, m.key(subject, namespace, resource, verb))
	return nil
}
func (m *mockPermManager) HasPermission(ctx context.Context, subject, namespace, resource, verb string) bool {
	return m.allow[m.key(subject, namespace, resource, verb)]
}

func TestHealthWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: false}}
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, nil) //nolint:staticcheck

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/health: want 200, got %d", rec.Code)
	}
}

func TestAPIDocsWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: false}}
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, nil) //nolint:staticcheck

	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/docs: want 200, got %d", rec.Code)
	}
}

func TestMetricsWithoutAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Auth: config.AuthConfig{Enabled: false}}
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, nil) //nolint:staticcheck

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /metrics: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "rbac_denied_total") {
		t.Errorf("GET /metrics: expected rbac_denied_total in body, got %q", truncate(body, 300))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func TestHealthWithAuthIsPublic(t *testing.T) {
	// /api/health доступен без авторизации (для liveness/readiness проб)
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Auth: config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"},
	}
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, nil) //nolint:staticcheck

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/health (public): want 200, got %d", rec.Code)
	}
}

func TestLoginWrongPasswordReturns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Auth: config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"},
	}
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, nil) //nolint:staticcheck

	body := []byte(`{"username":"admin","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/login wrong password: want 401, got %d", rec.Code)
	}
}

func TestProtectedRouteWithoutSessionReturns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Auth: config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"},
	}
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, nil) //nolint:staticcheck

	req := httptest.NewRequest(http.MethodGet, "/api/namespaces", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/namespaces without session: want 401, got %d", rec.Code)
	}
}

func TestRBACAllowWithPermissionAndSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Auth: config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"},
	}
	pm := &mockPermManager{allow: map[string]bool{}}
	pm.allow[pm.key("alice", "default", "pods", "read")] = true
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, pm) //nolint:staticcheck

	sid, err := auth.CreateSession(context.Background(), "alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/pods?namespace=default", nil)
	req.AddCookie(&http.Cookie{Name: "k8s_manager_session", Value: sid})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/pods with permission: want 200, got %d", rec.Code)
	}
}

func TestRBACDenyWithoutPermissionEvenWithSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Auth: config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"},
	}
	pm := &mockPermManager{allow: map[string]bool{}}
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, pm) //nolint:staticcheck

	sid, err := auth.CreateSession(context.Background(), "alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/pods?namespace=default", nil)
	req.AddCookie(&http.Cookie{Name: "k8s_manager_session", Value: sid})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("GET /api/pods without permission: want 403, got %d", rec.Code)
	}
}

func TestPermissionsAPIDeniedWithoutPermissionsResourceAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Auth: config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"},
	}
	pm := &mockPermManager{allow: map[string]bool{
		"alice|default|pods|read": true, // не даёт права на /api/permissions
	}}
	r := gin.New()
	SetupRoutes(r, fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, cfg, nil, nil, pm) //nolint:staticcheck

	sid, err := auth.CreateSession(context.Background(), "alice", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/permissions", nil)
	req.AddCookie(&http.Cookie{Name: "k8s_manager_session", Value: sid})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("GET /api/permissions without permissions access: want 403, got %d", rec.Code)
	}
}
