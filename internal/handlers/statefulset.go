package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetStatefulSetsHandler(c *gin.Context) {
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

	list, err := h.clientset.AppsV1().StatefulSets(namespace).List(c.Request.Context(), metav1.ListOptions{Limit: limit})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, sts := range list.Items {
		ready := int32(0)
		if sts.Status.ReadyReplicas > 0 {
			ready = sts.Status.ReadyReplicas
		}
		replicas := int32(0)
		if sts.Spec.Replicas != nil {
			replicas = *sts.Spec.Replicas
		}
		result = append(result, gin.H{
			"name":        sts.Name,
			"namespace":   sts.Namespace,
			"ready":       ready,
			"replicas":    replicas,
			"ready_count": ready,
			"age":         time.Since(sts.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"namespace": namespace, "count": len(result), "statefulsets": result})
}

func (h *Handler) GetStatefulSetYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}
	sts, err := h.clientset.AppsV1().StatefulSets(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	sts.ManagedFields = nil
	yamlData, _ := yaml.Marshal(sts)
	c.Data(http.StatusOK, "application/yaml", yamlData)
}

func (h *Handler) ScaleStatefulSetHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	replicasStr := c.DefaultQuery("replicas", "1")
	replicas, err := strconv.Atoi(replicasStr)
	if err != nil || replicas < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid replicas"})
		return
	}
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}
	sts, err := h.clientset.AppsV1().StatefulSets(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	r := int32(replicas)
	sts.Spec.Replicas = &r
	_, err = h.clientset.AppsV1().StatefulSets(namespace).Update(c.Request.Context(), sts, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "replicas": replicas})
}
