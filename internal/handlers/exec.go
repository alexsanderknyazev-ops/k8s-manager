package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// sizeQueue передаёт размер терминала в exec (TTY). Без него контейнер может получить 0x0 и не выводить данные (например ls).
type sizeQueue struct {
	ch chan *remotecommand.TerminalSize
}

func (q *sizeQueue) Next() *remotecommand.TerminalSize {
	t, ok := <-q.ch
	if !ok {
		return nil
	}
	return t
}

var execUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// PodExecWSHandler обрабатывает WebSocket для exec в под. Требует h.restConfig != nil.
// Query: namespace, pod, container (optional). Команда по умолчанию: /bin/sh.
func (h *Handler) PodExecWSHandler(c *gin.Context) {
	if h.restConfig == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "exec not configured"})
		return
	}
	namespace := c.Query("namespace")
	podName := c.Query("pod")
	container := c.Query("container")
	if namespace == "" || podName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and pod required"})
		return
	}

	conn, err := execUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	if container == "" {
		pod, err := h.clientset.CoreV1().Pods(namespace).Get(c.Request.Context(), podName, metav1.GetOptions{})
		if err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: pod not found\r\n"))
			return
		}
		if len(pod.Spec.Containers) > 0 {
			container = pod.Spec.Containers[0].Name
		}
	}
	if container == "" {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: no container\r\n"))
		return
	}

	req := h.clientset.CoreV1().RESTClient().Post().
		Resource("pods").Namespace(namespace).Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   []string{"/bin/sh", "-c", "TERM=xterm-256color exec /bin/sh"},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(h.restConfig, "POST", req.URL())
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()+"\r\n"))
		return
	}

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	// Очередь размера TTY: без неё контейнер получает 0x0 и команды вроде ls не выводят данные.
	sizeCh := make(chan *remotecommand.TerminalSize, 1)
	sizeCh <- &remotecommand.TerminalSize{Width: 80, Height: 24}
	sizeQ := &sizeQueue{ch: sizeCh}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer func() { _ = stdoutW.Close() }()
		buf := make([]byte, 4096)
		for {
			n, err := stdoutR.Read(buf)
			if n > 0 {
				if e := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); e != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		err := executor.StreamWithContext(c.Request.Context(), remotecommand.StreamOptions{
			Stdin:             stdinR,
			Stdout:            stdoutW,
			Stderr:            stdoutW,
			Tty:               true,
			TerminalSizeQueue: sizeQ,
		})
		_ = stdinW.Close()
		_ = stdoutW.Close()
		if err != nil {
			slog.Debug("exec stream ended", "err", err)
		}
	}()

	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if mt == websocket.TextMessage {
			var msg struct {
				Cols *uint16 `json:"cols"`
				Rows *uint16 `json:"rows"`
			}
			if json.Unmarshal(data, &msg) == nil && msg.Cols != nil && msg.Rows != nil && *msg.Cols > 0 && *msg.Rows > 0 {
				select {
				case sizeCh <- &remotecommand.TerminalSize{Width: *msg.Cols, Height: *msg.Rows}:
				default:
					// очередь полна, пропускаем
				}
				continue
			}
		}
		if mt == websocket.BinaryMessage || mt == websocket.TextMessage {
			if _, err := stdinW.Write(data); err != nil {
				break
			}
		}
	}
	close(sizeCh)
	_ = stdinW.Close()
	wg.Wait()
}
