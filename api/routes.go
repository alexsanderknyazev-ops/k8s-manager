package api

import (
	"net/http"

	"k8s-manager/internal/auth"
	"k8s-manager/internal/config"
	"k8s-manager/internal/handlers"
	"k8s-manager/internal/middleware"
	"k8s-manager/internal/rbac"
	"k8s-manager/internal/store"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

func SetupRoutes(r *gin.Engine, clientset kubernetes.Interface, metricsClient metricsv.Interface, restConfig *rest.Config, cfg *config.Config, userStore auth.UserStore, userManager store.UserManager, permManager store.PermissionManager) {
	handler := handlers.NewHandler(clientset, metricsClient, userManager, permManager, restConfig)
	rbac.SetPermissionStore(nil)
	if pm, ok := permManager.(rbac.PermissionStore); ok {
		rbac.SetPermissionStore(pm)
	}

	r.Use(middleware.RequestID(), middleware.SecurityHeaders(), middleware.PrometheusMetrics())
	r.GET("/metrics", middleware.MetricsHandler)

	if cfg.Auth.Enabled {
		r.GET("/login", func(c *gin.Context) {
			if cfg.Auth.OIDC.Enabled {
				c.Redirect(http.StatusFound, "/api/auth/oidc/login?next="+c.Query("next"))
				return
			}
			c.HTML(http.StatusOK, "login.html", gin.H{
				"Title": "Login",
				"Next":  c.Query("next"),
			})
		})
		if cfg.Auth.OIDC.Enabled {
			r.GET("/api/auth/oidc/login", auth.OIDCLogin)
			r.GET("/api/auth/oidc/callback", auth.OIDCCallback)
			r.POST("/api/login", func(c *gin.Context) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "password login disabled; use OIDC"})
			})
		} else {
			r.POST("/api/login", middleware.RateLimitLogin(cfg.RateLimit.LoginPerMin), func(c *gin.Context) {
				auth.Login(c, userStore, cfg.Auth.Username, cfg.Auth.Password, cfg.Auth.PasswordHash)
			})
		}
		r.POST("/api/logout", auth.Logout)

		r.Use(auth.Middleware())
	} else {
		r.POST("/api/logout", func(c *gin.Context) {
			if c.GetHeader("Accept") == "application/json" {
				c.JSON(http.StatusOK, gin.H{"ok": true})
				return
			}
			c.Redirect(http.StatusFound, "/")
		})
	}

	r.Use(rbac.Middleware(), middleware.CSRF(), middleware.ReadOnly(cfg.ReadOnly), middleware.Audit())
	setupAppRoutes(r, handler, cfg)
}

func setupAppRoutes(r *gin.Engine, handler *handlers.Handler, cfg *config.Config) {
	pageData := func(c *gin.Context, title string) gin.H {
		readOnly := cfg.ReadOnly
		if v, ok := c.Get("effective_read_only"); ok {
			if b, _ := v.(bool); b {
				readOnly = true
			}
		}
		return gin.H{"Title": title, "ReadOnly": readOnly}
	}

	// ===== UI ROUTES =====
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/dashboard")
	})

	r.GET("/ui", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/dashboard")
	})

	r.GET("/ui/dashboard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", pageData(c, "Dashboard"))
	})

	r.GET("/ui/applications", func(c *gin.Context) {
		c.HTML(http.StatusOK, "applications.html", pageData(c, "Applications"))
	})

	r.GET("/ui/pods", func(c *gin.Context) {
		c.HTML(http.StatusOK, "pods.html", pageData(c, "Pods"))
	})
	r.GET("/ui/exec", func(c *gin.Context) {
		data := pageData(c, "Pod Terminal")
		q := c.Query("embed")
		data["Embed"] = q == "1" || q == "true" || q == "yes"
		c.HTML(http.StatusOK, "exec.html", data)
	})

	r.GET("/ui/deployments", func(c *gin.Context) {
		c.HTML(http.StatusOK, "deployments.html", pageData(c, "Deployments"))
	})

	r.GET("/ui/config", func(c *gin.Context) {
		c.HTML(http.StatusOK, "config.html", pageData(c, "Configuration"))
	})
	r.GET("/ui/deploy", func(c *gin.Context) {
		c.HTML(http.StatusOK, "deploy.html", pageData(c, "Деплой"))
	})
	r.GET("/ui/events", func(c *gin.Context) {
		c.HTML(http.StatusOK, "events.html", pageData(c, "Events"))
	})
	r.GET("/ui/cronjobs", func(c *gin.Context) {
		c.HTML(http.StatusOK, "cronjobs.html", pageData(c, "CronJobs"))
	})
	r.GET("/ui/audit", func(c *gin.Context) {
		c.HTML(http.StatusOK, "audit.html", pageData(c, "Audit"))
	})
	r.GET("/ui/hpa", func(c *gin.Context) {
		c.HTML(http.StatusOK, "hpa.html", pageData(c, "HPA"))
	})
	r.GET("/ui/statefulsets", func(c *gin.Context) {
		c.HTML(http.StatusOK, "statefulsets.html", pageData(c, "StatefulSets"))
	})
	r.GET("/ui/daemonsets", func(c *gin.Context) {
		c.HTML(http.StatusOK, "daemonsets.html", pageData(c, "DaemonSets"))
	})
	r.GET("/ui/users", func(c *gin.Context) {
		data := pageData(c, "Users")
		data["UserManagementAvailable"] = handler.UserManagementAvailable()
		c.HTML(http.StatusOK, "users.html", data)
	})
	r.GET("/ui/permissions", func(c *gin.Context) {
		data := pageData(c, "Permissions")
		data["PermissionManagementAvailable"] = handler.PermissionManagementAvailable()
		c.HTML(http.StatusOK, "permissions.html", data)
	})

	// ===== API ROUTES =====
	api := r.Group("/api")
	api.Use(middleware.RateLimitAPI(cfg.RateLimit.APIPerMin))
	{
		api.GET("/", handler.HomeHandler)
		api.GET("/docs", handler.OpenAPIDocsHandler)
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
		api.GET("/pod/exec/ws", handler.PodExecWSHandler)

		// Port-forwarding
		api.GET("/portforward/sessions", handler.GetPortForwardSessionsHandler)
		api.POST("/portforward/start", handler.StartPortForwardHandler)
		api.POST("/portforward/stop/:id", handler.StopPortForwardHandler)
		api.GET("/portforward/check/:port", handler.CheckPortAvailableHandler)

		// Deployments
		api.GET("/deployments", handler.GetDeploymentsHandler)
		api.POST("/deployment", handler.CreateDeploymentHandler)
		// Простой деплой (шаблоны + создание по параметрам)
		api.GET("/deploy/templates", handler.DeployTemplatesHandler)
		api.POST("/deploy/simple", handler.SimpleDeployHandler)
		api.GET("/deployment/yaml/:namespace/:name", handler.GetDeploymentYAMLHandler)
		api.PUT("/deployment/yaml/:namespace/:name", handler.UpdateDeploymentYAMLHandler)
		api.POST("/scale/:namespace/:deployment", handler.ScaleDeploymentHandler)
		api.POST("/restart/:namespace/:deployment", handler.RestartDeploymentHandler)
		api.POST("/rollback/:namespace/:deployment", handler.RollbackDeploymentHandler)
		api.DELETE("/deployment/:namespace/:deployment", handler.DeleteDeploymentHandler)

		// HPA
		api.GET("/hpas", handler.GetHPAHandler)
		api.GET("/hpa/yaml/:namespace/:name", handler.GetHPAYAMLHandler)
		api.POST("/hpa/:namespace/:name", handler.CreateOrUpdateHPAHandler)

		// StatefulSets & DaemonSets
		api.GET("/statefulsets", handler.GetStatefulSetsHandler)
		api.GET("/statefulset/yaml/:namespace/:name", handler.GetStatefulSetYAMLHandler)
		api.POST("/statefulset/scale/:namespace/:name", handler.ScaleStatefulSetHandler)
		api.GET("/daemonsets", handler.GetDaemonSetsHandler)
		api.GET("/daemonset/yaml/:namespace/:name", handler.GetDaemonSetYAMLHandler)

		// Applications
		api.GET("/applications", handler.GetApplicationsHandler)

		// Services
		api.GET("/services", handler.GetServicesHandler)
		api.GET("/service/yaml/:namespace/:name", handler.GetServiceYAMLHandler)

		// ConfigMaps & Secrets
		api.GET("/configmaps/:namespace", handler.GetConfigMapsHandler)
		api.GET("/configmap/yaml/:namespace/:name", handler.GetConfigMapYAMLHandler)
		api.GET("/secrets/:namespace", handler.GetSecretsHandler)
		api.GET("/secret/yaml/:namespace/:name", handler.GetSecretYAMLHandler)
		api.GET("/secret/data/:namespace/:name", handler.GetSecretDataHandler)

		// Ingress
		api.GET("/ingresses", handler.GetIngressHandler)
		api.GET("/ingress/yaml/:namespace/:name", handler.GetIngressYAMLHandler)

		// PVC
		api.GET("/pvcs", handler.GetPVCHandler)

		// Export namespace
		api.GET("/export/namespace/:namespace", handler.ExportNamespaceHandler)

		// ResourceQuota & LimitRange
		api.GET("/resourcequotas", handler.GetResourceQuotasHandler)
		api.GET("/limitranges", handler.GetLimitRangesHandler)

		// RBAC
		api.GET("/roles/:namespace", handler.GetRolesHandler)
		api.GET("/rolebindings/:namespace", handler.GetRoleBindingsHandler)
		api.GET("/clusterroles", handler.GetClusterRolesHandler)
		api.GET("/clusterrolebindings", handler.GetClusterRoleBindingsHandler)
		api.GET("/rbac/visualization", handler.GetRBACVisualizationHandler)

		// CronJobs & Jobs
		api.GET("/cronjobs", handler.GetCronJobsHandler)
		api.GET("/jobs", handler.GetJobsHandler)
		api.POST("/cronjob/:namespace/:name/trigger", handler.CreateJobFromCronJobHandler)

		// Audit log
		api.GET("/audit", handler.GetAuditHandler)

		// Search
		api.GET("/search", handler.SearchHandler)
		api.GET("/contexts", handlers.GetContextsHandler(cfg.Kubeconfig))

		// Users (Postgres only; admin only except change-password)
		api.GET("/users", handler.ListUsersHandler)
		api.POST("/users", handler.CreateUserHandler)
		api.PATCH("/users/:username", handler.UpdateUserHandler)
		api.DELETE("/users/:username", handler.DeleteUserHandler)
		api.POST("/auth/change-password", handler.ChangePasswordHandler)
		api.GET("/permissions", handler.ListPermissionsHandler)
		api.POST("/permissions", handler.GrantPermissionHandler)
		api.DELETE("/permissions", handler.RevokePermissionHandler)

		// Namespaces & Nodes
		api.GET("/namespaces", handler.GetNamespacesHandler)
		api.GET("/nodes", handler.GetNodesHandler)

		// Events
		api.GET("/events", handler.GetEventsHandler)

		// Metrics API
		api.GET("/metrics/pods/:namespace", handler.GetPodMetricsHandler)
		api.GET("/metrics/pod/:namespace/:pod", handler.GetSinglePodMetricsHandler)
		api.GET("/metrics/all-pods", handler.GetAllPodsMetricsHandler)
		api.GET("/metrics/nodes", handler.GetNodeMetricsHandler)

		// Real-time logs API
		api.GET("/logs/stream/:namespace/:pod", handler.StartLogStreamHandler)
		api.GET("/logs/streams", handler.GetLogStreamsHandler)
		api.DELETE("/logs/stream/:id", handler.StopLogStreamHandler)
		api.GET("/watch/pods", handler.WatchPodsHandler)
	}
}
