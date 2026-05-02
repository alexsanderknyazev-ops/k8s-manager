package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	"k8s-manager/internal/k8s"
)

// PortForwardRequest - структура запроса для port-forward
type PortForwardRequest struct {
	Pod        string `json:"pod" binding:"required"`
	Namespace  string `json:"namespace" binding:"required"`
	RemotePort int    `json:"remotePort" binding:"required,min=1,max=65535"`
	LocalPort  int    `json:"localPort" binding:"required,min=1024,max=65535"`
}

// Валидация имени pod
func isValidPodName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	re := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	return re.MatchString(name)
}

// Валидация имени namespace
func isValidNamespace(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	re := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	return re.MatchString(name)
}

// CheckPortAvailableHandler - проверка доступности порта
func (h *Handler) CheckPortAvailableHandler(c *gin.Context) {
	portStr := c.Param("port")
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid port number",
			"message": "Port must be a number between 1 and 65535",
		})
		return
	}

	available := !k8s.IsPortInUse(port)
	
	c.JSON(http.StatusOK, gin.H{
		"port":      port,
		"available": available,
	})
}

// GetPortForwardSessionsHandler - получение активных сессий port-forward
func (h *Handler) GetPortForwardSessionsHandler(c *gin.Context) {
	manager := k8s.GetPortForwardManager()
	sessions := manager.GetSessions()
	
	var result []gin.H
	for _, session := range sessions {
		startedAt := ""
		if !session.StartedAt.IsZero() {
			startedAt = session.StartedAt.Format(time.RFC3339)
		}
		
		result = append(result, gin.H{
			"id":         session.ID,
			"pod":        session.Pod,
			"namespace":  session.Namespace,
			"localPort":  session.LocalPort,
			"remotePort": session.RemotePort,
			"status":     session.Status,
			"createdAt":  session.CreatedAt.Format(time.RFC3339),
			"startedAt":  startedAt,
			"url":        fmt.Sprintf("http://localhost:%d", session.LocalPort),
		})
	}
	
	c.JSON(http.StatusOK, gin.H{
		"sessions": result,
		"count":    len(result),
	})
}

// StartPortForwardHandler - запуск port-forward
func (h *Handler) StartPortForwardHandler(c *gin.Context) {
	var req PortForwardRequest
	
	// Валидация JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"message": err.Error(),
		})
		return
	}
	
	// Валидация pod name
	if !isValidPodName(req.Pod) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid pod name",
			"message": "Pod name must match Kubernetes naming conventions (lowercase, numbers, dashes)",
		})
		return
	}
	
	// Валидация namespace
	if !isValidNamespace(req.Namespace) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid namespace",
			"message": "Namespace must match Kubernetes naming conventions",
		})
		return
	}
	
	// Проверяем доступность порта
	if k8s.IsPortInUse(req.LocalPort) {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Port already in use",
			"message": fmt.Sprintf("Port %d is already in use on localhost", req.LocalPort),
		})
		return
	}
	
	// Проверяем, что clientset инициализирован
	if h.clientset == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Kubernetes client not initialized",
			"message": "Please check your K8s connection",
		})
		return
	}
	
	// Проверяем существует ли pod
	pod, err := h.clientset.CoreV1().Pods(req.Namespace).Get(c.Request.Context(), req.Pod, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Pod not found",
			"message": fmt.Sprintf("Pod %s/%s does not exist or is not accessible: %v", 
				req.Namespace, req.Pod, err),
		})
		return
	}
	
	// Проверяем, что pod в состоянии Running
	if pod.Status.Phase != corev1.PodRunning {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Pod not ready",
			"message": fmt.Sprintf("Pod %s/%s is not running (current status: %s)", 
				req.Namespace, req.Pod, pod.Status.Phase),
		})
		return
	}
	
	// Создаем сессию
	session := &k8s.PortForwardSession{
		ID:         k8s.GenerateSessionID(req.Namespace, req.Pod, req.RemotePort, req.LocalPort),
		Pod:        req.Pod,
		Namespace:  req.Namespace,
		LocalPort:  req.LocalPort,
		RemotePort: req.RemotePort,
		Status:     "starting",
		CreatedAt:  time.Now(),
		URL:        fmt.Sprintf("http://localhost:%d", req.LocalPort),
		StopChan:   make(chan struct{}),
	}
	
	// Добавляем сессию в менеджер
	manager := k8s.GetPortForwardManager()
	manager.AddSession(session)
	
	// Запускаем port-forward в горутине
	go func() {
		k8s.StartPortForward(session, h.clientset)
	}()
	
	// Даем время на запуск
	time.Sleep(500 * time.Millisecond)
	
	// Проверяем статус
	updatedSession, exists := manager.GetSession(session.ID)
	if !exists || updatedSession.Status != "running" {
		manager.RemoveSession(session.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Port-forward failed to start",
			"message": "Failed to establish port-forward connection",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"session": gin.H{
			"id":         session.ID,
			"pod":        session.Pod,
			"namespace":  session.Namespace,
			"localPort":  session.LocalPort,
			"remotePort": session.RemotePort,
			"status":     updatedSession.Status,
			"url":        session.URL,
		},
		"message": fmt.Sprintf("Port-forward started successfully: localhost:%d → %s/%s:%d",
			session.LocalPort, session.Namespace, session.Pod, session.RemotePort),
	})
}

// StopPortForwardHandler - остановка port-forward
func (h *Handler) StopPortForwardHandler(c *gin.Context) {
	sessionID := c.Param("id")
	
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}
	
	manager := k8s.GetPortForwardManager()
	
	// Проверяем существование сессии
	session, exists := manager.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found",
		})
		return
	}
	
	// Останавливаем сессию
	stopped := manager.StopSession(sessionID)
	
	if stopped {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("Port-forward stopped for %s/%s", 
				session.Namespace, session.Pod),
			"session": gin.H{
				"id":        sessionID,
				"pod":       session.Pod,
				"namespace": session.Namespace,
				"status":    "stopped",
			},
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to stop session",
		})
	}
}