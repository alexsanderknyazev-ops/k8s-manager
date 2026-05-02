package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetHPAHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	if namespace == "all" {
		namespace = ""
	}

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	list, err := h.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, hpa := range list.Items {
		targetRef := ""
		if hpa.Spec.ScaleTargetRef.Kind != "" {
			targetRef = hpa.Spec.ScaleTargetRef.Kind + "/" + hpa.Spec.ScaleTargetRef.Name
		} else {
			targetRef = hpa.Spec.ScaleTargetRef.Name
		}
		minReplicas := int32(0)
		if hpa.Spec.MinReplicas != nil {
			minReplicas = *hpa.Spec.MinReplicas
		}
		maxReplicas := hpa.Spec.MaxReplicas
		current := int32(0)
		if hpa.Status.CurrentReplicas > 0 {
			current = hpa.Status.CurrentReplicas
		}
		desired := int32(0)
		if hpa.Status.DesiredReplicas > 0 {
			desired = hpa.Status.DesiredReplicas
		}
		result = append(result, gin.H{
			"name":          hpa.Name,
			"namespace":     hpa.Namespace,
			"target":        targetRef,
			"minReplicas":   minReplicas,
			"maxReplicas":   maxReplicas,
			"current":       current,
			"desired":       desired,
			"metrics":       formatHPAMetrics(hpa),
			"age":           time.Since(hpa.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"namespace": namespace, "count": len(result), "hpas": result})
}

func formatHPAMetrics(hpa autoscalingv2.HorizontalPodAutoscaler) []gin.H {
	var out []gin.H
	for _, m := range hpa.Spec.Metrics {
		switch m.Type {
		case autoscalingv2.ResourceMetricSourceType:
			if m.Resource != nil {
				out = append(out, gin.H{"type": "resource", "name": string(m.Resource.Name), "target": m.Resource.Target})
			}
		case autoscalingv2.PodsMetricSourceType:
			if m.Pods != nil {
				out = append(out, gin.H{"type": "pods", "metric": m.Pods.Metric, "target": m.Pods.Target})
			}
		case autoscalingv2.ObjectMetricSourceType:
			if m.Object != nil {
				out = append(out, gin.H{"type": "object", "metric": m.Object.Metric, "target": m.Object.Target})
			}
		default:
			out = append(out, gin.H{"type": string(m.Type)})
		}
	}
	return out
}

func (h *Handler) GetHPAYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	hpa, err := h.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	hpa.ManagedFields = nil
	yamlData, err := yaml.Marshal(hpa)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/yaml", yamlData)
}

func (h *Handler) CreateOrUpdateHPAHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	var body struct {
		MinReplicas *int32 `json:"minReplicas"`
		MaxReplicas *int32 `json:"maxReplicas"`
		TargetKind  string `json:"targetKind"`
		TargetName  string `json:"targetName"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TargetName == "" || body.MaxReplicas == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targetName and maxReplicas required"})
		return
	}
	if body.TargetKind == "" {
		body.TargetKind = "Deployment"
	}
	minRep := int32(1)
	if body.MinReplicas != nil && *body.MinReplicas >= 0 {
		minRep = *body.MinReplicas
	}

	hpa, err := h.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		// Create new
		hpa = &autoscalingv2.HorizontalPodAutoscaler{}
		hpa.APIVersion = "autoscaling/v2"
		hpa.Kind = "HorizontalPodAutoscaler"
		hpa.Name = name
		hpa.Namespace = namespace
		hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       body.TargetKind,
				Name:       body.TargetName,
				APIVersion: "apps/v1",
			},
			MinReplicas: &minRep,
			MaxReplicas: *body.MaxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: func() *int32 { x := int32(80); return &x }(),
						},
					},
				},
			},
		}
		hpa, err = h.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Create(c.Request.Context(), hpa, metav1.CreateOptions{})
	} else {
		hpa.Spec.MinReplicas = &minRep
		hpa.Spec.MaxReplicas = *body.MaxReplicas
		hpa, err = h.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(c.Request.Context(), hpa, metav1.UpdateOptions{})
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": hpa.Name})
}
