package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetDeploymentsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "market")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	deployments, err := h.clientset.AppsV1().Deployments(namespace).List(c.Request.Context(), metav1.ListOptions{})
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

func (h *Handler) GetDeploymentYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	deployment, err := h.clientset.AppsV1().Deployments(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
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

func (h *Handler) UpdateDeploymentYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

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
	_, err := h.clientset.AppsV1().Deployments(namespace).Update(c.Request.Context(), &deployment, metav1.UpdateOptions{})
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

func (h *Handler) ScaleDeploymentHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	deploymentName := c.Param("deployment")
	replicasStr := c.DefaultQuery("replicas", "1")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	replicas, err := strconv.Atoi(replicasStr)
	if err != nil || replicas < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid replicas value"})
		return
	}

	deployment, err := h.clientset.AppsV1().Deployments(namespace).Get(
		c.Request.Context(), deploymentName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found: " + err.Error()})
		return
	}

	deployment.Spec.Replicas = int32Ptr(int32(replicas))
	_, err = h.clientset.AppsV1().Deployments(namespace).Update(
		c.Request.Context(), deployment, metav1.UpdateOptions{})
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

func (h *Handler) RestartDeploymentHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	deploymentName := c.Param("deployment")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	deployment, err := h.clientset.AppsV1().Deployments(namespace).Get(
		c.Request.Context(), deploymentName, metav1.GetOptions{})
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

	_, err = h.clientset.AppsV1().Deployments(namespace).Update(
		c.Request.Context(), deployment, metav1.UpdateOptions{})
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

func (h *Handler) DeleteDeploymentHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	deploymentName := c.Param("deployment")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	err := h.clientset.AppsV1().Deployments(namespace).Delete(
		c.Request.Context(), deploymentName, metav1.DeleteOptions{})
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

func int32Ptr(i int32) *int32 {
	return &i
}
