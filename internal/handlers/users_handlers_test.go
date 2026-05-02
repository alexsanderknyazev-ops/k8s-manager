package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s-manager/internal/store"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

type mockUserMgr struct{}

func (mockUserMgr) GetUser(ctx context.Context, username string) (passwordHash, role string, err error) {
	h, err := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.MinCost)
	if err != nil {
		return "", "", err
	}
	return string(h), store.RoleViewer, nil
}

func (mockUserMgr) ListUsers(ctx context.Context) ([]store.User, error) {
	return []store.User{{Username: "alice", Role: store.RoleAdmin}}, nil
}
func (mockUserMgr) CreateUser(ctx context.Context, username, password, role string) error {
	return nil
}
func (mockUserMgr) UpdateRole(ctx context.Context, username, role string) error { return nil }
func (mockUserMgr) SetPassword(ctx context.Context, username, newPassword string) error {
	return nil
}
func (mockUserMgr) DeleteUser(ctx context.Context, username string) error { return nil }

func TestListUsersHandler_admin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	c.Set("role", "admin")
	h.ListUsersHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestListUsersHandler_notAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	c.Set("role", "viewer")
	h.ListUsersHandler(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestCreateUserHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"username":"bob","password":"secret","role":"viewer"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", "admin")
	h.CreateUserHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestDeleteUserHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/users/other", nil)
	c.Params = gin.Params{{Key: "username", Value: "other"}}
	c.Set("role", "admin")
	c.Set("username", "admin")
	h.DeleteUserHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestUpdateUserHandler_role(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
	body := `{"role":"viewer"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/users/bob", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "username", Value: "bob"}}
	c.Set("role", "admin")
	h.UpdateUserHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestChangePasswordHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
	body := `{"current_password":"oldpass","new_password":"newpass"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/users/me/password", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("username", "alice")
	h.ChangePasswordHandler(c)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}
