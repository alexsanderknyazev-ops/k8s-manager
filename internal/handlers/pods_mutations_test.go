package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestDeletePodHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/pod/default/web-pod", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "pod", Value: "web-pod"}}
	h.DeletePodHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestUpdatePodYAMLHandler_ok(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cs := richFakeCluster(t)
	h := NewHandler(cs, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	pod, err := cs.CoreV1().Pods("default").Get(t.Context(), "web-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod.Annotations = map[string]string{"patched": "1"}
	yb, err := yaml.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"yaml": string(yb)})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/pod/yaml/default/web-pod", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "pod", Value: "web-pod"}}
	h.UpdatePodYAMLHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestGetLogsHandler_nilClientset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/logs/default/web-pod", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "pod", Value: "web-pod"}}
	h.GetLogsHandler(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

func TestDownloadLogsHandler_nilClientset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/logs/download/default/web-pod", nil)
	c.Params = gin.Params{{Key: "namespace", Value: "default"}, {Key: "pod", Value: "web-pod"}}
	h.DownloadLogsHandler(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}
