package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFormatHPAMetrics_allBranches(t *testing.T) {
	cpuQty := resource.MustParse("100m")
	x := int32(80)
	hpa := autoscalingv2.HorizontalPodAutoscaler{
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &x,
						},
					},
				},
				{
					Type: autoscalingv2.PodsMetricSourceType,
					Pods: &autoscalingv2.PodsMetricSource{
						Metric: autoscalingv2.MetricIdentifier{Name: "packets-per-second"},
						Target: autoscalingv2.MetricTarget{Type: autoscalingv2.AverageValueMetricType, AverageValue: &cpuQty},
					},
				},
				{
					Type: autoscalingv2.ObjectMetricSourceType,
					Object: &autoscalingv2.ObjectMetricSource{
						Metric: autoscalingv2.MetricIdentifier{Name: "requests-per-second"},
						Target: autoscalingv2.MetricTarget{Type: autoscalingv2.ValueMetricType, Value: &cpuQty},
					},
				},
				{Type: autoscalingv2.ContainerResourceMetricSourceType},
			},
		},
	}
	out := formatHPAMetrics(hpa)
	if len(out) < 4 {
		t.Fatalf("got %d entries", len(out))
	}
}

func TestCheckMetricsHandler_withFakeMetricsClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mc := metricsfake.NewSimpleClientset() //nolint:staticcheck
	h := NewHandler(fake.NewClientset(), mc, nil, nil, nil)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/metrics/check")
	h.CheckMetricsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestCreateOrUpdateHPAHandler_create(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
	)
	body := `{"targetName":"web","targetKind":"Deployment","minReplicas":2,"maxReplicas":5}`
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxPOSTJSON(w, "/api/hpa/default/new-hpa", body)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "new-hpa"}}
	h.CreateOrUpdateHPAHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestScaleStatefulSetHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/statefulset/default/sts1/scale?replicas=2", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "sts1"}}
	h.ScaleStatefulSetHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestGetStatefulSetYAMLHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxGETParams(w, "/api/statefulset/yaml/default/sts1", gin.Params{
		{Key: "namespace", Value: "default"}, {Key: "name", Value: "sts1"},
	})
	h.GetStatefulSetYAMLHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestGetSecretYAMLAndDataHandlers_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck

	w := httptest.NewRecorder()
	c := ctxGETParams(w, "/api/secret/yaml/default/sec", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "sec"}})
	h.GetSecretYAMLHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}

	w2 := httptest.NewRecorder()
	c2 := ctxGETParams(w2, "/api/secret/data/default/sec", gin.Params{{Key: "namespace", Value: "default"}, {Key: "name", Value: "sec"}})
	h.GetSecretDataHandler(c2)
	if w2.Code != http.StatusOK {
		t.Fatal(w2.Body.String())
	}
}

func TestGetEventsHandler_namespaceAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ev1 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "e1", Namespace: "default"},
		Type:       corev1.EventTypeNormal,
		Message:    "hello",
	}
	ev2 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "e2", Namespace: "kube-system"},
		Type:       corev1.EventTypeWarning,
		Message:    "warn",
	}
	cs := fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		ev1, ev2,
	)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/events?namespace=all&limit=50")
	h.GetEventsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}
