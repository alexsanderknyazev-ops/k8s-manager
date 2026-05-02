package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetPVCHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	pvcList, err := h.clientset.CoreV1().PersistentVolumeClaims(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, pvc := range pvcList.Items {
		capacity := ""
		if req, ok := pvc.Status.Capacity["storage"]; ok {
			q := req
			capacity = (&q).String()
		}
		if capacity == "" && len(pvc.Spec.Resources.Requests) > 0 {
			if req, ok := pvc.Spec.Resources.Requests["storage"]; ok {
				q := req
				capacity = (&q).String()
			}
		}
		storageClass := ""
		if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
			storageClass = *pvc.Spec.StorageClassName
		}
		result = append(result, gin.H{
			"name":          pvc.Name,
			"namespace":     pvc.Namespace,
			"status":        string(pvc.Status.Phase),
			"capacity":      capacity,
			"storageClass":  storageClass,
			"volume":        pvc.Spec.VolumeName,
			"age":           time.Since(pvc.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"pvcs":      result,
	})
}
