package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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

	// В реальной реализации здесь должна быть логика получения приложений
	// Пока возвращаем заглушку
	c.JSON(http.StatusOK, gin.H{
		"namespace":    namespace,
		"count":        0,
		"applications": []gin.H{},
		"message":      "Applications endpoint - implementation pending",
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

// GetPodMetricsHandler - Обработчик для получения метрик подов
func (h *Handler) GetPodMetricsHandler(c *gin.Context) {
	namespace := c.Param("namespace")

	if h.clientset == nil || h.metricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
		return
	}

	// Реализация будет в отдельном файле
	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"metrics":   []gin.H{},
		"message":   "Pod metrics endpoint - implementation pending",
	})
}

// GetSinglePodMetricsHandler - Обработчик для получения метрик конкретного пода
func (h *Handler) GetSinglePodMetricsHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	c.JSON(http.StatusOK, gin.H{
		"pod":       podName,
		"namespace": namespace,
		"message":   "Single pod metrics endpoint - implementation pending",
	})
}

// GetAllPodsMetricsHandler - Обработчик для получения метрик всех подов
func (h *Handler) GetAllPodsMetricsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "All pods metrics endpoint - implementation pending",
	})
}

// GetNodeMetricsHandler - Обработчик для получения метрик нод
func (h *Handler) GetNodeMetricsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Node metrics endpoint - implementation pending",
	})
}

// GetPortForwardSessionsHandler - Обработчик для получения активных сессий port-forward
func (h *Handler) GetPortForwardSessionsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Port-forward sessions endpoint - implementation pending",
	})
}

// StartPortForwardHandler - Обработчик для запуска port-forward
func (h *Handler) StartPortForwardHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Start port-forward endpoint - implementation pending",
	})
}

// StopPortForwardHandler - Обработчик для остановки port-forward
func (h *Handler) StopPortForwardHandler(c *gin.Context) {
	sessionID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"session": sessionID,
		"message": "Stop port-forward endpoint - implementation pending",
	})
}

// CheckPortAvailableHandler - Обработчик для проверки доступности порта
func (h *Handler) CheckPortAvailableHandler(c *gin.Context) {
	portStr := c.Param("port")
	c.JSON(http.StatusOK, gin.H{
		"port":      portStr,
		"available": false,
		"message":   "Check port available endpoint - implementation pending",
	})
}
