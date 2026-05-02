package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeployTemplatesHandler возвращает список шаблонов для быстрого деплоя.
func (h *Handler) DeployTemplatesHandler(c *gin.Context) {
	templates := []gin.H{
		{"id": "nginx", "name": "NGINX", "image": "nginx:alpine", "port": 80, "description": "Веб-сервер NGINX"},
		{"id": "redis", "name": "Redis", "image": "redis:alpine", "port": 6379, "description": "Кэш Redis"},
		{"id": "busybox", "name": "BusyBox", "image": "busybox:latest", "port": 80, "description": "Тестовый образ"},
		{"id": "postgres", "name": "PostgreSQL", "image": "postgres:15-alpine", "port": 5432, "description": "БД PostgreSQL"},
		{"id": "custom", "name": "Свой образ", "image": "", "port": 80, "description": "Укажите образ вручную"},
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// SimpleDeployRequest — тело запроса для простого деплоя.
type SimpleDeployRequest struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	Image         string `json:"image"`
	Replicas      int    `json:"replicas"`
	ContainerPort int    `json:"container_port"`
	CreateService bool   `json:"create_service"`
	ServicePort   int    `json:"service_port"`
}

// SimpleDeployHandler создаёт Deployment и опционально Service по упрощённым параметрам.
func (h *Handler) SimpleDeployHandler(c *gin.Context) {
	var req SimpleDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Name == "" || req.Image == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and image are required"})
		return
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}
	if req.Replicas <= 0 {
		req.Replicas = 1
	}
	if req.ContainerPort <= 0 {
		req.ContainerPort = 80
	}
	if req.ServicePort <= 0 {
		req.ServicePort = req.ContainerPort
	}

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	ctx := c.Request.Context()
	labels := map[string]string{"app": req.Name}
	replicas := int32(req.Replicas)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: req.Namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  req.Name,
							Image: req.Image,
							Ports: []corev1.ContainerPort{{ContainerPort: int32(req.ContainerPort), Name: "http"}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := h.clientset.AppsV1().Deployments(req.Namespace).Create(ctx, dep, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "deployment: " + err.Error()})
		return
	}

	created := []string{"Deployment/" + req.Name}
	if req.CreateService {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: req.Namespace},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports:    []corev1.ServicePort{{Port: int32(req.ServicePort), TargetPort: intstr.FromInt(req.ContainerPort), Name: "http"}},
				Type:     corev1.ServiceTypeClusterIP,
			},
		}
		_, err = h.clientset.CoreV1().Services(req.Namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"message":   "Deployment created; Service creation failed: " + err.Error(),
				"name":      req.Name,
				"namespace": req.Namespace,
				"created":   created,
			})
			return
		}
		created = append(created, "Service/"+req.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Deployment created successfully",
		"name":      req.Name,
		"namespace": req.Namespace,
		"created":   created,
	})
}
