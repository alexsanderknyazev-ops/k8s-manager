package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetResourceQuotasHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	if namespace == "all" {
		namespace = ""
	}
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}
	list, err := h.clientset.CoreV1().ResourceQuotas(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var result []gin.H
	for _, rq := range list.Items {
		result = append(result, gin.H{
			"name":      rq.Name,
			"namespace": rq.Namespace,
			"hard":      formatResourceList(rq.Spec.Hard),
			"used":      formatResourceList(rq.Status.Used),
			"age":       time.Since(rq.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"namespace": namespace, "count": len(result), "resourcequotas": result})
}

func formatResourceList(m corev1.ResourceList) map[string]string {
	out := make(map[string]string)
	for k, v := range m {
		out[string(k)] = v.String()
	}
	return out
}

func (h *Handler) GetLimitRangesHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	if namespace == "all" {
		namespace = ""
	}
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}
	list, err := h.clientset.CoreV1().LimitRanges(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var result []gin.H
	for _, lr := range list.Items {
		limits := make([]gin.H, 0)
		for _, limit := range lr.Spec.Limits {
			limits = append(limits, gin.H{
				"type":           string(limit.Type),
				"min":            formatResourceList(limit.Min),
				"max":            formatResourceList(limit.Max),
				"default":        formatResourceList(limit.Default),
				"defaultRequest": formatResourceList(limit.DefaultRequest),
			})
		}
		result = append(result, gin.H{
			"name":      lr.Name,
			"namespace": lr.Namespace,
			"limits":    limits,
			"age":       time.Since(lr.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"namespace": namespace, "count": len(result), "limitranges": result})
}
