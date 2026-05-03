package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGenerateCSRFTokenHexLength(t *testing.T) {
	tok, err := GenerateCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 {
		t.Errorf("token length: got %d want 64", len(tok))
	}
	for _, c := range tok {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex in token: %q", tok)
			break
		}
	}
}

func TestCSRFBlocksPOSTWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.POST("/api/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 without csrf cookie, got %d", rec.Code)
	}
}

func TestCSRFAllowsPOSTWhenCookieMatchesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.POST("/api/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	req.Header.Set(csrfHeader, "tok123")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "tok123"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 when cookie matches header, got %d", rec.Code)
	}
}

func TestCSRFSkipsGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.GET("/api/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET should skip CSRF: got %d", rec.Code)
	}
}

func TestCSRFLoginPathSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.POST("/api/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST /api/login should skip CSRF: got %d", rec.Code)
	}
}
