package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetConfigMapsHandler(c *gin.Context) {
	namespace := c.Param("namespace")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	configmaps, err := h.clientset.CoreV1().ConfigMaps(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, cm := range configmaps.Items {
		result = append(result, gin.H{
			"name":      cm.Name,
			"namespace": cm.Namespace,
			"data":      len(cm.Data),
			"age":       time.Since(cm.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace":  namespace,
		"count":      len(result),
		"configmaps": result,
	})
}

func (h *Handler) GetConfigMapYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	configmap, err := h.clientset.CoreV1().ConfigMaps(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	configmap.ManagedFields = nil
	configmap.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	}

	yamlData, err := yaml.Marshal(configmap)
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
