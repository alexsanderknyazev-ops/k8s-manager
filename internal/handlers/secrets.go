package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gopkg.in/yaml.v3"
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
		dataKeys := 0
		for range secret.Data {
			dataKeys++
		}
		result = append(result, gin.H{
			"name":      secret.Name,
			"namespace": secret.Namespace,
			"type":      string(secret.Type),
			"data":      dataKeys,
			"age":       time.Since(secret.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"secrets":   result,
	})
}

func (h *Handler) GetSecretYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	secret, err := h.clientset.CoreV1().Secrets(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	secret.ManagedFields = nil
	secret.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}
	yamlData, err := yaml.Marshal(secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":      name,
		"namespace": namespace,
		"yaml":      string(yamlData),
	})
}

// GetSecretDataHandler returns secret data with values base64-decoded (for "Show value" in UI).
func (h *Handler) GetSecretDataHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	secret, err := h.clientset.CoreV1().Secrets(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	decoded := make(map[string]string)
	for k, v := range secret.Data {
		decoded[k] = string(v)
	}
	c.JSON(http.StatusOK, gin.H{
		"name":      name,
		"namespace": namespace,
		"data":      decoded,
	})
}
