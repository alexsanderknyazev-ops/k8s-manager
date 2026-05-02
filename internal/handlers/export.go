package handlers

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExportNamespaceHandler exports main namespace resources as YAML (documents separated by ---).
func (h *Handler) ExportNamespaceHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		namespace = c.DefaultQuery("namespace", "default")
	}
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	var out bytes.Buffer
	appendYAML := func(obj interface{}) {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return
		}
		if out.Len() > 0 {
			out.WriteString("---\n")
		}
		out.Write(data)
	}

	list, _ := h.clientset.AppsV1().Deployments(namespace).List(c.Request.Context(), metav1.ListOptions{})
	for i := range list.Items {
		list.Items[i].ManagedFields = nil
		appendYAML(&list.Items[i])
	}
	svcList, _ := h.clientset.CoreV1().Services(namespace).List(c.Request.Context(), metav1.ListOptions{})
	for i := range svcList.Items {
		svcList.Items[i].ManagedFields = nil
		appendYAML(&svcList.Items[i])
	}
	cmList, _ := h.clientset.CoreV1().ConfigMaps(namespace).List(c.Request.Context(), metav1.ListOptions{})
	for i := range cmList.Items {
		cmList.Items[i].ManagedFields = nil
		appendYAML(&cmList.Items[i])
	}
	secList, _ := h.clientset.CoreV1().Secrets(namespace).List(c.Request.Context(), metav1.ListOptions{})
	for i := range secList.Items {
		secList.Items[i].ManagedFields = nil
		appendYAML(&secList.Items[i])
	}

	c.Header("Content-Disposition", "attachment; filename="+namespace+"-export.yaml")
	c.Data(http.StatusOK, "application/yaml", out.Bytes())
}
