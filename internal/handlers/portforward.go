package handlers

import (
	// "fmt"
	// "net/http"
	// "strconv"
	// "time"

	// "k8s-manager/internal/k8s"

	// "github.com/gin-gonic/gin"
	// corev1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func (h *Handler) GetPortForwardSessionsHandler(c *gin.Context) {
// 	manager := k8s.GetPortForwardManager()
// 	sessions := manager.GetSessions()

// 	c.JSON(http.StatusOK, gin.H{
// 		"count":    len(sessions),
// 		"sessions": sessions,
// 	})
// }

// func (h *Handler) StartPortForwardHandler(c *gin.Context) {
// 	var request struct {
// 		Pod        string `json:"pod" binding:"required"`
// 		Namespace  string `json:"namespace" binding:"required"`
// 		RemotePort int    `json:"remotePort" binding:"required"`
// 		LocalPort  int    `json:"localPort"`
// 	}

// 	if err := c.BindJSON(&request); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}

// 	// Проверяем, существует ли под
// 	pod, err := h.clientset.CoreV1().Pods(request.Namespace).Get(
// 		c.Request.Context(), request.Pod, metav1.GetOptions{})
// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found: " + err.Error()})
// 		return
// 	}

// 	// Проверяем, готов ли под
// 	if pod.Status.Phase != corev1.PodRunning {
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Pod is not running",
// 			"phase": pod.Status.Phase,
// 		})
// 		return
// 	}

// 	// Если localPort не указан, используем тот же что и remote
// 	if request.LocalPort == 0 {
// 		request.LocalPort = request.RemotePort
// 	}

// 	// Проверяем, не занят ли local порт
// 	if k8s.IsPortInUse(request.LocalPort) {
// 		c.JSON(http.StatusConflict, gin.H{
// 			"error": fmt.Sprintf("Local port %d is already in use", request.LocalPort),
// 		})
// 		return
// 	}

// 	// Создаем сессию
// 	sessionID := k8s.GenerateSessionID(request.Namespace, request.Pod, request.RemotePort, request.LocalPort)

// 	session := &k8s.PortForwardSession{
// 		ID:         sessionID,
// 		Pod:        request.Pod,
// 		Namespace:  request.Namespace,
// 		LocalPort:  request.LocalPort,
// 		RemotePort: request.RemotePort,
// 		Status:     "starting",
// 		CreatedAt:  time.Now(),
// 		StopChan:   make(chan struct{}),
// 		URL:        fmt.Sprintf("http://localhost:%d", request.LocalPort),
// 	}

// 	// Сохраняем сессию
// 	manager := k8s.GetPortForwardManager()
// 	manager.AddSession(session)

// 	// Запускаем port-forward в горутине
// 	go k8s.StartPortForward(session, h.clientset)

// 	c.JSON(http.StatusOK, gin.H{
// 		"session": session,
// 		"message": fmt.Sprintf("Port-forward started: %d -> %s:%d",
// 			request.LocalPort, request.Pod, request.RemotePort),
// 	})
// }

// func (h *Handler) StopPortForwardHandler(c *gin.Context) {
// 	sessionID := c.Param("id")

// 	manager := k8s.GetPortForwardManager()
// 	stopped := manager.StopSession(sessionID)

// 	if !stopped {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"message": "Port-forward stopped",
// 		"session": sessionID,
// 	})
// }

// func (h *Handler) CheckPortAvailableHandler(c *gin.Context) {
// 	portStr := c.Param("port")
// 	port, err := strconv.Atoi(portStr)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid port number"})
// 		return
// 	}

// 	available := !k8s.IsPortInUse(port)
// 	c.JSON(http.StatusOK, gin.H{
// 		"port":      port,
// 		"available": available,
// 	})
// }
