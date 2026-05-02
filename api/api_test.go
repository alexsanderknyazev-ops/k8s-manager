package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s-manager/internal/auth"
	"k8s-manager/internal/config"
	"k8s-manager/internal/store"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes/fake"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned/fake"
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
	SetupRoutes(r, fake.NewSimpleClientset(), metricsv.NewSimpleClientset(), nil, cfg, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/health: want 200, got %d", rec.Code)
	}
}

func TestHealthWithAuthIsPublic(t *testing.T) {
	// /api/health доступен без авторизации (для liveness/readiness проб)
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Auth: config.AuthConfig{Enabled: true, Username: "admin", Password: "secret"},
	}
	r := gin.New()
	SetupRoutes(r, fake.NewSimpleClientset(), metricsv.NewSimpleClientset(), nil, cfg, nil, nil, nil)

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
	SetupRoutes(r, fake.NewSimpleClientset(), metricsv.NewSimpleClientset(), nil, cfg, nil, nil, nil)

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
	SetupRoutes(r, fake.NewSimpleClientset(), metricsv.NewSimpleClientset(), nil, cfg, nil, nil, nil)

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
	SetupRoutes(r, fake.NewSimpleClientset(), metricsv.NewSimpleClientset(), nil, cfg, nil, nil, pm)

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
	SetupRoutes(r, fake.NewSimpleClientset(), metricsv.NewSimpleClientset(), nil, cfg, nil, nil, pm)

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
	SetupRoutes(r, fake.NewSimpleClientset(), metricsv.NewSimpleClientset(), nil, cfg, nil, nil, pm)

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
