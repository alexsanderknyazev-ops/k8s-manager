package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s-manager/internal/auth"
)

func (h *Handler) ListPermissionsHandler(c *gin.Context) {
	if h.permManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "permission management not available"})
		return
	}
	subject := c.Query("subject")
	namespace := c.Query("namespace")
	list, err := h.permManager.ListPermissions(c.Request.Context(), subject, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"permissions": list})
}

func (h *Handler) GrantPermissionHandler(c *gin.Context) {
	if h.permManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "permission management not available"})
		return
	}
	var body struct {
		Subject   string `json:"subject"`
		Namespace string `json:"namespace"`
		Resource  string `json:"resource"`
		Verb      string `json:"verb"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Subject == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject is required"})
		return
	}
	if body.Namespace == "" {
		body.Namespace = "*"
	}
	if body.Resource == "" {
		body.Resource = "*"
	}
	if body.Verb == "" {
		body.Verb = "read"
	}
	if body.Verb != "read" && body.Verb != "write" && body.Verb != "*" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verb must be read/write/*"})
		return
	}
	grantedBy, _ := c.Get("username")
	grantedByStr, _ := grantedBy.(string)
	if err := h.permManager.GrantPermission(c.Request.Context(), body.Subject, body.Namespace, body.Resource, body.Verb, grantedByStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = auth.InvalidateUserSessions(c.Request.Context(), body.Subject)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) RevokePermissionHandler(c *gin.Context) {
	if h.permManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "permission management not available"})
		return
	}
	subject := c.Query("subject")
	namespace := c.Query("namespace")
	resource := c.Query("resource")
	verb := c.Query("verb")
	if subject == "" || namespace == "" || resource == "" || verb == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject, namespace, resource, verb are required"})
		return
	}
	if err := h.permManager.RevokePermission(c.Request.Context(), subject, namespace, resource, verb); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = auth.InvalidateUserSessions(c.Request.Context(), subject)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
