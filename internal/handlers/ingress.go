package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gopkg.in/yaml.v3"
)

func (h *Handler) GetIngressHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	ingressList, err := h.clientset.NetworkingV1().Ingresses(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, ing := range ingressList.Items {
		hosts := []string{}
		for _, rule := range ing.Spec.Rules {
			if rule.Host != "" {
				hosts = append(hosts, rule.Host)
			}
		}
		tlsHosts := []string{}
		for _, t := range ing.Spec.TLS {
			tlsHosts = append(tlsHosts, t.Hosts...)
		}
		className := ""
		if ing.Spec.IngressClassName != nil {
			className = *ing.Spec.IngressClassName
		}
		result = append(result, gin.H{
			"name":       ing.Name,
			"namespace": ing.Namespace,
			"hosts":     hosts,
			"tls":       len(ing.Spec.TLS) > 0,
			"tlsHosts":  tlsHosts,
			"className": className,
			"age":       time.Since(ing.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"ingresses": result,
	})
}

func (h *Handler) GetIngressYAMLHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	ing, err := h.clientset.NetworkingV1().Ingresses(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ing.ManagedFields = nil
	ing.TypeMeta = metav1.TypeMeta{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
	}
	yamlData, err := yaml.Marshal(ing)
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
