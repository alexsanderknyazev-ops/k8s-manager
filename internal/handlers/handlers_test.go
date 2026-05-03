package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func fakeHandler(cs *fake.Clientset) *Handler {
	if cs == nil {
		cs = fake.NewClientset()
	}
	return NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
}

// Gin CreateTestContext не задаёт Request — без него Context() падает.
func ctxGET(w *httptest.ResponseRecorder, path string) *gin.Context {
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	return c
}

func TestUserManagementAvailable_false(t *testing.T) {
	h := fakeHandler(nil)
	if h.UserManagementAvailable() {
		t.Error("want false without user manager")
	}
	if h.PermissionManagementAvailable() {
		t.Error("want false without perm manager")
	}
}

func TestHomeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := fakeHandler(fake.NewClientset())
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/")
	h.HomeHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "connected" {
		t.Errorf("status field: %v", body["status"])
	}
}

func TestHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := fakeHandler(fake.NewClientset())
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/health")
	h.HealthHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestTestConnectionHandler_success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	})
	h := fakeHandler(cs)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/test")
	h.TestConnectionHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestTestConnectionHandler_noClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/test")
	h.TestConnectionHandler(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

func TestGetNamespacesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	})
	h := fakeHandler(cs)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/namespaces")
	h.GetNamespacesHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestGetNodesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.0"},
		},
	})
	h := fakeHandler(cs)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/nodes")
	h.GetNodesHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestOpenAPIDocsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := fakeHandler(nil)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/docs")
	h.OpenAPIDocsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestDeployTemplatesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := fakeHandler(nil)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/deploy/templates")
	h.DeployTemplatesHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestGetApplicationsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rep := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &rep,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "nginx"}}},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	cs := fake.NewClientset()
	_, err := cs.AppsV1().Deployments("default").Create(t.Context(), dep, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h := fakeHandler(cs)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/applications?namespace=default", nil)
	h.GetApplicationsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
}

func TestSearchHandler_emptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := fakeHandler(fake.NewClientset())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/search?q=", nil)
	h.SearchHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestSearchHandler_matchesPod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "nginx"}}},
	}
	cs := fake.NewClientset(pod)
	h := fakeHandler(cs)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/search?q=nginx&namespace=default", nil)
	h.SearchHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	pods, _ := body["pods"].([]any)
	if len(pods) < 1 {
		t.Fatalf("expected pods, got %v", body)
	}
}

func TestSimpleDeployHandler_createsDeployment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset()
	h := fakeHandler(cs)
	body := `{"name":"app1","namespace":"default","image":"nginx:alpine","replicas":1,"container_port":80,"create_service":false}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/deploy/simple", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SimpleDeployHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestSimpleDeployHandler_validationError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := fakeHandler(fake.NewClientset())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/deploy/simple", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	h.SimpleDeployHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSimpleDeployHandler_withService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset()
	h := fakeHandler(cs)
	body := `{"name":"svcapp","namespace":"default","image":"nginx:alpine","replicas":1,"container_port":8080,"create_service":true,"service_port":80}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/deploy/simple", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	h.SimpleDeployHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
}

func TestGetApplicationsHandler_noClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/applications")
	h.GetApplicationsHandler(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

func TestSearchHandler_noClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/search?q=x", nil)
	h.SearchHandler(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

func TestSimpleDeployHandler_missingNameImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := fakeHandler(fake.NewClientset())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/deploy/simple", bytes.NewReader([]byte(`{"name":"","image":""}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	h.SimpleDeployHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestHomeHandler_disconnectedWhenNilClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, nil, nil, nil, nil)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/")
	h.HomeHandler(c)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "disconnected" {
		t.Errorf("want disconnected, got %v", body["status"])
	}
}

func TestHealthHandler_nilClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, nil, nil, nil, nil)
	w := httptest.NewRecorder()
	c := ctxGET(w, "/api/health")
	h.HealthHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestSimpleDeployHandler_servicePortDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := fake.NewClientset()
	h := fakeHandler(cs)
	body := `{"name":"sp","namespace":"default","image":"nginx","replicas":1,"container_port":3000,"create_service":true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/deploy/simple", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	h.SimpleDeployHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	// verify service port follows container when service_port omitted
	svc, err := cs.CoreV1().Services("default").Get(t.Context(), "sp", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Spec.Ports[0].TargetPort.IntValue() != 3000 {
		t.Errorf("TargetPort: %+v", svc.Spec.Ports[0].TargetPort)
	}
}
