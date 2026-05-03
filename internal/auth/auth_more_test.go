package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestIsPublicPath_table(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/login", true},
		{"/api/health", true},
		{"/metrics", true},
		{"/static/js/x.js", true},
		{"/api/login", true},
		{"/api/auth/oidc/callback", true},
		{"/ui/dashboard", false},
		{"/api/pods", false},
	}
	for _, tt := range tests {
		if got := isPublicPath(tt.path); got != tt.want {
			t.Errorf("isPublicPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsAPIRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/foo", nil)
	if !isAPIRequest(req) {
		t.Error("want true for /api/foo")
	}
	req2 := httptest.NewRequest(http.MethodGet, "/ui/foo", nil)
	if isAPIRequest(req2) {
		t.Error("want false for /ui/foo")
	}
}

func TestMiddleware_skipsPublicHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/api/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestMiddleware_API_withoutCookie_UNAUTHORIZED(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/api/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestMiddleware_HTML_redirect_withoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/ui/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/ui/x", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
}

func TestLogin_JSON_success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"username":"admin","password":"secret"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	Login(c, nil, "admin", "secret", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestLogin_JSON_badJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{`))
	c.Request.Header.Set("Content-Type", "application/json")
	Login(c, nil, "admin", "secret", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestLogout_JSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	sid, err := CreateSession(ctx, "u", "admin")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	c.Request.Header.Set("Cookie", cookieName+"="+sid)
	c.Request.Header.Set("Accept", "application/json")

	Logout(c)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestInvalidateUserSessions_emptySubject(t *testing.T) {
	ctx := context.Background()
	if err := InvalidateUserSessions(ctx, ""); err != nil {
		t.Fatal(err)
	}
}

func TestSetOIDCConfig_nil(t *testing.T) {
	if err := SetOIDCConfig(nil); err != nil {
		t.Fatal(err)
	}
}

func TestOIDCLogin_notConfigured(t *testing.T) {
	_ = SetOIDCConfig(nil)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil)
	OIDCLogin(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

func TestOIDCCallback_missingParams(t *testing.T) {
	_ = SetOIDCConfig(nil)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback", nil)
	OIDCCallback(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when OIDC not configured, got %d", w.Code)
	}
}

func TestOIDCOAuthConfig_nilWhenNoOIDC(t *testing.T) {
	_ = SetOIDCConfig(nil)
	if oidcOAuthConfig() != nil {
		t.Fatal("want nil")
	}
}

type stubUserStore struct {
	hash string
	role string
	err  error
}

func (s *stubUserStore) GetUser(ctx context.Context, username string) (passwordHash, role string, err error) {
	if s.err != nil {
		return "", "", s.err
	}
	return s.hash, s.role, nil
}

func TestLogin_viaUserStore_success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hashBytes, err := bcrypt.GenerateFromPassword([]byte("pw123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := &stubUserStore{hash: string(hashBytes), role: "viewer"}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"username":"dbuser","password":"pw123"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	Login(c, store, "", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", w.Code, w.Body.String())
	}
}

func TestLogin_viaUserStore_dbError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &stubUserStore{err: errors.New("db down")}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"x","password":"y"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	Login(c, store, "", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestCookieSecure_env(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		t.Setenv("COOKIE_SECURE", "true")
		if !cookieSecure() {
			t.Error("want true")
		}
	})
	t.Run("disabled", func(t *testing.T) {
		t.Setenv("COOKIE_SECURE", "")
		if cookieSecure() {
			t.Error("want false when unset")
		}
	})
}

func TestLogin_form_success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	form := "username=admin&password=secret"
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(form))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	Login(c, nil, "admin", "secret", "")
	if w.Code != http.StatusFound && w.Code != http.StatusOK {
		t.Fatalf("unexpected %d %s", w.Code, w.Body.String())
	}
}
