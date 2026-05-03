package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestCheckPassword_plainAndHash(t *testing.T) {
	if !CheckPassword("secret", "secret", "") {
		t.Fatal("plain match")
	}
	if CheckPassword("wrong", "secret", "") {
		t.Fatal()
	}
	h, err := bcrypt.GenerateFromPassword([]byte("x"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword("x", "", string(h)) {
		t.Fatal("hash match")
	}
	if CheckPassword("y", "", string(h)) {
		t.Fatal()
	}
}

func TestRejectLogin_JSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login", nil)
	c.Request.Header.Set("Accept", "application/json")
	rejectLogin(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatal(w.Code)
	}
}

func TestOIDCCallback_oidcDisabled(t *testing.T) {
	_ = SetOIDCConfig(nil)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/cb?state=x&code=y", nil)
	OIDCCallback(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 without OIDC, got %d", w.Code)
	}
}

func TestRedirectOr401_HTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ui/x", nil)
	redirectOr401(c)
	if w.Code != http.StatusFound {
		t.Fatal(w.Code)
	}
}

func TestIsPublicPath_apiLogout(t *testing.T) {
	if !isPublicPath("/api/logout") {
		t.Fatal()
	}
}
