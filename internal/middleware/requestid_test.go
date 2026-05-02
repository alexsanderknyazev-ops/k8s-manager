package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDPreservesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/t", func(c *gin.Context) {
		c.String(http.StatusOK, GetRequestID(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set(requestIDHeader, "upstream-id-123")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if got := rec.Body.String(); got != "upstream-id-123" {
		t.Errorf("body: got %q want upstream-id-123", got)
	}
	if rec.Header().Get(requestIDHeader) != "upstream-id-123" {
		t.Errorf("response header %s: got %q", requestIDHeader, rec.Header().Get(requestIDHeader))
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/t", func(c *gin.Context) {
		id := GetRequestID(c)
		if len(id) != 16 {
			t.Errorf("expected hex len 16, got %d (%q)", len(id), id)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	got := rec.Header().Get(requestIDHeader)
	if len(got) != 16 {
		t.Errorf("response header id length: got %d (%q)", len(got), got)
	}
}
