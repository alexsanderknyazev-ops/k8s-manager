package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetSecretsHandler(c *gin.Context) {
	namespace := c.Param("namespace")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	secrets, err := h.clientset.CoreV1().Secrets(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, secret := range secrets.Items {
		result = append(result, gin.H{
			"name":      secret.Name,
			"namespace": secret.Namespace,
			"type":      string(secret.Type),
			"age":       time.Since(secret.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"secrets":   result,
	})
}
