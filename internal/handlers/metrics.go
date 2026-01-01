package handlers

import (
	// "fmt"
	// "net/http"

	// "k8s-manager/internal/k8s"

	// "github.com/gin-gonic/gin"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func (h *Handler) GetPodMetricsHandler(c *gin.Context) {
// 	namespace := c.Param("namespace")

// 	if h.clientset == nil || h.metricsClient == nil {
// 		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
// 		return
// 	}

// 	// Получаем метрики подов
// 	metrics, err := k8s.GetPodMetrics(h.metricsClient, h.clientset, namespace)
// 	if err != nil {
// 		// Если метрики недоступны, возвращаем заглушку
// 		c.JSON(http.StatusOK, gin.H{
// 			"namespace": namespace,
// 			"metrics":   []gin.H{},
// 			"error":     "Metrics not available: " + err.Error(),
// 			"tip":       "Install Metrics Server: kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml",
// 		})
// 		return
// 	}

// 	// Конвертируем в формат ответа
// 	var result []gin.H
// 	for _, m := range metrics {
// 		result = append(result, gin.H{
// 			"pod":            m.Name,
// 			"namespace":      m.Namespace,
// 			"cpu_usage":      m.CPUUsage,
// 			"memory_usage":   m.MemoryUsage,
// 			"cpu_limit":      m.CPULimit,
// 			"memory_limit":   m.MemoryLimit,
// 			"cpu_percent":    m.CPUPercent,
// 			"memory_percent": m.MemoryPercent,
// 			"cpu_raw":        m.CPURaw,
// 			"memory_raw":     m.MemoryRaw,
// 			"timestamp":      m.Timestamp,
// 		})
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"namespace": namespace,
// 		"count":     len(result),
// 		"metrics":   result,
// 	})
// }

// func (h *Handler) GetSinglePodMetricsHandler(c *gin.Context) {
// 	namespace := c.Param("namespace")
// 	podName := c.Param("pod")

// 	if h.clientset == nil || h.metricsClient == nil {
// 		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
// 		return
// 	}

// 	// Получаем метрики пода
// 	podMetrics, pod, err := k8s.GetSinglePodMetrics(h.metricsClient, h.clientset, namespace, podName)
// 	if err != nil {
// 		c.JSON(http.StatusOK, gin.H{
// 			"pod":       podName,
// 			"namespace": namespace,
// 			"error":     "Metrics not available: " + err.Error(),
// 			"cpu":       "N/A",
// 			"memory":    "N/A",
// 		})
// 		return
// 	}

// 	// Собираем метрики по контейнерам
// 	containerMetrics := []gin.H{}
// 	totalCPU := int64(0)
// 	totalMemory := int64(0)

// 	for _, container := range podMetrics.Containers {
// 		cpuUsage := container.Usage.Cpu().MilliValue()
// 		memoryUsage := container.Usage.Memory().Value()

// 		totalCPU += cpuUsage
// 		totalMemory += memoryUsage

// 		// Находим лимиты для этого контейнера
// 		cpuLimit := "N/A"
// 		memoryLimit := "N/A"
// 		cpuPercent := 0
// 		memoryPercent := 0

// 		for _, podContainer := range pod.Spec.Containers {
// 			if podContainer.Name == container.Name {
// 				if limit := podContainer.Resources.Limits.Cpu(); !limit.IsZero() {
// 					cpuLimitValue := limit.MilliValue()
// 					cpuLimit = fmt.Sprintf("%dm", cpuLimitValue)
// 					if cpuLimitValue > 0 {
// 						cpuPercent = int(float64(cpuUsage) / float64(cpuLimitValue) * 100)
// 					}
// 				}
// 				if limit := podContainer.Resources.Limits.Memory(); !limit.IsZero() {
// 					memoryLimitValue := limit.Value()
// 					memoryLimit = k8s.FormatBytes(memoryLimitValue)
// 					if memoryLimitValue > 0 {
// 						memoryPercent = int(float64(memoryUsage) / float64(memoryLimitValue) * 100)
// 					}
// 				}

// 				// Если нет лимитов, проверяем requests
// 				if cpuLimit == "N/A" {
// 					if request := podContainer.Resources.Requests.Cpu(); !request.IsZero() {
// 						cpuLimit = fmt.Sprintf("%dm (request)", request.MilliValue())
// 					}
// 				}

// 				if memoryLimit == "N/A" {
// 					if request := podContainer.Resources.Requests.Memory(); !request.IsZero() {
// 						memoryLimit = k8s.FormatBytes(request.Value()) + " (request)"
// 					}
// 				}
// 				break
// 			}
// 		}

// 		containerMetrics = append(containerMetrics, gin.H{
// 			"name":           container.Name,
// 			"cpu_usage":      fmt.Sprintf("%dm", cpuUsage),
// 			"memory_usage":   k8s.FormatBytes(memoryUsage),
// 			"cpu_limit":      cpuLimit,
// 			"memory_limit":   memoryLimit,
// 			"cpu_percent":    cpuPercent,
// 			"memory_percent": memoryPercent,
// 		})
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"pod":          podName,
// 		"namespace":    namespace,
// 		"total_cpu":    fmt.Sprintf("%dm", totalCPU),
// 		"total_memory": k8s.FormatBytes(totalMemory),
// 		"containers":   containerMetrics,
// 		"timestamp":    podMetrics.Timestamp.Format(metav1.RFC3339),
// 	})
// }

// func (h *Handler) GetAllPodsMetricsHandler(c *gin.Context) {
// 	if h.clientset == nil || h.metricsClient == nil {
// 		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
// 		return
// 	}

// 	// Получаем все метрики подов
// 	allMetrics, totalCPU, totalMemory, err := k8s.GetAllPodsMetrics(h.metricsClient, h.clientset)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	// Берем топ 10 по использованию CPU
// 	topConsumers := allMetrics
// 	if len(allMetrics) > 10 {
// 		topConsumers = allMetrics[:10]
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"total_pods":    len(allMetrics),
// 		"total_cpu":     fmt.Sprintf("%dm", totalCPU),
// 		"total_memory":  k8s.FormatBytes(totalMemory),
// 		"top_consumers": topConsumers,
// 		"all_metrics":   allMetrics,
// 	})
// }

// func (h *Handler) GetNodeMetricsHandler(c *gin.Context) {
// 	if h.clientset == nil || h.metricsClient == nil {
// 		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s or Metrics client not ready"})
// 		return
// 	}

// 	// Получаем метрики нод
// 	nodeMetrics, clusterMetrics, err := k8s.GetNodeMetrics(h.metricsClient, h.clientset)
// 	if err != nil {
// 		c.JSON(http.StatusOK, gin.H{
// 			"error": "Node metrics not available: " + err.Error(),
// 			"nodes": []gin.H{},
// 		})
// 		return
// 	}

// 	// Конвертируем в формат ответа
// 	var metrics []gin.H
// 	for _, nm := range nodeMetrics {
// 		metrics = append(metrics, gin.H{
// 			"name":               nm.Name,
// 			"cpu_usage":          nm.CPUUsage,
// 			"memory_usage":       nm.MemoryUsage,
// 			"cpu_allocatable":    nm.CPUAllocatable,
// 			"memory_allocatable": nm.MemoryAllocatable,
// 			"cpu_percent":        nm.CPUPercent,
// 			"memory_percent":     nm.MemoryPercent,
// 			"cpu_raw":            nm.CPURaw,
// 			"memory_raw":         nm.MemoryRaw,
// 			"timestamp":          nm.Timestamp,
// 		})
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"nodes": metrics,
// 		"cluster_usage": gin.H{
// 			"cpu_percent":              clusterMetrics.ClusterCPUPercent,
// 			"memory_percent":           clusterMetrics.ClusterMemoryPercent,
// 			"total_cpu_used":           fmt.Sprintf("%dm", clusterMetrics.TotalUsedCPU),
// 			"total_memory_used":        k8s.FormatBytes(clusterMetrics.TotalUsedMemory),
// 			"total_cpu_allocatable":    fmt.Sprintf("%dm", clusterMetrics.TotalAllocatableCPU),
// 			"total_memory_allocatable": k8s.FormatBytes(clusterMetrics.TotalAllocatableMemory),
// 			"total_cpu_used_raw":       clusterMetrics.TotalUsedCPU,
// 			"total_memory_used_raw":    clusterMetrics.TotalUsedMemory,
// 		},
// 	})
// }
