package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"k8s-manager/internal/audit"

	"github.com/gin-gonic/gin"
)

// ReadOnly блокирует мутирующие запросы (POST/PUT/DELETE к API), если глобальный readOnly == true
// или у пользователя роль "viewer" (из c.Get("role")).
func ReadOnly(cfgReadOnly bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		readOnly := cfgReadOnly
		if !readOnly {
			if role, _ := c.Get("role"); role != nil {
				if r, _ := role.(string); r == "viewer" {
					readOnly = true
				}
			}
		}
		c.Set("effective_read_only", readOnly)
		if !readOnly {
			c.Next()
			return
		}
		method := c.Request.Method
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete {
			if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "read-only: changes are disabled"})
				return
			}
		}
		c.Next()
	}
}

// Audit логирует запрос и сохраняет в store для страницы аудита.
// При статусе >= 500 и заданном NOTIFICATION_WEBHOOK_URL отправляет POST с телом события.
func Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		user, _ := c.Get("username")
		username := ""
		if u, ok := user.(string); ok {
			username = u
		}
		status := c.Writer.Status()
		requestID := GetRequestID(c)
		audit.Append(c.Request.Context(), c.Request.Method, c.Request.URL.Path, status, username, requestID)
		args := []any{"method", c.Request.Method, "path", c.Request.URL.Path, "status", status, "user", username}
		if requestID != "" {
			args = append(args, "request_id", requestID)
		}
		slog.Info("request", args...)
		if status >= 500 {
			if url := os.Getenv("NOTIFICATION_WEBHOOK_URL"); url != "" {
				go sendWebhook(url, map[string]any{
					"event": "request_error", "method": c.Request.Method, "path": c.Request.URL.Path,
					"status": status, "user": username, "request_id": requestID,
				})
			}
		}
	}
}

func sendWebhook(url string, payload map[string]any) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	_, _ = client.Do(req)
}
