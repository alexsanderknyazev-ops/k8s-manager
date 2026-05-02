package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetDaemonSetsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	if namespace == "all" {
		namespace = ""
	}
	limit := int64(500)
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.ParseInt(l, 10, 64); err == nil && n > 0 {
			limit = n
		}
	}

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	list, err := h.clientset.AppsV1().DaemonSets(namespace).List(c.Request.Context(), metav1.ListOptions{Limit: limit})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, ds := range list.Items {
		ready := ds.Status.NumberReady
		desired := ds.Status.DesiredNumberScheduled
		result = append(result, gin.H{
			"name":      ds.Name,
			"namespace": ds.Namespace,
			"ready":     ready,
			"desired":   desired,
			"current":   ds.Status.CurrentNumberScheduled,
			"age":       time.Since(ds.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"namespace": namespace, "count": len(result), "daemonsets": result})
}

func (h *Handler) GetDaemonSetYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}
	ds, err := h.clientset.AppsV1().DaemonSets(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ds.ManagedFields = nil
	yamlData, _ := yaml.Marshal(ds)
	c.Data(http.StatusOK, "application/yaml", yamlData)
}
