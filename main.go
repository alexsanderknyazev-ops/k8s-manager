package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

var clientset *kubernetes.Clientset
var metricsClient *metricsv.Clientset

func main() {
	// Инициализируем клиент
	initK8s()

	r := gin.Default()

	// Загружаем HTML шаблоны
	r.LoadHTMLGlob("templates/*.html")

	// Статические файлы
	r.Static("/static", "./static")

	// CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// ===== UI ROUTES =====
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/dashboard")
	})

	r.GET("/ui", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/dashboard")
	})

	r.GET("/ui/dashboard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"Title": "Dashboard",
		})
	})

	r.GET("/ui/applications", func(c *gin.Context) {
		c.HTML(http.StatusOK, "applications.html", gin.H{
			"Title": "Applications",
		})
	})

	r.GET("/ui/pods", func(c *gin.Context) {
		c.HTML(http.StatusOK, "pods.html", gin.H{
			"Title": "Pods",
		})
	})

	r.GET("/ui/deployments", func(c *gin.Context) {
		c.HTML(http.StatusOK, "deployments.html", gin.H{
			"Title": "Deployments",
		})
	})

	r.GET("/ui/config", func(c *gin.Context) {
		c.HTML(http.StatusOK, "config.html", gin.H{
			"Title": "Configuration",
		})
	})

	// ===== API ROUTES =====

	// Health & Info
	r.GET("/api", homeHandler)
	r.GET("/api/health", healthHandler)
	r.GET("/api/test", testConnectionHandler)

	// Pods
	r.GET("/api/pods", getPodsHandler)
	r.GET("/api/logs/:namespace/:pod", getLogsHandler)
	r.GET("/api/logs/download/:namespace/:pod", downloadLogsHandler)
	r.GET("/api/pod/yaml/:namespace/:pod", getPodYAMLHandler)
	r.PUT("/api/pod/yaml/:namespace/:pod", updatePodYAMLHandler)
	r.DELETE("/api/pod/:namespace/:pod", deletePodHandler)
	r.GET("/api/pod/details/:namespace/:pod", getPodDetailsHandler)

	// Deployments
	r.GET("/api/deployments", getDeploymentsHandler)
	r.GET("/api/deployment/yaml/:namespace/:name", getDeploymentYAMLHandler)
	r.PUT("/api/deployment/yaml/:namespace/:name", updateDeploymentYAMLHandler)
	r.POST("/api/scale/:namespace/:deployment", scaleDeploymentHandler)
	r.POST("/api/restart/:namespace/:deployment", restartDeploymentHandler)
	r.DELETE("/api/deployment/:namespace/:deployment", deleteDeploymentHandler)

	// Applications
	r.GET("/api/applications", getApplicationsHandler)

	// Services
	r.GET("/api/services", getServicesHandler)
	r.GET("/api/service/yaml/:namespace/:name", getServiceYAMLHandler)

	// ConfigMaps & Secrets
	r.GET("/api/configmaps/:namespace", getConfigMapsHandler)
	r.GET("/api/configmap/yaml/:namespace/:name", getConfigMapYAMLHandler)
	r.GET("/api/secrets/:namespace", getSecretsHandler)

	// Namespaces & Nodes
	r.GET("/api/namespaces", getNamespacesHandler)
	r.GET("/api/nodes", getNodesHandler)

	// ===== METRICS API =====
	r.GET("/api/metrics/pods/:namespace", getPodMetricsHandler)
	r.GET("/api/metrics/pod/:namespace/:pod", getSinglePodMetricsHandler)
	r.GET("/api/metrics/all-pods", getAllPodsMetricsHandler)
	r.GET("/api/metrics/nodes", getNodeMetricsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 K8s Manager started on :%s", port)
	log.Printf("📊 Dashboard: http://localhost:%s/ui/dashboard", port)
	log.Printf("🚀 Applications: http://localhost:%s/ui/applications", port)
	log.Printf("🔧 Pods: http://localhost:%s/ui/pods", port)
	log.Printf("⚙️  Deployments: http://localhost:%s/ui/deployments", port)
	log.Printf("🛠️  Configuration: http://localhost:%s/ui/config", port)
	log.Printf("📚 API: http://localhost:%s/api", port)

	r.Run(":" + port)
}

// ===== ИНИЦИАЛИЗАЦИЯ =====
func initK8s() {
	log.Println("🔧 Initializing Kubernetes client...")

	var config *rest.Config
	var err error

	// Способ 1: Попробовать получить конфиг из кластера
	config, err = rest.InClusterConfig()
	if err != nil {
		log.Println("ℹ️  Not running inside Kubernetes cluster, trying local kubeconfig...")

		// Способ 2: Использовать локальный kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
			log.Printf("ℹ️  Using default kubeconfig: %s", kubeconfig)
		}

		// Проверить существует ли файл
		if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
			log.Printf("❌ Kubeconfig not found at: %s", kubeconfig)
			log.Println("💡 Run: minikube kubectl -- config view > ~/.kube/config")
			return
		}

		// Загрузить конфиг из файла
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Printf("❌ Failed to build config from kubeconfig: %v", err)
			return
		}
	}

	// Создать клиент
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("❌ Failed to create clientset: %v", err)
		return
	}

	// Создать клиент для метрик
	metricsClient, err = metricsv.NewForConfig(config)
	if err != nil {
		log.Printf("⚠️  Failed to create metrics client: %v", err)
		log.Println("ℹ️  Make sure Metrics Server is installed: kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml")
	} else {
		log.Println("📊 Metrics client initialized")
	}

	log.Println("✅ Kubernetes client initialized successfully")

	// Тест подключения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("⚠️  Connection test failed: %v", err)
	} else {
		log.Println("🔗 Successfully connected to Kubernetes API")
	}
}

// ===== METRICS HANDLERS =====

// Получение метрик всех подов в namespace
func getPodMetricsHandler(c *gin.Context) {
	namespace := c.Param("namespace")

	if clientset == nil || metricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
		return
	}

	// Получаем метрики подов
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		// Если метрики недоступны, возвращаем заглушку
		c.JSON(http.StatusOK, gin.H{
			"namespace": namespace,
			"metrics":   []gin.H{},
			"error":     "Metrics not available: " + err.Error(),
			"tip":       "Install Metrics Server: kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml",
		})
		return
	}

	// Получаем список подов для дополнительной информации
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Создаем map для быстрого поиска
	podMap := make(map[string]corev1.Pod)
	for _, pod := range pods.Items {
		podMap[pod.Name] = pod
	}

	var metrics []gin.H
	for _, pm := range podMetrics.Items {
		podInfo, exists := podMap[pm.Name]
		if !exists {
			continue
		}

		// Суммируем метрики по контейнерам
		totalCPU := int64(0)
		totalMemory := int64(0)

		for _, container := range pm.Containers {
			totalCPU += container.Usage.Cpu().MilliValue()
			totalMemory += container.Usage.Memory().Value()
		}

		// Форматируем метрики
		cpuUsage := fmt.Sprintf("%dm", totalCPU)
		memoryUsage := formatBytes(totalMemory)

		// Получаем лимиты из спецификации пода
		cpuLimit := "N/A"
		memoryLimit := "N/A"
		cpuPercent := 0
		memoryPercent := 0

		if len(podInfo.Spec.Containers) > 0 {
			// CPU лимиты
			if limit := podInfo.Spec.Containers[0].Resources.Limits.Cpu(); !limit.IsZero() {
				cpuLimitValue := limit.MilliValue()
				cpuLimit = fmt.Sprintf("%dm", cpuLimitValue)
				if cpuLimitValue > 0 {
					cpuPercent = int(float64(totalCPU) / float64(cpuLimitValue) * 100)
				}
			}

			// Memory лимиты
			if limit := podInfo.Spec.Containers[0].Resources.Limits.Memory(); !limit.IsZero() {
				memoryLimitValue := limit.Value()
				memoryLimit = formatBytes(memoryLimitValue)
				if memoryLimitValue > 0 {
					memoryPercent = int(float64(totalMemory) / float64(memoryLimitValue) * 100)
				}
			}

			// Если нет лимитов, проверяем requests
			if cpuLimit == "N/A" {
				if request := podInfo.Spec.Containers[0].Resources.Requests.Cpu(); !request.IsZero() {
					cpuLimit = fmt.Sprintf("%dm (request)", request.MilliValue())
				}
			}

			if memoryLimit == "N/A" {
				if request := podInfo.Spec.Containers[0].Resources.Requests.Memory(); !request.IsZero() {
					memoryLimit = formatBytes(request.Value()) + " (request)"
				}
			}
		}

		metrics = append(metrics, gin.H{
			"pod":            pm.Name,
			"namespace":      pm.Namespace,
			"cpu_usage":      cpuUsage,
			"memory_usage":   memoryUsage,
			"cpu_limit":      cpuLimit,
			"memory_limit":   memoryLimit,
			"cpu_percent":    cpuPercent,
			"memory_percent": memoryPercent,
			"cpu_raw":        totalCPU,
			"memory_raw":     totalMemory,
			"timestamp":      pm.Timestamp.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(metrics),
		"metrics":   metrics,
	})
}

// Получение метрик конкретного пода
func getSinglePodMetricsHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	if clientset == nil || metricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
		return
	}

	// Получаем метрики пода
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"pod":       podName,
			"namespace": namespace,
			"error":     "Metrics not available: " + err.Error(),
			"cpu":       "N/A",
			"memory":    "N/A",
		})
		return
	}

	// Получаем информацию о поде
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	// Собираем метрики по контейнерам
	containerMetrics := []gin.H{}
	totalCPU := int64(0)
	totalMemory := int64(0)

	for _, container := range podMetrics.Containers {
		cpuUsage := container.Usage.Cpu().MilliValue()
		memoryUsage := container.Usage.Memory().Value()

		totalCPU += cpuUsage
		totalMemory += memoryUsage

		// Находим лимиты для этого контейнера
		cpuLimit := "N/A"
		memoryLimit := "N/A"
		cpuPercent := 0
		memoryPercent := 0

		for _, podContainer := range pod.Spec.Containers {
			if podContainer.Name == container.Name {
				if limit := podContainer.Resources.Limits.Cpu(); !limit.IsZero() {
					cpuLimitValue := limit.MilliValue()
					cpuLimit = fmt.Sprintf("%dm", cpuLimitValue)
					if cpuLimitValue > 0 {
						cpuPercent = int(float64(cpuUsage) / float64(cpuLimitValue) * 100)
					}
				}
				if limit := podContainer.Resources.Limits.Memory(); !limit.IsZero() {
					memoryLimitValue := limit.Value()
					memoryLimit = formatBytes(memoryLimitValue)
					if memoryLimitValue > 0 {
						memoryPercent = int(float64(memoryUsage) / float64(memoryLimitValue) * 100)
					}
				}

				// Если нет лимитов, проверяем requests
				if cpuLimit == "N/A" {
					if request := podContainer.Resources.Requests.Cpu(); !request.IsZero() {
						cpuLimit = fmt.Sprintf("%dm (request)", request.MilliValue())
					}
				}

				if memoryLimit == "N/A" {
					if request := podContainer.Resources.Requests.Memory(); !request.IsZero() {
						memoryLimit = formatBytes(request.Value()) + " (request)"
					}
				}
				break
			}
		}

		containerMetrics = append(containerMetrics, gin.H{
			"name":           container.Name,
			"cpu_usage":      fmt.Sprintf("%dm", cpuUsage),
			"memory_usage":   formatBytes(memoryUsage),
			"cpu_limit":      cpuLimit,
			"memory_limit":   memoryLimit,
			"cpu_percent":    cpuPercent,
			"memory_percent": memoryPercent,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"pod":          podName,
		"namespace":    namespace,
		"total_cpu":    fmt.Sprintf("%dm", totalCPU),
		"total_memory": formatBytes(totalMemory),
		"containers":   containerMetrics,
		"timestamp":    podMetrics.Timestamp.Format(time.RFC3339),
	})
}

// Получение метрик всех подов в кластере
func getAllPodsMetricsHandler(c *gin.Context) {
	if clientset == nil || metricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
		return
	}

	// Получаем все namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	allMetrics := []gin.H{}
	totalCPU := int64(0)
	totalMemory := int64(0)

	for _, ns := range namespaces.Items {
		// Пропускаем системные namespace по желанию
		if ns.Name == "kube-system" {
			continue
		}

		podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			continue // Пропускаем namespaces без метрик
		}

		for _, pm := range podMetrics.Items {
			podCPU := int64(0)
			podMemory := int64(0)

			for _, container := range pm.Containers {
				podCPU += container.Usage.Cpu().MilliValue()
				podMemory += container.Usage.Memory().Value()
			}

			totalCPU += podCPU
			totalMemory += podMemory

			allMetrics = append(allMetrics, gin.H{
				"pod":        pm.Name,
				"namespace":  pm.Namespace,
				"cpu":        fmt.Sprintf("%dm", podCPU),
				"memory":     formatBytes(podMemory),
				"cpu_raw":    podCPU,
				"memory_raw": podMemory,
			})
		}
	}

	// Сортируем по использованию CPU (по убыванию)
	sort.Slice(allMetrics, func(i, j int) bool {
		return allMetrics[i]["cpu_raw"].(int64) > allMetrics[j]["cpu_raw"].(int64)
	})

	c.JSON(http.StatusOK, gin.H{
		"total_pods":    len(allMetrics),
		"total_cpu":     fmt.Sprintf("%dm", totalCPU),
		"total_memory":  formatBytes(totalMemory),
		"top_consumers": allMetrics[:min(10, len(allMetrics))], // Топ 10
		"all_metrics":   allMetrics,
	})
}

// Получение метрик нод
func getNodeMetricsHandler(c *gin.Context) {
	if clientset == nil || metricsClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
		return
	}

	// Получаем метрики нод
	nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": "Node metrics not available: " + err.Error(),
			"nodes": []gin.H{},
		})
		return
	}

	// Получаем информацию о нодах
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Создаем map для быстрого поиска
	nodeMap := make(map[string]corev1.Node)
	for _, node := range nodes.Items {
		nodeMap[node.Name] = node
	}

	var metrics []gin.H
	totalAllocatableCPU := int64(0)
	totalAllocatableMemory := int64(0)
	totalUsedCPU := int64(0)
	totalUsedMemory := int64(0)

	for _, nm := range nodeMetrics.Items {
		nodeInfo, exists := nodeMap[nm.Name]
		if !exists {
			continue
		}

		cpuUsage := nm.Usage.Cpu().MilliValue()
		memoryUsage := nm.Usage.Memory().Value()

		allocatableCPU := nodeInfo.Status.Allocatable.Cpu().MilliValue()
		allocatableMemory := nodeInfo.Status.Allocatable.Memory().Value()

		totalAllocatableCPU += allocatableCPU
		totalAllocatableMemory += allocatableMemory
		totalUsedCPU += cpuUsage
		totalUsedMemory += memoryUsage

		cpuPercent := 0
		memoryPercent := 0

		if allocatableCPU > 0 {
			cpuPercent = int(float64(cpuUsage) / float64(allocatableCPU) * 100)
		}

		if allocatableMemory > 0 {
			memoryPercent = int(float64(memoryUsage) / float64(allocatableMemory) * 100)
		}

		metrics = append(metrics, gin.H{
			"name":               nm.Name,
			"cpu_usage":          fmt.Sprintf("%dm", cpuUsage),
			"memory_usage":       formatBytes(memoryUsage),
			"cpu_allocatable":    fmt.Sprintf("%dm", allocatableCPU),
			"memory_allocatable": formatBytes(allocatableMemory),
			"cpu_percent":        cpuPercent,
			"memory_percent":     memoryPercent,
			"cpu_raw":            cpuUsage,
			"memory_raw":         memoryUsage,
			"timestamp":          nm.Timestamp.Format(time.RFC3339),
		})
	}

	// Рассчитываем общее использование кластера
	clusterCPUPercent := 0
	clusterMemoryPercent := 0

	if totalAllocatableCPU > 0 {
		clusterCPUPercent = int(float64(totalUsedCPU) / float64(totalAllocatableCPU) * 100)
	}

	if totalAllocatableMemory > 0 {
		clusterMemoryPercent = int(float64(totalUsedMemory) / float64(totalAllocatableMemory) * 100)
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": metrics,
		"cluster_usage": gin.H{
			"cpu_percent":              clusterCPUPercent,
			"memory_percent":           clusterMemoryPercent,
			"total_cpu_used":           fmt.Sprintf("%dm", totalUsedCPU),
			"total_memory_used":        formatBytes(totalUsedMemory),
			"total_cpu_allocatable":    fmt.Sprintf("%dm", totalAllocatableCPU),
			"total_memory_allocatable": formatBytes(totalAllocatableMemory),
			"total_cpu_used_raw":       totalUsedCPU,
			"total_memory_used_raw":    totalUsedMemory,
		},
	})
}

// Вспомогательная функция для форматирования байтов
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2fGi", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2fMi", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2fKi", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ===== ОСНОВНЫЕ ОБРАБОТЧИКИ =====
func homeHandler(c *gin.Context) {
	status := "disconnected"
	if clientset != nil {
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
		},
	})
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "k8s-manager",
		"k8s":     clientset != nil,
		"metrics": metricsClient != nil,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func testConnectionHandler(c *gin.Context) {
	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"connected": false,
			"error":     "Kubernetes client not initialized",
		})
		return
	}

	_, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
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

// ===== APPLICATIONS =====
func getApplicationsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "all")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	// Получаем деплойменты
	var deployments *appsv1.DeploymentList
	var err error

	if namespace == "all" {
		deployments, err = clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
	} else {
		deployments, err = clientset.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, dep := range deployments.Items {
		readyReplicas := int32(0)
		if dep.Status.ReadyReplicas > 0 {
			readyReplicas = dep.Status.ReadyReplicas
		}

		result = append(result, gin.H{
			"name":        dep.Name,
			"namespace":   dep.Namespace,
			"type":        "Deployment",
			"instances":   *dep.Spec.Replicas,
			"ready":       fmt.Sprintf("%d/%d", readyReplicas, *dep.Spec.Replicas),
			"ready_count": readyReplicas,
			"total_count": *dep.Spec.Replicas,
			"age":         time.Since(dep.CreationTimestamp.Time).Round(time.Second).String(),
			"labels":      dep.Labels,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace":    namespace,
		"count":        len(result),
		"applications": result,
	})
}

// ===== PODS =====
func getPodsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "market")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Kubernetes client not initialized",
			"tip":   "Check if ~/.kube/config exists and is accessible",
		})
		return
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, pod := range pods.Items {
		ready := 0
		total := len(pod.Spec.Containers)
		for _, status := range pod.Status.ContainerStatuses {
			if status.Ready {
				ready++
			}
		}

		result = append(result, gin.H{
			"name":      pod.Name,
			"namespace": pod.Namespace,
			"status":    pod.Status.Phase,
			"ready":     fmt.Sprintf("%d/%d", ready, total),
			"restarts":  getRestartCount(pod),
			"age":       time.Since(pod.CreationTimestamp.Time).Round(time.Second).String(),
			"ip":        pod.Status.PodIP,
			"node":      pod.Spec.NodeName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(pods.Items),
		"pods":      result,
	})
}

func getLogsHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")
	tailLines := c.DefaultQuery("tail", "100")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Kubernetes client not initialized",
		})
		return
	}

	tail, err := strconv.ParseInt(tailLines, 10, 64)
	if err != nil {
		tail = 100
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tail,
	})

	logs, err := req.Do(context.TODO()).Raw()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"pod":   podName,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pod":        podName,
		"namespace":  namespace,
		"logs":       string(logs),
		"tail_lines": tail,
	})
}

func downloadLogsHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")
	tailLines := c.DefaultQuery("tail", "1000")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	tail, err := strconv.ParseInt(tailLines, 10, 64)
	if err != nil {
		tail = 1000
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tail,
	})

	logs, err := req.Do(context.TODO()).Raw()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Устанавливаем заголовки для скачивания
	c.Header("Content-Type", "text/plain")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s.log", podName, time.Now().Format("20060102-150405")))
	c.String(http.StatusOK, string(logs))
}

func getPodYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Конвертируем в YAML
	pod.ManagedFields = nil
	pod.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Pod",
	}

	yamlData, err := yaml.Marshal(pod)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pod":       podName,
		"namespace": namespace,
		"yaml":      string(yamlData),
	})
}

func updatePodYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	var request struct {
		YAML string `json:"yaml"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	// Декодируем YAML
	var pod corev1.Pod
	if err := yaml.Unmarshal([]byte(request.YAML), &pod); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid YAML: " + err.Error()})
		return
	}

	// Проверяем, что имя и namespace совпадают
	if pod.Name != podName || pod.Namespace != namespace {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name or namespace mismatch"})
		return
	}

	// Обновляем под
	_, err := clientset.CoreV1().Pods(namespace).Update(context.TODO(), &pod, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Pod updated successfully",
		"pod":       podName,
		"namespace": namespace,
	})
}

func getPodDetailsHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Получаем контейнеры
	containers := []gin.H{}
	for _, container := range pod.Spec.Containers {
		containers = append(containers, gin.H{
			"name":      container.Name,
			"image":     container.Image,
			"ports":     container.Ports,
			"resources": container.Resources,
		})
	}

	// Получаем статусы контейнеров
	containerStatuses := []gin.H{}
	for _, status := range pod.Status.ContainerStatuses {
		containerStatuses = append(containerStatuses, gin.H{
			"name":         status.Name,
			"ready":        status.Ready,
			"restartCount": status.RestartCount,
			"state":        status.State,
			"image":        status.Image,
			"imageID":      status.ImageID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"pod":               podName,
		"namespace":         namespace,
		"metadata":          pod.ObjectMeta,
		"spec":              pod.Spec,
		"status":            pod.Status,
		"containers":        containers,
		"containerStatuses": containerStatuses,
		"nodeName":          pod.Spec.NodeName,
		"podIP":             pod.Status.PodIP,
		"hostIP":            pod.Status.HostIP,
		"startTime":         pod.Status.StartTime,
		"conditions":        pod.Status.Conditions,
	})
}

func deletePodHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Kubernetes client not initialized",
		})
		return
	}

	err := clientset.CoreV1().Pods(namespace).Delete(
		context.TODO(), podName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Pod deleted successfully",
		"pod":       podName,
		"namespace": namespace,
	})
}

// ===== DEPLOYMENTS =====
func getDeploymentsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "market")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	deployments, err := clientset.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, dep := range deployments.Items {
		readyReplicas := int32(0)
		if dep.Status.ReadyReplicas > 0 {
			readyReplicas = dep.Status.ReadyReplicas
		}

		result = append(result, gin.H{
			"name":        dep.Name,
			"namespace":   dep.Namespace,
			"ready":       fmt.Sprintf("%d/%d", readyReplicas, *dep.Spec.Replicas),
			"ready_count": readyReplicas,
			"total_count": *dep.Spec.Replicas,
			"replicas":    *dep.Spec.Replicas,
			"age":         time.Since(dep.CreationTimestamp.Time).Round(time.Second).String(),
			"labels":      dep.Labels,
			"strategy":    string(dep.Spec.Strategy.Type),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace":   namespace,
		"count":       len(deployments.Items),
		"deployments": result,
	})
}

func getDeploymentYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Конвертируем в YAML
	deployment.ManagedFields = nil
	deployment.TypeMeta = metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	}

	yamlData, err := yaml.Marshal(deployment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":      name,
		"namespace": namespace,
		"yaml":      string(yamlData),
	})
}

func updateDeploymentYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	var request struct {
		YAML string `json:"yaml"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	// Декодируем YAML
	var deployment appsv1.Deployment
	if err := yaml.Unmarshal([]byte(request.YAML), &deployment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid YAML: " + err.Error()})
		return
	}

	// Проверяем имя
	if deployment.Name != name || deployment.Namespace != namespace {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name or namespace mismatch"})
		return
	}

	// Обновляем деплоймент
	_, err := clientset.AppsV1().Deployments(namespace).Update(context.TODO(), &deployment, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Deployment updated successfully",
		"name":      name,
		"namespace": namespace,
	})
}

func scaleDeploymentHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	deploymentName := c.Param("deployment")
	replicasStr := c.DefaultQuery("replicas", "1")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	replicas, err := strconv.Atoi(replicasStr)
	if err != nil || replicas < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid replicas value"})
		return
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Get(
		context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found: " + err.Error()})
		return
	}

	deployment.Spec.Replicas = int32Ptr(int32(replicas))
	_, err = clientset.AppsV1().Deployments(namespace).Update(
		context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    fmt.Sprintf("Deployment %s scaled to %d replicas", deploymentName, replicas),
		"deployment": deploymentName,
		"replicas":   replicas,
		"namespace":  namespace,
	})
}

func restartDeploymentHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	deploymentName := c.Param("deployment")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Get(
		context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	// Добавляем аннотацию для рестарта
	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] =
		time.Now().Format(time.RFC3339)

	_, err = clientset.AppsV1().Deployments(namespace).Update(
		context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    fmt.Sprintf("Deployment %s restarted", deploymentName),
		"deployment": deploymentName,
		"time":       time.Now().Format(time.RFC3339),
	})
}

func deleteDeploymentHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	deploymentName := c.Param("deployment")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	err := clientset.AppsV1().Deployments(namespace).Delete(
		context.TODO(), deploymentName, metav1.DeleteOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Deployment deleted successfully",
		"deployment": deploymentName,
		"namespace":  namespace,
	})
}

// ===== SERVICES =====
func getServicesHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	services, err := clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, svc := range services.Items {
		ports := []string{}
		for _, port := range svc.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
		}

		result = append(result, gin.H{
			"name":      svc.Name,
			"namespace": svc.Namespace,
			"type":      string(svc.Spec.Type),
			"ports":     ports,
			"clusterIP": svc.Spec.ClusterIP,
			"age":       time.Since(svc.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(services.Items),
		"services":  result,
	})
}

func getServiceYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	service, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	service.ManagedFields = nil
	service.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}

	yamlData, err := yaml.Marshal(service)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":      name,
		"namespace": namespace,
		"yaml":      string(yamlData),
	})
}

// ===== CONFIGMAPS =====
func getConfigMapsHandler(c *gin.Context) {
	namespace := c.Param("namespace")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	configmaps, err := clientset.CoreV1().ConfigMaps(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, cm := range configmaps.Items {
		result = append(result, gin.H{
			"name":      cm.Name,
			"namespace": cm.Namespace,
			"data":      len(cm.Data),
			"age":       time.Since(cm.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace":  namespace,
		"count":      len(result),
		"configmaps": result,
	})
}

func getConfigMapYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	configmap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	configmap.ManagedFields = nil
	configmap.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	}

	yamlData, err := yaml.Marshal(configmap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":      name,
		"namespace": namespace,
		"yaml":      string(yamlData),
	})
}

// ===== SECRETS =====
func getSecretsHandler(c *gin.Context) {
	namespace := c.Param("namespace")

	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	secrets, err := clientset.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, secret := range secrets.Items {
		result = append(result, gin.H{
			"name":      secret.Name,
			"namespace": secret.Namespace,
			"type":      string(secret.Type),
			"age":       time.Since(secret.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"secrets":   result,
	})
}

// ===== NAMESPACES =====
func getNamespacesHandler(c *gin.Context) {
	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
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

// ===== NODES =====
func getNodesHandler(c *gin.Context) {
	if clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, node := range nodes.Items {
		conditions := []string{}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
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

// ===== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ =====
func getRestartCount(pod corev1.Pod) int32 {
	total := int32(0)
	for _, status := range pod.Status.ContainerStatuses {
		total += status.RestartCount
	}
	return total
}

func int32Ptr(i int32) *int32 { return &i }
