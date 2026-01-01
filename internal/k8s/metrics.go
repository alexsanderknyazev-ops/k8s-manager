package k8s

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type PodMetrics struct {
	Name          string
	Namespace     string
	CPUUsage      string
	MemoryUsage   string
	CPULimit      string
	MemoryLimit   string
	CPUPercent    int
	MemoryPercent int
	CPURaw        int64
	MemoryRaw     int64
	Timestamp     string
}

type NodeMetrics struct {
	Name              string
	CPUUsage          string
	MemoryUsage       string
	CPUAllocatable    string
	MemoryAllocatable string
	CPUPercent        int
	MemoryPercent     int
	CPURaw            int64
	MemoryRaw         int64
	Timestamp         string
}

type ClusterMetrics struct {
	TotalAllocatableCPU    int64
	TotalAllocatableMemory int64
	TotalUsedCPU           int64
	TotalUsedMemory        int64
	ClusterCPUPercent      int
	ClusterMemoryPercent   int
}

func GetPodMetrics(metricsClient *metricsv.Clientset, clientset *kubernetes.Clientset, namespace string) ([]PodMetrics, error) {
	if metricsClient == nil {
		return nil, fmt.Errorf("metrics client not initialized")
	}

	// Получаем метрики подов
	podMetricsList, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Получаем список подов для дополнительной информации
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Создаем map для быстрого поиска
	podMap := make(map[string]corev1.Pod)
	for _, pod := range pods.Items {
		podMap[pod.Name] = pod
	}

	var metrics []PodMetrics
	for _, pm := range podMetricsList.Items {
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
		memoryUsage := FormatBytes(totalMemory)

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
				memoryLimit = FormatBytes(memoryLimitValue)
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
					memoryLimit = FormatBytes(request.Value()) + " (request)"
				}
			}
		}

		// Исправляем форматирование времени
		timestampStr := ""
		if !pm.Timestamp.IsZero() {
			timestampStr = pm.Timestamp.Time.Format(time.RFC3339)
		}

		metrics = append(metrics, PodMetrics{
			Name:          pm.Name,
			Namespace:     pm.Namespace,
			CPUUsage:      cpuUsage,
			MemoryUsage:   memoryUsage,
			CPULimit:      cpuLimit,
			MemoryLimit:   memoryLimit,
			CPUPercent:    cpuPercent,
			MemoryPercent: memoryPercent,
			CPURaw:        totalCPU,
			MemoryRaw:     totalMemory,
			Timestamp:     timestampStr,
		})
	}

	return metrics, nil
}

func GetSinglePodMetrics(metricsClient *metricsv.Clientset, clientset *kubernetes.Clientset, namespace, podName string) (*metricsv1beta1.PodMetrics, *corev1.Pod, error) {
	// Получаем метрики пода
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	// Получаем информацию о поде
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	return podMetrics, pod, nil
}

func GetNodeMetrics(metricsClient *metricsv.Clientset, clientset *kubernetes.Clientset) ([]NodeMetrics, *ClusterMetrics, error) {
	// Получаем метрики нод
	nodeMetricsList, err := metricsClient.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	// Получаем информацию о нодах
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	// Создаем map для быстрого поиска
	nodeMap := make(map[string]corev1.Node)
	for _, node := range nodes.Items {
		nodeMap[node.Name] = node
	}

	var metrics []NodeMetrics
	var clusterMetrics ClusterMetrics

	for _, nm := range nodeMetricsList.Items {
		nodeInfo, exists := nodeMap[nm.Name]
		if !exists {
			continue
		}

		cpuUsage := nm.Usage.Cpu().MilliValue()
		memoryUsage := nm.Usage.Memory().Value()

		allocatableCPU := nodeInfo.Status.Allocatable.Cpu().MilliValue()
		allocatableMemory := nodeInfo.Status.Allocatable.Memory().Value()

		clusterMetrics.TotalAllocatableCPU += allocatableCPU
		clusterMetrics.TotalAllocatableMemory += allocatableMemory
		clusterMetrics.TotalUsedCPU += cpuUsage
		clusterMetrics.TotalUsedMemory += memoryUsage

		cpuPercent := 0
		memoryPercent := 0

		if allocatableCPU > 0 {
			cpuPercent = int(float64(cpuUsage) / float64(allocatableCPU) * 100)
		}

		if allocatableMemory > 0 {
			memoryPercent = int(float64(memoryUsage) / float64(allocatableMemory) * 100)
		}

		// Исправляем форматирование времени
		timestampStr := ""
		if !nm.Timestamp.IsZero() {
			timestampStr = nm.Timestamp.Time.Format(time.RFC3339)
		}

		metrics = append(metrics, NodeMetrics{
			Name:              nm.Name,
			CPUUsage:          fmt.Sprintf("%dm", cpuUsage),
			MemoryUsage:       FormatBytes(memoryUsage),
			CPUAllocatable:    fmt.Sprintf("%dm", allocatableCPU),
			MemoryAllocatable: FormatBytes(allocatableMemory),
			CPUPercent:        cpuPercent,
			MemoryPercent:     memoryPercent,
			CPURaw:            cpuUsage,
			MemoryRaw:         memoryUsage,
			Timestamp:         timestampStr,
		})
	}

	// Рассчитываем общее использование кластера
	if clusterMetrics.TotalAllocatableCPU > 0 {
		clusterMetrics.ClusterCPUPercent = int(float64(clusterMetrics.TotalUsedCPU) / float64(clusterMetrics.TotalAllocatableCPU) * 100)
	}

	if clusterMetrics.TotalAllocatableMemory > 0 {
		clusterMetrics.ClusterMemoryPercent = int(float64(clusterMetrics.TotalUsedMemory) / float64(clusterMetrics.TotalAllocatableMemory) * 100)
	}

	return metrics, &clusterMetrics, nil
}

func GetAllPodsMetrics(metricsClient *metricsv.Clientset, clientset *kubernetes.Clientset) ([]map[string]interface{}, int64, int64, error) {
	// Получаем все namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, 0, 0, err
	}

	allMetrics := []map[string]interface{}{}
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

			allMetrics = append(allMetrics, map[string]interface{}{
				"pod":        pm.Name,
				"namespace":  pm.Namespace,
				"cpu":        fmt.Sprintf("%dm", podCPU),
				"memory":     FormatBytes(podMemory),
				"cpu_raw":    podCPU,
				"memory_raw": podMemory,
			})
		}
	}

	// Сортируем по использованию CPU (по убыванию)
	sort.Slice(allMetrics, func(i, j int) bool {
		return allMetrics[i]["cpu_raw"].(int64) > allMetrics[j]["cpu_raw"].(int64)
	})

	return allMetrics, totalCPU, totalMemory, nil
}

func FormatBytes(bytes int64) string {
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

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}