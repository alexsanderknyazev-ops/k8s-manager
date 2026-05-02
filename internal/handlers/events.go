package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetEventsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	limit := int64(100)
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.ParseInt(l, 10, 64); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	var events []corev1.Event
	if namespace == "all" {
		nsList, err := h.clientset.CoreV1().Namespaces().List(c.Request.Context(), metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		maxTotal := int(limit)
		perNS := int64(25)
		if limit < 25 {
			perNS = limit
		}
		for _, ns := range nsList.Items {
			if len(events) >= maxTotal {
				break
			}
			list, err := h.clientset.CoreV1().Events(ns.Name).List(c.Request.Context(), metav1.ListOptions{Limit: perNS})
			if err != nil {
				continue
			}
			for i := range list.Items {
				events = append(events, list.Items[i])
				if len(events) >= maxTotal {
					break
				}
			}
		}
	} else {
		list, err := h.clientset.CoreV1().Events(namespace).List(c.Request.Context(), metav1.ListOptions{Limit: limit})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		events = list.Items
	}

	var result []gin.H
	for _, ev := range events {
		obj := ev.InvolvedObject
		msg := ev.Message
		if msg == "" {
			msg = ev.Reason
		}
		ts := ev.LastTimestamp.Time
		if ts.IsZero() {
			ts = ev.EventTime.Time
		}
		age := ""
		if !ts.IsZero() {
			age = time.Since(ts).Round(time.Second).String() + " ago"
		}
		result = append(result, gin.H{
			"type":      ev.Type,
			"object":    obj.Name,
			"kind":      obj.Kind,
			"namespace": obj.Namespace,
			"message":   msg,
			"reason":    ev.Reason,
			"time":      age,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"events":    result,
	})
}
