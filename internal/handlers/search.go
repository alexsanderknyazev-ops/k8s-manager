package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) SearchHandler(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"pods": []gin.H{}, "deployments": []gin.H{}, "services": []gin.H{}})
		return
	}

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	namespace := c.DefaultQuery("namespace", "default")
	qLower := strings.ToLower(q)

	var pods []gin.H
	var deployments []gin.H
	var services []gin.H

	// Pods
	podList, err := h.clientset.CoreV1().Pods(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err == nil {
		for _, p := range podList.Items {
			if strings.Contains(strings.ToLower(p.Name), qLower) {
				pods = append(pods, gin.H{"name": p.Name, "namespace": p.Namespace, "kind": "Pod"})
			}
		}
	}

	// Deployments
	depList, err := h.clientset.AppsV1().Deployments(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err == nil {
		for _, d := range depList.Items {
			if strings.Contains(strings.ToLower(d.Name), qLower) {
				deployments = append(deployments, gin.H{"name": d.Name, "namespace": d.Namespace, "kind": "Deployment"})
			}
		}
	}

	// Services
	svcList, err := h.clientset.CoreV1().Services(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err == nil {
		for _, s := range svcList.Items {
			if strings.Contains(strings.ToLower(s.Name), qLower) {
				services = append(services, gin.H{"name": s.Name, "namespace": s.Namespace, "kind": "Service"})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"query":       q,
		"namespace":  namespace,
		"pods":       pods,
		"deployments": deployments,
		"services":   services,
	})
}
