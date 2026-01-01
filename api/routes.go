package api

import (
	"net/http"

	"k8s-manager/internal/handlers" // Используйте полный путь

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

func SetupRoutes(r *gin.Engine, clientset *kubernetes.Clientset, metricsClient *metricsv.Clientset) {
	handler := handlers.NewHandler(clientset, metricsClient)

	// ===== UI ROUTES =====
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/dashboard")
	})

	r.GET("/ui", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/dashboard")
	})

	r.GET("/ui/dashboard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"Title": "Dashboard",
		})
	})

	r.GET("/ui/applications", func(c *gin.Context) {
		c.HTML(http.StatusOK, "applications.html", gin.H{
			"Title": "Applications",
		})
	})

	r.GET("/ui/pods", func(c *gin.Context) {
		c.HTML(http.StatusOK, "pods.html", gin.H{
			"Title": "Pods",
		})
	})

	r.GET("/ui/deployments", func(c *gin.Context) {
		c.HTML(http.StatusOK, "deployments.html", gin.H{
			"Title": "Deployments",
		})
	})

	r.GET("/ui/config", func(c *gin.Context) {
		c.HTML(http.StatusOK, "config.html", gin.H{
			"Title": "Configuration",
		})
	})

	// ===== API ROUTES =====

	// Health & Info
	api := r.Group("/api")
	{
		api.GET("/", handler.HomeHandler)
		api.GET("/health", handler.HealthHandler)
		api.GET("/test", handler.TestConnectionHandler)

		// Pods
		api.GET("/pods", handler.GetPodsHandler)
		api.GET("/logs/:namespace/:pod", handler.GetLogsHandler)
		api.GET("/logs/download/:namespace/:pod", handler.DownloadLogsHandler)
		api.GET("/pod/yaml/:namespace/:pod", handler.GetPodYAMLHandler)
		api.PUT("/pod/yaml/:namespace/:pod", handler.UpdatePodYAMLHandler)
		api.DELETE("/pod/:namespace/:pod", handler.DeletePodHandler)
		api.GET("/pod/details/:namespace/:pod", handler.GetPodDetailsHandler)

		// Port-forwarding
		api.GET("/portforward/sessions", handler.GetPortForwardSessionsHandler)
		api.POST("/portforward/start", handler.StartPortForwardHandler)
		api.POST("/portforward/stop/:id", handler.StopPortForwardHandler)
		api.GET("/portforward/check/:port", handler.CheckPortAvailableHandler)

		// Deployments
		api.GET("/deployments", handler.GetDeploymentsHandler)
		api.GET("/deployment/yaml/:namespace/:name", handler.GetDeploymentYAMLHandler)
		api.PUT("/deployment/yaml/:namespace/:name", handler.UpdateDeploymentYAMLHandler)
		api.POST("/scale/:namespace/:deployment", handler.ScaleDeploymentHandler)
		api.POST("/restart/:namespace/:deployment", handler.RestartDeploymentHandler)
		api.DELETE("/deployment/:namespace/:deployment", handler.DeleteDeploymentHandler)

		// Applications
		api.GET("/applications", handler.GetApplicationsHandler)

		// Services
		api.GET("/services", handler.GetServicesHandler)
		api.GET("/service/yaml/:namespace/:name", handler.GetServiceYAMLHandler)

		// ConfigMaps & Secrets
		api.GET("/configmaps/:namespace", handler.GetConfigMapsHandler)
		api.GET("/configmap/yaml/:namespace/:name", handler.GetConfigMapYAMLHandler)
		api.GET("/secrets/:namespace", handler.GetSecretsHandler)

		// Namespaces & Nodes
		api.GET("/namespaces", handler.GetNamespacesHandler)
		api.GET("/nodes", handler.GetNodesHandler)

		// Metrics API
		api.GET("/metrics/pods/:namespace", handler.GetPodMetricsHandler)
		api.GET("/metrics/pod/:namespace/:pod", handler.GetSinglePodMetricsHandler)
		api.GET("/metrics/all-pods", handler.GetAllPodsMetricsHandler)
		api.GET("/metrics/nodes", handler.GetNodeMetricsHandler)
	}
}
