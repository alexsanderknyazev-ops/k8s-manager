package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetPodsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "market")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Kubernetes client not initialized",
			"tip":   "Check if ~/.kube/config exists and is accessible",
		})
		return
	}

	pods, err := h.clientset.CoreV1().Pods(namespace).List(c.Request.Context(), metav1.ListOptions{})
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

func (h *Handler) GetLogsHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")
	tailLines := c.DefaultQuery("tail", "100")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Kubernetes client not initialized",
		})
		return
	}

	tail, err := strconv.ParseInt(tailLines, 10, 64)
	if err != nil {
		tail = 100
	}

	req := h.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tail,
	})

	logs, err := req.Do(c.Request.Context()).Raw()
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

func (h *Handler) DownloadLogsHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")
	tailLines := c.DefaultQuery("tail", "1000")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	tail, err := strconv.ParseInt(tailLines, 10, 64)
	if err != nil {
		tail = 1000
	}

	req := h.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tail,
	})

	logs, err := req.Do(c.Request.Context()).Raw()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Устанавливаем заголовки для скачивания
	c.Header("Content-Type", "text/plain")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s.log", podName, time.Now().Format("20060102-150405")))
	c.String(http.StatusOK, string(logs))
}

func (h *Handler) GetPodYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	pod, err := h.clientset.CoreV1().Pods(namespace).Get(c.Request.Context(), podName, metav1.GetOptions{})
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

func (h *Handler) UpdatePodYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	var request struct {
		YAML string `json:"yaml"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.clientset == nil {
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
	_, err := h.clientset.CoreV1().Pods(namespace).Update(c.Request.Context(), &pod, metav1.UpdateOptions{})
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

func (h *Handler) GetPodDetailsHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	pod, err := h.clientset.CoreV1().Pods(namespace).Get(c.Request.Context(), podName, metav1.GetOptions{})
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

	// Получаем статусы контейнеры
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

func (h *Handler) DeletePodHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	podName := c.Param("pod")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Kubernetes client not initialized",
		})
		return
	}

	err := h.clientset.CoreV1().Pods(namespace).Delete(
		c.Request.Context(), podName, metav1.DeleteOptions{})
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

func getRestartCount(pod corev1.Pod) int32 {
	total := int32(0)
	for _, status := range pod.Status.ContainerStatuses {
		total += status.RestartCount
	}
	return total
}
