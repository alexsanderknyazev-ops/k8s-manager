package handlers

import (
	"net/http"
	"strconv"

	"k8s-manager/internal/audit"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetAuditHandler(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	entries := audit.Get(c.Request.Context(), limit)
	c.JSON(http.StatusOK, gin.H{
		"count":   len(entries),
		"entries": entries,
	})
}
