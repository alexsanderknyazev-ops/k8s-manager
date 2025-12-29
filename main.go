package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    "gopkg.in/yaml.v3"
)

var clientset *kubernetes.Clientset

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
// Получение метрик памяти (заглушка)
r.GET("/api/pod/metrics/:namespace/:pod", func(c *gin.Context) {
    namespace := c.Param("namespace")
    podName := c.Param("pod")
    
    // В реальном приложении нужно получать метрики из Metrics Server
    c.JSON(http.StatusOK, gin.H{
        "pod":        podName,
        "namespace":  namespace,
        "cpu":        "100m",
        "memory":     "128Mi",
        "timestamp":  time.Now().Format(time.RFC3339),
    })
})

// Получение событий пода
r.GET("/api/pod/events/:namespace/:pod", func(c *gin.Context) {
    namespace := c.Param("namespace")
    podName := c.Param("pod")
    
    if clientset == nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
        return
    }
    
    events, err := clientset.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
        FieldSelector: fmt.Sprintf("involvedObject.name=%s", podName),
    })
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    var result []gin.H
    for _, event := range events.Items {
        result = append(result, gin.H{
            "type":      event.Type,
            "reason":    event.Reason,
            "message":   event.Message,
            "count":     event.Count,
            "lastSeen":  event.LastTimestamp.Time.Format(time.RFC3339),
            "firstSeen": event.FirstTimestamp.Time.Format(time.RFC3339),
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "pod":    podName,
        "events": result,
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
        },
    })
}

func healthHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status":  "healthy",
        "service": "k8s-manager",
        "k8s":     clientset != nil,
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
        "pod":        podName,
        "namespace":  namespace,
        "yaml":       string(yamlData),
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
            "name":         dep.Name,
            "namespace":    dep.Namespace,
            "ready":        fmt.Sprintf("%d/%d", readyReplicas, *dep.Spec.Replicas),
            "ready_count":  readyReplicas,
            "total_count":  *dep.Spec.Replicas,
            "replicas":     *dep.Spec.Replicas,
            "age":          time.Since(dep.CreationTimestamp.Time).Round(time.Second).String(),
            "labels":       dep.Labels,
            "strategy":     string(dep.Spec.Strategy.Type),
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
        "message":     fmt.Sprintf("Deployment %s scaled to %d replicas", deploymentName, replicas),
        "deployment":  deploymentName,
        "replicas":    replicas,
        "namespace":   namespace,
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