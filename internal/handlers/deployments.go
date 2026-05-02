package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetDeploymentsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "market")
	if namespace == "all" {
		namespace = ""
	}
	limit := int64(500)
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.ParseInt(l, 10, 64); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
	}
	labelSelector := c.Query("labels")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	deployments, err := h.clientset.AppsV1().Deployments(namespace).List(c.Request.Context(), metav1.ListOptions{LabelSelector: labelSelector, Limit: limit})
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

func (h *Handler) CreateDeploymentHandler(c *gin.Context) {
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

	var deployment appsv1.Deployment
	if err := yaml.Unmarshal([]byte(request.YAML), &deployment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid YAML: " + err.Error()})
		return
	}

	namespace := deployment.Namespace
	if namespace == "" {
		namespace = "default"
		deployment.Namespace = namespace
	}

	_, err := h.clientset.AppsV1().Deployments(namespace).Create(c.Request.Context(), &deployment, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Deployment created successfully",
		"name":      deployment.Name,
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

func (h *Handler) RollbackDeploymentHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	deploymentName := c.Param("deployment")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	dep, err := h.clientset.AppsV1().Deployments(namespace).Get(c.Request.Context(), deploymentName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	selector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid selector"})
		return
	}
	rsList, err := h.clientset.AppsV1().ReplicaSets(namespace).List(c.Request.Context(), metav1.ListOptions{
		LabelSelector: selector.String(), Limit: 50,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Sort by revision (annotation deployment.kubernetes.io/revision), find previous
	type revRS struct {
		rev int64
		rs  *appsv1.ReplicaSet
	}
	var revs []revRS
	for i := range rsList.Items {
		rs := &rsList.Items[i]
		if rs.Annotations == nil {
			continue
		}
		r := int64(0)
		if s, ok := rs.Annotations["deployment.kubernetes.io/revision"]; ok && s != "" {
			if n, _ := strconv.ParseInt(s, 10, 64); n > 0 {
				r = n
			}
		}
		revs = append(revs, revRS{rev: r, rs: rs})
	}
	if len(revs) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no previous revision to rollback to"})
		return
	}
	sort.Slice(revs, func(i, j int) bool { return revs[i].rev > revs[j].rev })
	prev := revs[1].rs
	dep.Spec.Template = prev.Spec.Template
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string)
	}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = h.clientset.AppsV1().Deployments(namespace).Update(c.Request.Context(), dep, metav1.UpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "rollback initiated", "revision": prev.Annotations["deployment.kubernetes.io/revision"]})
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
