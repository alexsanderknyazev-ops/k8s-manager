package handlers

import (
	"net/http"
	"time"

	"fmt"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Handler struct {
	clientset     *kubernetes.Clientset
	metricsClient *metricsv.Clientset
}

func NewHandler(clientset *kubernetes.Clientset, metricsClient *metricsv.Clientset) *Handler {
	return &Handler{
		clientset:     clientset,
		metricsClient: metricsClient,
	}
}

func (h *Handler) HomeHandler(c *gin.Context) {
	status := "disconnected"
	if h.clientset != nil {
		status = "connected"
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    "Kubernetes Manager API",
		"version": "1.0.0",
		"status":  status,
		"endpoints": []string{
			"GET  /api/health - Health check",
			"GET  /api/test - Test K8s connection",
			"GET  /api/applications - List applications",
			"GET  /api/pods?namespace=default - List pods",
			"GET  /api/logs/:namespace/:pod?tail=100 - Get pod logs",
			"GET  /api/logs/download/:namespace/:pod - Download logs",
			"GET  /api/pod/yaml/:namespace/:pod - Get pod YAML",
			"PUT  /api/pod/yaml/:namespace/:pod - Update pod YAML",
			"DELETE /api/pod/:namespace/:pod - Delete pod",
			"GET  /api/deployments?namespace=default - List deployments",
			"GET  /api/deployment/yaml/:namespace/:name - Get deployment YAML",
			"PUT  /api/deployment/yaml/:namespace/:name - Update deployment YAML",
			"POST /api/scale/:namespace/:deployment?replicas=N - Scale deployment",
			"POST /api/restart/:namespace/:deployment - Restart deployment",
			"DELETE /api/deployment/:namespace/:deployment - Delete deployment",
			"GET  /api/services?namespace=default - List services",
			"GET  /api/configmaps/:namespace - List configmaps",
			"GET  /api/secrets/:namespace - List secrets",
			"GET  /api/namespaces - List namespaces",
			"GET  /api/nodes - List nodes",
			"GET  /api/metrics/pods/:namespace - Get pod metrics",
			"GET  /api/metrics/nodes - Get node metrics",
			"GET  /api/portforward/sessions - Get active port-forward sessions",
			"POST /api/portforward/start - Start port-forward",
			"POST /api/portforward/stop/:id - Stop port-forward",
		},
	})
}

func (h *Handler) HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "k8s-manager",
		"k8s":     h.clientset != nil,
		"metrics": h.metricsClient != nil,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) TestConnectionHandler(c *gin.Context) {
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"connected": false,
			"error":     "Kubernetes client not initialized",
		})
		return
	}

	_, err := h.clientset.CoreV1().Namespaces().List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"connected": true,
		"message":   "Successfully connected to Kubernetes API",
	})
}

// GetApplicationsHandler - Обработчик для получения списка приложений
func (h *Handler) GetApplicationsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "all")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	// Получаем деплойменты
	var deployments *appsv1.DeploymentList
	var err error

	if namespace == "all" {
		deployments, err = h.clientset.AppsV1().Deployments("").List(c.Request.Context(), metav1.ListOptions{})
	} else {
		deployments, err = h.clientset.AppsV1().Deployments(namespace).List(c.Request.Context(), metav1.ListOptions{})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var applications []gin.H
	for _, dep := range deployments.Items {
		readyReplicas := int32(0)
		if dep.Status.ReadyReplicas > 0 {
			readyReplicas = dep.Status.ReadyReplicas
		}

		totalReplicas := int32(1)
		if dep.Spec.Replicas != nil {
			totalReplicas = *dep.Spec.Replicas
		}

		applications = append(applications, gin.H{
			"name":        dep.Name,
			"namespace":   dep.Namespace,
			"type":        "Deployment",
			"instances":   totalReplicas,
			"ready":       fmt.Sprintf("%d/%d", readyReplicas, totalReplicas),
			"ready_count": readyReplicas,
			"total_count": totalReplicas,
			"age":         time.Since(dep.CreationTimestamp.Time).Round(time.Second).String(),
			"labels":      dep.Labels,
			"strategy":    string(dep.Spec.Strategy.Type),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace":    namespace,
		"count":        len(applications),
		"applications": applications,
	})
}

// GetNamespacesHandler - Обработчик для получения списка namespace
func (h *Handler) GetNamespacesHandler(c *gin.Context) {
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	namespaces, err := h.clientset.CoreV1().Namespaces().List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, ns := range namespaces.Items {
		result = append(result, gin.H{
			"name":   ns.Name,
			"status": string(ns.Status.Phase),
			"age":    time.Since(ns.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"count":      len(namespaces.Items),
		"namespaces": result,
	})
}

// GetNodesHandler - Обработчик для получения списка нод
func (h *Handler) GetNodesHandler(c *gin.Context) {
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	nodes, err := h.clientset.CoreV1().Nodes().List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, node := range nodes.Items {
		conditions := []string{}
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" {
				conditions = append(conditions, string(condition.Status))
			}
		}

		result = append(result, gin.H{
			"name":       node.Name,
			"status":     conditions,
			"age":        time.Since(node.CreationTimestamp.Time).Round(time.Second).String(),
			"version":    node.Status.NodeInfo.KubeletVersion,
			"os":         node.Status.NodeInfo.OSImage,
			"kernel":     node.Status.NodeInfo.KernelVersion,
			"containerd": node.Status.NodeInfo.ContainerRuntimeVersion,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"count": len(nodes.Items),
		"nodes": result,
	})
}
