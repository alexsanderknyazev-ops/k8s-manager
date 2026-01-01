package handlers

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Для разработки
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// Хранилище активных WebSocket соединений
	logStreams   = make(map[string]*LogStream)
	logStreamsMu sync.RWMutex
)

type LogStream struct {
	ID         string
	Namespace  string
	Pod        string
	Conn       *websocket.Conn
	StopChan   chan struct{}
	BufferSize int
	TailLines  int64
	Follow     bool
}

type LogMessage struct {
	Type    string      `json:"type"` // log, error, info, warning
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Time    string      `json:"time"`
}

// GetLogStreamsHandler - Получение активных лог-стримов
func (h *Handler) GetLogStreamsHandler(c *gin.Context) {
	logStreamsMu.RLock()
	defer logStreamsMu.RUnlock()

	streams := make([]map[string]interface{}, 0)
	for id, stream := range logStreams {
		streams = append(streams, map[string]interface{}{
			"id":        id,
			"pod":       stream.Pod,
			"namespace": stream.Namespace,
			"follow":    stream.Follow,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(streams),
		"streams": streams,
	})
}

// StartLogStreamHandler - Запуск лог-стрима через WebSocket
func (h *Handler) StartLogStreamHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	// Получаем параметры из запроса
	tailLines := int64(100)
	if tailStr := c.Query("tail"); tailStr != "" {
		if n, err := fmt.Sscanf(tailStr, "%d", &tailLines); err != nil || n != 1 {
			tailLines = 100
		}
	}

	follow := c.Query("follow") == "true"
	bufferSize := 100
	if bufStr := c.Query("buffer"); bufStr != "" {
		if n, err := fmt.Sscanf(bufStr, "%d", &bufferSize); err != nil || n != 1 {
			bufferSize = 100
		}
	}

	// Обновляем соединение до WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer ws.Close()

	// Создаем ID стрима
	streamID := fmt.Sprintf("%s-%s-%d", namespace, podName, time.Now().UnixNano())

	stream := &LogStream{
		ID:         streamID,
		Namespace:  namespace,
		Pod:        podName,
		Conn:       ws,
		StopChan:   make(chan struct{}),
		BufferSize: bufferSize,
		TailLines:  tailLines,
		Follow:     follow,
	}

	// Сохраняем стрим
	logStreamsMu.Lock()
	logStreams[streamID] = stream
	logStreamsMu.Unlock()

	// Удаляем стрим при завершении
	defer func() {
		logStreamsMu.Lock()
		delete(logStreams, streamID)
		logStreamsMu.Unlock()
		close(stream.StopChan)
		log.Printf("Log stream stopped: %s/%s", namespace, podName)
	}()

	log.Printf("Log stream started: %s/%s (follow: %v, tail: %d)",
		namespace, podName, follow, tailLines)

	// Отправляем начальное сообщение
	ws.WriteJSON(LogMessage{
		Type:    "info",
		Message: fmt.Sprintf("Log stream started for pod %s/%s", namespace, podName),
		Time:    time.Now().Format(time.RFC3339),
	})

	// Запускаем чтение логов
	err = h.streamPodLogs(stream, h.clientset)
	if err != nil {
		ws.WriteJSON(LogMessage{
			Type:    "error",
			Message: fmt.Sprintf("Error streaming logs: %v", err),
			Time:    time.Now().Format(time.RFC3339),
		})
	}
}

// streamPodLogs - Чтение и отправка логов через WebSocket
func (h *Handler) streamPodLogs(stream *LogStream, clientset *kubernetes.Clientset) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Отслеживаем закрытие WebSocket
	go func() {
		<-stream.StopChan
		cancel()
	}()

	// Получаем информацию о поде
	pod, err := clientset.CoreV1().Pods(stream.Namespace).Get(ctx, stream.Pod, metav1.GetOptions{})
	if err != nil {
		stream.Conn.WriteJSON(LogMessage{
			Type:    "error",
			Message: fmt.Sprintf("Pod not found: %v", err),
			Time:    time.Now().Format(time.RFC3339),
		})
		return err
	}

	// Отправляем информацию о поде
	stream.Conn.WriteJSON(LogMessage{
		Type: "info",
		Message: fmt.Sprintf("Pod: %s, Status: %s, Node: %s",
			pod.Name, pod.Status.Phase, pod.Spec.NodeName),
		Time: time.Now().Format(time.RFC3339),
	})

	// Если в поде несколько контейнеров, нужно указать конкретный контейнер
	// или получить логи из первого контейнера
	containerName := ""
	if len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	// Получаем логи с правильными параметрами
	podLogOpts := &corev1.PodLogOptions{
		Container:  containerName,
		TailLines:  &stream.TailLines,
		Follow:     stream.Follow,
		Timestamps: true, // Добавляем временные метки
	}

	log.Printf("Requesting logs for pod %s/%s, container: %s, follow: %v, tail: %d",
		stream.Namespace, stream.Pod, containerName, stream.Follow, stream.TailLines)

	req := clientset.CoreV1().Pods(stream.Namespace).GetLogs(stream.Pod, podLogOpts)

	podLogs, err := req.Stream(ctx)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get log stream: %v", err)
		log.Printf(errorMsg)

		// Попробуем получить логи без указания контейнера
		if containerName != "" {
			log.Printf("Trying without container name...")
			podLogOpts.Container = ""
			req = clientset.CoreV1().Pods(stream.Namespace).GetLogs(stream.Pod, podLogOpts)
			podLogs, err = req.Stream(ctx)

			if err != nil {
				stream.Conn.WriteJSON(LogMessage{
					Type:    "error",
					Message: errorMsg,
					Time:    time.Now().Format(time.RFC3339),
				})
				return err
			}
		} else {
			stream.Conn.WriteJSON(LogMessage{
				Type:    "error",
				Message: errorMsg,
				Time:    time.Now().Format(time.RFC3339),
			})
			return err
		}
	}
	defer podLogs.Close()

	// Сообщаем об успешном подключении
	stream.Conn.WriteJSON(LogMessage{
		Type:    "info",
		Message: "Successfully connected to pod logs",
		Time:    time.Now().Format(time.RFC3339),
	})

	// Используем Scanner для построчного чтения
	scanner := bufio.NewScanner(podLogs)

	// Увеличиваем максимальный размер буфера (логи могут быть длинными)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		select {
		case <-stream.StopChan:
			stream.Conn.WriteJSON(LogMessage{
				Type:    "info",
				Message: "Log stream stopped by user",
				Time:    time.Now().Format(time.RFC3339),
			})
			return nil

		case <-ctx.Done():
			return nil

		default:
			line := scanner.Text()

			// Отправляем строку лога
			err := stream.Conn.WriteJSON(LogMessage{
				Type:    "log",
				Message: line,
				Time:    time.Now().Format(time.RFC3339),
			})

			if err != nil {
				log.Printf("WebSocket write error: %v", err)
				return err
			}
		}
	}

	// Проверяем ошибку сканера
	if err := scanner.Err(); err != nil {
		if err == context.Canceled {
			return nil
		}

		log.Printf("Scanner error: %v", err)
		stream.Conn.WriteJSON(LogMessage{
			Type:    "error",
			Message: fmt.Sprintf("Error reading logs: %v", err),
			Time:    time.Now().Format(time.RFC3339),
		})
		return err
	}

	// Если мы здесь, значит сканер закончил (EOF)
	if stream.Follow {
		// В режиме follow это может означать, что под перезапустился
		stream.Conn.WriteJSON(LogMessage{
			Type:    "warning",
			Message: "Pod logs ended (pod might have restarted). Trying to reconnect...",
			Time:    time.Now().Format(time.RFC3339),
		})

		// Ждем и пытаемся переподключиться
		time.Sleep(2 * time.Second)
		return h.streamPodLogs(stream, clientset) // Рекурсивная переподключка
	}

	// Если не follow, просто завершаем
	stream.Conn.WriteJSON(LogMessage{
		Type:    "info",
		Message: "Log stream completed",
		Time:    time.Now().Format(time.RFC3339),
	})

	return nil
}

// StopLogStreamHandler - Остановка лог-стрима
func (h *Handler) StopLogStreamHandler(c *gin.Context) {
	streamID := c.Param("id")

	logStreamsMu.Lock()
	stream, exists := logStreams[streamID]
	logStreamsMu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log stream not found"})
		return
	}

	// Закрываем канал остановки
	close(stream.StopChan)

	// Удаляем из хранилища
	logStreamsMu.Lock()
	delete(logStreams, streamID)
	logStreamsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "Log stream stopped",
		"stream":  streamID,
	})
}

// WatchPodsHandler - WebSocket для отслеживания изменений подов в реальном времени
func (h *Handler) WatchPodsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer ws.Close()

	// Начальное сообщение
	ws.WriteJSON(LogMessage{
		Type:    "info",
		Message: fmt.Sprintf("Started watching pods in namespace: %s", namespace),
		Time:    time.Now().Format(time.RFC3339),
	})

	// Канал для остановки
	stopChan := make(chan struct{})
	defer close(stopChan)

	// Создаем watcher для подов
	watcher, err := h.clientset.CoreV1().Pods(namespace).Watch(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		ws.WriteJSON(LogMessage{
			Type:    "error",
			Message: fmt.Sprintf("Failed to create pod watcher: %v", err),
			Time:    time.Now().Format(time.RFC3339),
		})
		return
	}
	defer watcher.Stop()

	// Читаем события
	for {
		select {
		case <-stopChan:
			return

		case event, ok := <-watcher.ResultChan():
			if !ok {
				ws.WriteJSON(LogMessage{
					Type:    "warning",
					Message: "Pod watch channel closed",
					Time:    time.Now().Format(time.RFC3339),
				})
				return
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			// Отправляем событие
			ws.WriteJSON(map[string]interface{}{
				"type":      string(event.Type),
				"pod":       pod.Name,
				"namespace": pod.Namespace,
				"status":    string(pod.Status.Phase),
				"time":      time.Now().Format(time.RFC3339),
				"event": map[string]interface{}{
					"type":   event.Type,
					"object": pod,
				},
			})
		}
	}
}
