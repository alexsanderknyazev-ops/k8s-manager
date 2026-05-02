package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "request_id"
const requestIDHeader = "X-Request-Id"

// RequestID генерирует или читает X-Request-Id, кладёт в контекст и в заголовок ответа.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			}
		}
		c.Set(requestIDKey, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// GetRequestID возвращает request id из контекста (если есть).
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// SlogRequestID добавляет request_id в slog при логировании (для использования в Audit и т.д.).
func SlogRequestID(c *gin.Context) []any {
	id := GetRequestID(c)
	if id != "" {
		return []any{"request_id", id}
	}
	return nil
}
