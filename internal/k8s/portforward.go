package k8s

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwardSession struct {
	ID         string    `json:"id"`
	Pod        string    `json:"pod"`
	Namespace  string    `json:"namespace"`
	LocalPort  int       `json:"localPort"`
	RemotePort int       `json:"remotePort"`
	Status     string    `json:"status"` // running, stopped, error
	CreatedAt  time.Time `json:"createdAt"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	URL        string    `json:"url"`
	StopChan   chan struct{}
}

type PortForwardManager struct {
	sessions map[string]*PortForwardSession
	mu       sync.RWMutex
}

var pfManager = &PortForwardManager{
	sessions: make(map[string]*PortForwardSession),
}

func GetPortForwardManager() *PortForwardManager {
	return pfManager
}

func (m *PortForwardManager) GetSessions() []*PortForwardSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*PortForwardSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (m *PortForwardManager) AddSession(session *PortForwardSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
}

func (m *PortForwardManager) RemoveSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

func (m *PortForwardManager) GetSession(id string) (*PortForwardSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, exists := m.sessions[id]
	return session, exists
}

func (m *PortForwardManager) StopSession(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return false
	}

	if session.StopChan != nil {
		close(session.StopChan)
	}

	return true
}

func IsPortInUse(port int) bool {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		conn.Close()
		return true
	}
	return false
}

func GenerateSessionID(namespace, pod string, remotePort, localPort int) string {
	return fmt.Sprintf("%s-%s-%d-%d-%d",
		namespace, pod, remotePort, localPort, time.Now().Unix())
}

func StartPortForward(session *PortForwardSession, clientset *kubernetes.Clientset) {
	log.Printf("🚀 Starting port-forward for pod %s/%s: %d -> %d",
		session.Namespace, session.Pod, session.LocalPort, session.RemotePort)

	session.Status = "running"
	session.StartedAt = time.Now()

	defer func() {
		session.Status = "stopped"
		close(session.StopChan)

		// Удаляем сессию из менеджера
		pfManager.RemoveSession(session.ID)

		log.Printf("🛑 Port-forward stopped for pod %s/%s", session.Namespace, session.Pod)
	}()

	// Получаем конфиг
	config, err := getK8sConfig()
	if err != nil {
		log.Printf("❌ Failed to get kubeconfig: %v", err)
		session.Status = "error"
		return
	}

	// Создаем round tripper для SPDY
	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		log.Printf("❌ Failed to create round tripper: %v", err)
		session.Status = "error"
		return
	}

	// URL для port-forward
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		session.Namespace, session.Pod)

	// Получаем хост из конфига
	hostURL, err := url.Parse(config.Host)
	if err != nil {
		log.Printf("❌ Failed to parse host URL: %v", err)
		session.Status = "error"
		return
	}

	// Создаем полный URL для порт-форвардинга
	serverURL := &url.URL{
		Scheme: hostURL.Scheme,
		Host:   hostURL.Host,
		Path:   path,
	}

	// Создаем dialer
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper},
		http.MethodPost, serverURL)

	// Порт для форвардинга
	ports := []string{fmt.Sprintf("%d:%d", session.LocalPort, session.RemotePort)}

	// Каналы для ошибок
	readyChan := make(chan struct{}, 1)

	// Запускаем port-forward
	pf, err := portforward.New(dialer, ports, session.StopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		log.Printf("❌ Failed to create port forward: %v", err)
		session.Status = "error"
		return
	}

	// Запускаем в горутине
	errChan := make(chan error, 1)
	go func() {
		errChan <- pf.ForwardPorts()
	}()

	// Ждем готовности
	select {
	case <-readyChan:
		log.Printf("✅ Port-forward ready: %s/%s %d->%d",
			session.Namespace, session.Pod, session.LocalPort, session.RemotePort)

		// Обновляем статус
		session.Status = "running"

	case err := <-errChan:
		log.Printf("❌ Port-forward error: %v", err)
		session.Status = "error"
		return

	case <-time.After(10 * time.Second):
		log.Printf("❌ Port-forward timeout")
		session.Status = "error"
		return
	}

	// Ждем остановки
	select {
	case err := <-errChan:
		if err != nil {
			log.Printf("❌ Port-forward stopped with error: %v", err)
			session.Status = "error"
		} else {
			log.Printf("ℹ️ Port-forward completed normally")
			session.Status = "stopped"
		}

	case <-session.StopChan:
		log.Printf("ℹ️ Port-forward manually stopped: %s/%s", session.Namespace, session.Pod)
		session.Status = "stopped"
	}
}

func getK8sConfig() (*rest.Config, error) {
	// Попробовать получить конфиг из кластера
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Использовать локальный kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
