package handlers

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) GetRolesHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	roles, err := h.clientset.RbacV1().Roles(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, r := range roles.Items {
		rulesCount := len(r.Rules)
		result = append(result, gin.H{
			"name":       r.Name,
			"namespace":  r.Namespace,
			"rulesCount": rulesCount,
			"age":        time.Since(r.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace": namespace,
		"count":     len(result),
		"roles":     result,
	})
}

func (h *Handler) GetRoleBindingsHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	bindings, err := h.clientset.RbacV1().RoleBindings(namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, b := range bindings.Items {
		subjects := []string{}
		for _, s := range b.Subjects {
			subjects = append(subjects, s.Kind+"/"+s.Name)
		}
		result = append(result, gin.H{
			"name":      b.Name,
			"namespace": b.Namespace,
			"roleRef":   b.RoleRef.Name,
			"subjects": subjects,
			"age":      time.Since(b.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"namespace":    namespace,
		"count":        len(result),
		"roleBindings": result,
	})
}

func (h *Handler) GetClusterRolesHandler(c *gin.Context) {
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	roles, err := h.clientset.RbacV1().ClusterRoles().List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, r := range roles.Items {
		result = append(result, gin.H{
			"name":       r.Name,
			"rulesCount": len(r.Rules),
			"age":        time.Since(r.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"count": len(result),
		"roles": result,
	})
}

func (h *Handler) GetClusterRoleBindingsHandler(c *gin.Context) {
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	bindings, err := h.clientset.RbacV1().ClusterRoleBindings().List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, b := range bindings.Items {
		subjects := []string{}
		for _, s := range b.Subjects {
			subjects = append(subjects, s.Kind+"/"+s.Name)
		}
		result = append(result, gin.H{
			"name":     b.Name,
			"roleRef":  b.RoleRef.Name,
			"subjects": subjects,
			"age":      time.Since(b.CreationTimestamp.Time).Round(time.Second).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"count":        len(result),
		"roleBindings": result,
	})
}

// GetRBACVisualizationHandler возвращает плоский список: субъект → роль → права (для UI-визуализации).
func (h *Handler) GetRBACVisualizationHandler(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "")
	if h.clientset == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "K8s client not ready"})
		return
	}

	type row struct {
		Subject    string   `json:"subject"`
		Role       string   `json:"role"`
		Kind       string   `json:"kind"` // Role or ClusterRole
		Namespace  string   `json:"namespace"`
		APIGroups  []string `json:"apiGroups"`
		Resources  []string `json:"resources"`
		Verbs      []string `json:"verbs"`
		RulesSummary string `json:"rulesSummary"`
	}

	var rows []row
	roleRules := make(map[string][]rbacv1.PolicyRule) // "ns/name" or "cluster/name" -> rules

	// Namespace-scoped: Roles + RoleBindings
	nsList := []string{namespace}
	if namespace == "" || namespace == "all" {
		nsList, _ = h.listNamespaces(c.Request.Context())
		if len(nsList) == 0 {
			nsList = []string{"default"}
		}
	}
	for _, ns := range nsList {
		roles, err := h.clientset.RbacV1().Roles(ns).List(c.Request.Context(), metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, r := range roles.Items {
			key := ns + "/" + r.Name
			roleRules[key] = r.Rules
		}
		bindings, err := h.clientset.RbacV1().RoleBindings(ns).List(c.Request.Context(), metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, b := range bindings.Items {
			key := b.Namespace + "/" + b.RoleRef.Name
			rules := roleRules[key]
			summary := rulesSummaryMany(rules)
			subjects := subjectStrings(b.Subjects)
			for _, subj := range subjects {
				rows = append(rows, row{Subject: subj, Role: b.RoleRef.Name, Kind: "Role", Namespace: b.Namespace, RulesSummary: summary})
			}
		}
	}

	// Cluster-scoped: ClusterRoles + ClusterRoleBindings
	crList, err := h.clientset.RbacV1().ClusterRoles().List(c.Request.Context(), metav1.ListOptions{})
	if err == nil {
		for _, r := range crList.Items {
			roleRules["cluster/"+r.Name] = r.Rules
		}
	}
	crbList, err := h.clientset.RbacV1().ClusterRoleBindings().List(c.Request.Context(), metav1.ListOptions{})
	if err == nil {
		for _, b := range crbList.Items {
			key := "cluster/" + b.RoleRef.Name
			rules := roleRules[key]
			summary := rulesSummaryMany(rules)
			subjects := subjectStrings(b.Subjects)
			for _, subj := range subjects {
				rows = append(rows, row{Subject: subj, Role: b.RoleRef.Name, Kind: "ClusterRole", Namespace: "", RulesSummary: summary})
			}
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Subject != rows[j].Subject {
			return rows[i].Subject < rows[j].Subject
		}
		return rows[i].Role < rows[j].Role
	})

	c.JSON(http.StatusOK, gin.H{"namespace": namespace, "rows": rows})
}

func (h *Handler) listNamespaces(ctx context.Context) ([]string, error) {
	list, err := h.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	ns := make([]string, 0, len(list.Items))
	for _, n := range list.Items {
		ns = append(ns, n.Name)
	}
	return ns, nil
}

func subjectStrings(subjects []rbacv1.Subject) []string {
	if len(subjects) == 0 {
		return []string{"—"}
	}
	out := make([]string, 0, len(subjects))
	for _, s := range subjects {
		out = append(out, s.Kind+"/"+s.Name)
	}
	return out
}

func rulesSummary(r rbacv1.PolicyRule) string {
	api := strings.Join(r.APIGroups, ",")
	if api == "" {
		api = "core"
	}
	res := strings.Join(r.Resources, ",")
	verbs := strings.Join(r.Verbs, ",")
	return api + " | " + res + " | " + verbs
}

func rulesSummaryMany(rules []rbacv1.PolicyRule) string {
	if len(rules) == 0 {
		return "—"
	}
	var parts []string
	for _, r := range rules {
		parts = append(parts, rulesSummary(r))
	}
	return strings.Join(parts, " · ")
}
