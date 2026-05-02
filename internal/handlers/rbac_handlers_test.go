package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	rbacv1 "k8s.io/api/rbac/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func rbacFakeCluster(t *testing.T) *fake.Clientset {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "r1", Namespace: "default"},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list"},
		}},
	}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "rb1", Namespace: "default"},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
		RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "r1"},
	}
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "cr1"},
		Rules:      role.Rules,
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "crb1"},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: "system:masters"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "cr1"},
	}
	return fake.NewClientset(ns, role, rb, cr, crb)
}

func TestRBAC_ListHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := rbacFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck

	for _, tc := range []struct {
		path string
		fn   func(*gin.Context)
	}{
		{"/api/rbac/roles?namespace=default", h.GetRolesHandler},
		{"/api/rbac/rolebindings?namespace=default", h.GetRoleBindingsHandler},
		{"/api/rbac/clusterroles", h.GetClusterRolesHandler},
		{"/api/rbac/clusterrolebindings", h.GetClusterRoleBindingsHandler},
		{"/api/rbac/visualization?namespace=default", h.GetRBACVisualizationHandler},
		{"/api/rbac/visualization?namespace=all", h.GetRBACVisualizationHandler},
	} {
		w := httptest.NewRecorder()
		c := ctxGET(w, tc.path)
		tc.fn(c)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}

func TestRBAC_clientsetNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/rbac/roles")
	h.GetRolesHandler(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}
