package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetCronJobsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	list, err := h.clientset.BatchV1().CronJobs(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, cj := range list.Items {
		lastSchedule := ""
		if cj.Status.LastSuccessfulTime != nil {
			lastSchedule = cj.Status.LastSuccessfulTime.Format(time.RFC3339)
		}
		schedule := ""
		if cj.Spec.Schedule != "" {
			schedule = cj.Spec.Schedule
		}
		active := len(cj.Status.Active)
		result = append(result, gin.H{
			"name":          cj.Name,
			"namespace":     cj.Namespace,
			"schedule":      schedule,
			"lastSchedule":  lastSchedule,
			"active":        active,
			"suspend":       cj.Spec.Suspend != nil && *cj.Spec.Suspend,
			"age":           time.Since(cj.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"cronjobs":  result,
	})
}

func (h *Handler) GetJobsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	list, err := h.clientset.BatchV1().Jobs(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, j := range list.Items {
		succeeded := int32(0)
		if j.Status.Succeeded > 0 {
			succeeded = j.Status.Succeeded
		}
		failed := int32(0)
		if j.Status.Failed > 0 {
			failed = j.Status.Failed
		}
		completionTime := ""
		if j.Status.CompletionTime != nil {
			completionTime = j.Status.CompletionTime.Format(time.RFC3339)
		}
		result = append(result, gin.H{
			"name":           j.Name,
			"namespace":      j.Namespace,
			"completions":    succeeded + failed,
			"succeeded":      succeeded,
			"failed":         failed,
			"completionTime": completionTime,
			"age":            time.Since(j.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"jobs":      result,
	})
}

func (h *Handler) CreateJobFromCronJobHandler(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	cronJob, err := h.clientset.BatchV1().CronJobs(namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cronJob.Name + "-manual-",
			Namespace:    namespace,
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}
	job, err = h.clientset.BatchV1().Jobs(namespace).Create(c.Request.Context(), job, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job":       job.Name,
		"namespace": namespace,
	})
}
