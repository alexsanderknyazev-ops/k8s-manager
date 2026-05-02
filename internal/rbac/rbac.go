package rbac

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type PermissionStore interface {
	HasPermission(ctx context.Context, subject, namespace, resource, verb string) bool
}

var permissionStore PermissionStore
var legacyAdminBypass bool

func SetPermissionStore(s PermissionStore) { permissionStore = s }
func SetLegacyAdminBypass(v bool)         { legacyAdminBypass = v }

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}
		if path == "/api/health" || path == "/api/docs" || strings.HasPrefix(path, "/api/auth/oidc/") || path == "/api/logout" {
			c.Next()
			return
		}
		username, _ := c.Get("username")
		subject, _ := username.(string)
		if subject == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		// Совместимость с legacy-ролями (включается явно через env).
		if legacyAdminBypass {
			if role, _ := c.Get("role"); role == "admin" {
				c.Next()
				return
			}
		}
		resource := detectResource(path)
		verb := detectVerb(c.Request.Method)
		ns := c.Param("namespace")
		if ns == "" {
			ns = c.Query("namespace")
		}
		if ns == "" {
			ns = c.Query("ns")
		}
		if ns == "" {
			ns = "*"
		}
		if permissionStore == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden: permission store is not configured"})
			return
		}
		if !permissionStore.HasPermission(c.Request.Context(), subject, ns, resource, verb) {
			c.Header("X-RBAC-Deny", "1")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

func detectVerb(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return "read"
	default:
		return "write"
	}
}

func detectResource(path string) string {
	switch {
	case strings.Contains(path, "/permissions"):
		return "permissions"
	case strings.Contains(path, "/pods"), strings.Contains(path, "/pod/"):
		return "pods"
	case strings.Contains(path, "/deployments"), strings.Contains(path, "/deployment/"), strings.Contains(path, "/deploy/"):
		return "deployments"
	case strings.Contains(path, "/services"), strings.Contains(path, "/service/"):
		return "services"
	case strings.Contains(path, "/configmap"):
		return "configmaps"
	case strings.Contains(path, "/secret"):
		return "secrets"
	case strings.Contains(path, "/ingress"):
		return "ingresses"
	case strings.Contains(path, "/hpa"):
		return "hpa"
	case strings.Contains(path, "/statefulset"):
		return "statefulsets"
	case strings.Contains(path, "/daemonset"):
		return "daemonsets"
	case strings.Contains(path, "/cronjob"), strings.Contains(path, "/jobs"):
		return "jobs"
	case strings.Contains(path, "/nodes"):
		return "nodes"
	case strings.Contains(path, "/namespaces"):
		return "namespaces"
	default:
		return "cluster"
	}
}
