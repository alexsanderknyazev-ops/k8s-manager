package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

const csrfCookieName = "k8s_manager_csrf"
const csrfHeader = "X-CSRF-Token"

// GenerateCSRFToken возвращает случайный токен (32 байта hex).
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetCSRFCookie устанавливает cookie с CSRF-токеном (без HttpOnly, чтобы JS мог прочитать и отправить в заголовке).
func SetCSRFCookie(c *gin.Context, token string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   24 * 60 * 60,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

// CSRF проверяет заголовок X-CSRF-Token или форму csrf_token для мутирующих запросов к /api/*.
// Публичные пути и GET не проверяются. Требует, чтобы cookie csrf совпадал с заголовком/формой.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if len(path) < 4 || path[:4] != "/api" {
			c.Next()
			return
		}
		// Публичные API не требуют CSRF
		if path == "/api/login" || path == "/api/logout" {
			c.Next()
			return
		}
		expected, _ := c.Cookie(csrfCookieName)
		if expected == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "csrf token missing"})
			return
		}
		token := c.GetHeader(csrfHeader)
		if token == "" {
			token = c.PostForm("csrf_token")
		}
		if token != expected {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			return
		}
		c.Next()
	}
}
