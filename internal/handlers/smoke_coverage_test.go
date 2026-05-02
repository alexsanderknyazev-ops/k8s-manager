package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"k8s-manager/internal/store"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

type smokePermMgr struct{}

func (smokePermMgr) ListPermissions(ctx context.Context, subject, namespace string) ([]store.Permission, error) {
	return []store.Permission{{Subject: "admin", Namespace: "default", Resource: "pods", Verb: "read"}}, nil
}
func (smokePermMgr) GrantPermission(ctx context.Context, subject, namespace, resource, verb, grantedBy string) error {
	return nil
}
func (smokePermMgr) RevokePermission(ctx context.Context, subject, namespace, resource, verb string) error {
	return nil
}

func ctxPOSTJSON(w *httptest.ResponseRecorder, path string, body string) *gin.Context {
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("username", "admin")
	return c
}

func ctxGETParams(w *httptest.ResponseRecorder, path string, params gin.Params) *gin.Context {
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Params = params
	return c
}

func richFakeCluster(t *testing.T) *fake.Clientset {
	t.Helper()
	r1 := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r1,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "c", Image: "nginx"}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			NodeName:   "node-1",
			Containers: []corev1.Container{{Name: "c", Image: "nginx"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "c", Ready: true,
			}},
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			Type:  corev1.ServiceTypeClusterIP,
		},
	}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default"}, Data: map[string]string{"k": "v"}}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "sec", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"key": []byte("val")},
	}
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "ev1", Namespace: "default"},
		Type:       corev1.EventTypeNormal,
		Message:    "hello",
	}
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing", Namespace: "default"},
		Spec:       networkingv1.IngressSpec{},
	}
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "cj", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers:    []corev1.Container{{Name: "c", Image: "busybox"}},
						},
					},
				},
			},
		},
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job1", Namespace: "default"},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{{Name: "c", Image: "busybox"}},
				},
			},
		},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ds1", Namespace: "default"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"a": "b"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "c", Image: "nginx"}},
				},
			},
		},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "sts1", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &r1,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"a": "s"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"a": "s"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "c", Image: "nginx"}},
				},
			},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc1", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "rq1", Namespace: "default"},
		Spec:       corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")}},
		Status:     corev1.ResourceQuotaStatus{Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("1")}},
	}
	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: "lr1", Namespace: "default"},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{{Type: corev1.LimitTypePod}},
		},
	}
	minR := int32(1)
	maxR := int32(5)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "hpa1", Namespace: "default"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "web"},
			MinReplicas:    &minR,
			MaxReplicas:    maxR,
			Metrics:        []autoscalingv2.MetricSpec{{Type: autoscalingv2.ResourceMetricSourceType, Resource: &autoscalingv2.ResourceMetricSource{Name: corev1.ResourceCPU}}},
		},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{CurrentReplicas: 2, DesiredReplicas: 2},
	}

	cs := fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		pod, dep, svc, cm, sec, ev, ing, cj, job, ds, sts, pvc, rq, lr, hpa,
	)
	return cs
}

func TestSmoke_ListPodsDeploymentsServices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck

	for _, tc := range []struct {
		path string
		fn   func(*gin.Context)
	}{
		{"/api/pods?namespace=default", h.GetPodsHandler},
		{"/api/pods?namespace=all", h.GetPodsHandler},
		{"/api/deployments?namespace=default", h.GetDeploymentsHandler},
		{"/api/services?namespace=default", h.GetServicesHandler},
		{"/api/events?namespace=default", h.GetEventsHandler},
		// Param(":namespace") задаётся через Params — см. отдельные вызовы ниже
		{"/api/ingresses?namespace=default", h.GetIngressHandler},
		{"/api/cronjobs?namespace=default", h.GetCronJobsHandler},
		{"/api/jobs?namespace=default", h.GetJobsHandler},
		{"/api/daemonsets?namespace=default", h.GetDaemonSetsHandler},
		{"/api/statefulsets?namespace=default", h.GetStatefulSetsHandler},
		{"/api/pvcs?namespace=default", h.GetPVCHandler},
		{"/api/resourcequotas?namespace=default", h.GetResourceQuotasHandler},
		{"/api/limitranges?namespace=default", h.GetLimitRangesHandler},
		{"/api/hpas?namespace=default", h.GetHPAHandler},
		{"/api/search?q=web&namespace=default", h.SearchHandler},
		{"/api/applications?namespace=default", h.GetApplicationsHandler},
	} {
		w := httptest.NewRecorder()
		c := ctxGET(w, tc.path)
		tc.fn(c)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d %s", tc.path, w.Code, w.Body.String())
		}
	}

	for _, x := range []struct {
		fn func(*gin.Context)
		p  gin.Params
	}{
		{h.GetConfigMapsHandler, gin.Params{{Key: "namespace", Value: "default"}}},
		{h.GetSecretsHandler, gin.Params{{Key: "namespace", Value: "default"}}},
	} {
		w := httptest.NewRecorder()
		c := ctxGETParams(w, "/api/x", x.p)
		x.fn(c)
		if w.Code != http.StatusOK {
			t.Fatalf("list cm/sec %d %s", w.Code, w.Body.String())
		}
	}
}

func TestSmoke_YAMLAndDetailsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck

	tests := []struct {
		path string
		p    gin.Params
		fn   func(*gin.Context)
	}{
		{"/api/deployment/yaml/default/web", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "web"}}, h.GetDeploymentYAMLHandler},
		{"/api/pod/yaml/default/web-pod", gin.Params{{Key: "namespace", Value: "default"}, {Key: "pod", Value: "web-pod"}}, h.GetPodYAMLHandler},
		{"/api/service/yaml/default/svc", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "svc"}}, h.GetServiceYAMLHandler},
		{"/api/configmap/yaml/default/cfg", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "cfg"}}, h.GetConfigMapYAMLHandler},
		{"/api/secret/yaml/default/sec", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "sec"}}, h.GetSecretYAMLHandler},
		{"/api/ingress/yaml/default/ing", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "ing"}}, h.GetIngressYAMLHandler},
		{"/api/daemonset/yaml/default/ds1", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "ds1"}}, h.GetDaemonSetYAMLHandler},
		{"/api/statefulset/yaml/default/sts1", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "sts1"}}, h.GetStatefulSetYAMLHandler},
		{"/api/hpa/yaml/default/hpa1", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "hpa1"}}, h.GetHPAYAMLHandler},
		{"/api/pod/details/default/web-pod", gin.Params{{Key: "namespace", Value: "default"}, {Key: "pod", Value: "web-pod"}}, h.GetPodDetailsHandler},
	}
	for _, tc := range tests {
		w := httptest.NewRecorder()
		c := ctxGETParams(w, tc.path, tc.p)
		tc.fn(c)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}

func TestSmoke_MetricsHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	mf := metricsfake.NewSimpleClientset() //nolint:staticcheck
	h := NewHandler(cs, mf, nil, nil, nil)

	w := httptest.NewRecorder()
	c := ctxGETParams(w, "/api/metrics/pods/default", gin.Params{{Key: "namespace", Value: "default"}})
	h.GetPodMetricsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("pod metrics %d %s", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	c2 := ctxGETParams(w2, "/api/metrics/pod/default/web-pod", gin.Params{{Key: "namespace", Value: "default"}, {Key: "pod", Value: "web-pod"}})
	h.GetSinglePodMetricsHandler(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("single pod metrics %d", w2.Code)
	}

	w3 := httptest.NewRecorder()
	c3 := ctxGET(w3, "/api/metrics/all-pods")
	h.GetAllPodsMetricsHandler(c3)
	if w3.Code != http.StatusOK {
		t.Fatalf("all pods metrics %d", w3.Code)
	}

	w4 := httptest.NewRecorder()
	c4 := ctxGET(w4, "/api/metrics/nodes")
	h.GetNodeMetricsHandler(c4)
	if w4.Code != http.StatusOK {
		t.Fatalf("node metrics %d", w4.Code)
	}

	w5 := httptest.NewRecorder()
	c5 := ctxGET(w5, "/api/metrics/check")
	h.CheckMetricsHandler(c5)
	if w5.Code != http.StatusOK {
		t.Fatalf("check metrics %d", w5.Code)
	}

	w6 := httptest.NewRecorder()
	c6 := ctxGET(w6, "/api/metrics/pods/summary?namespace=default")
	h.GetPodMetricsSummaryHandler(c6)
	if w6.Code != http.StatusOK {
		t.Fatalf("summary %d", w6.Code)
	}
}


func TestSmoke_AuditAndPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, &smokePermMgr{}, nil) //nolint:staticcheck

	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/audit?limit=10")
	h.GetAuditHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}

	w2 := httptest.NewRecorder()
	c2 := ctxGET(w2, "/api/permissions?subject=admin")
	h.ListPermissionsHandler(c2)
	if w2.Code != http.StatusOK {
		t.Fatal(w2.Code)
	}

	w3 := httptest.NewRecorder()
	h.GrantPermissionHandler(ctxPOSTJSON(w3, "/api/permissions", `{"subject":"u","namespace":"default","resource":"pods","verb":"read"}`))
	if w3.Code != http.StatusOK {
		t.Fatal(w3.Body.String())
	}

	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest(http.MethodDelete, "/api/permissions?subject=u&namespace=default&resource=pods&verb=read", nil)
	h.RevokePermissionHandler(c4)
	if w4.Code != http.StatusOK {
		t.Fatalf("revoke %d %s", w4.Code, w4.Body.String())
	}
}

func TestSmoke_GetContexts_withTempKubeconfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	kcfg := filepath.Join(dir, "kubeconfig")
	content := `apiVersion: v1
kind: Config
current-context: ctx1
contexts:
- name: ctx1
  context:
    cluster: c1
    user: u1
clusters:
- name: c1
  cluster:
    server: https://127.0.0.1:6443
users:
- name: u1
  user: {}
`
	if err := os.WriteFile(kcfg, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	h := GetContextsHandler(kcfg)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/contexts")
	h(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["current"] != "ctx1" {
		t.Fatalf("current context: %v", body["current"])
	}
}

func TestSmoke_ExportNamespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxGETParams(w, "/api/export/namespace/default", gin.Params{{Key: "namespace", Value: "default"}})
	h.ExportNamespaceHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestSmoke_CreateJobFromCronJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "cjrun", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			Schedule: "* * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers:    []corev1.Container{{Name: "c", Image: "busybox"}},
						},
					},
				},
			},
		},
	}
	_, err := cs.BatchV1().CronJobs("default").Create(context.Background(), cj, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/cronjob/default/cjrun/trigger", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "cjrun"}}
	h.CreateJobFromCronJobHandler(c)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
}
