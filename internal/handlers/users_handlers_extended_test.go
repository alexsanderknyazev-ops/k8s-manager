package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s-manager/internal/store"

	"github.com/gin-gonic/gin"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
)

type errUserMgr struct {
	mockUserMgr
	listErr    error
	createErr  error
	updateErr  error
	setPassErr error
	deleteErr  error
	getUserErr error
}

func (e *errUserMgr) ListUsers(ctx context.Context) ([]store.User, error) {
	if e.listErr != nil {
		return nil, e.listErr
	}
	return e.mockUserMgr.ListUsers(ctx)
}
func (e *errUserMgr) CreateUser(ctx context.Context, username, password, role string) error {
	if e.createErr != nil {
		return e.createErr
	}
	return e.mockUserMgr.CreateUser(ctx, username, password, role)
}
func (e *errUserMgr) UpdateRole(ctx context.Context, username, role string) error {
	if e.updateErr != nil {
		return e.updateErr
	}
	return e.mockUserMgr.UpdateRole(ctx, username, role)
}
func (e *errUserMgr) SetPassword(ctx context.Context, username, newPassword string) error {
	if e.setPassErr != nil {
		return e.setPassErr
	}
	return e.mockUserMgr.SetPassword(ctx, username, newPassword)
}
func (e *errUserMgr) DeleteUser(ctx context.Context, username string) error {
	if e.deleteErr != nil {
		return e.deleteErr
	}
	return e.mockUserMgr.DeleteUser(ctx, username)
}
func (e *errUserMgr) GetUser(ctx context.Context, username string) (string, string, error) {
	if e.getUserErr != nil {
		return "", "", e.getUserErr
	}
	return e.mockUserMgr.GetUser(ctx, username)
}

func TestListUsersHandler_noUserManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	c.Set("role", "admin")
	h.ListUsersHandler(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestListUsersHandler_listError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := &errUserMgr{listErr: errors.New("db")}
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), m, nil, nil) //nolint:staticcheck
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	c.Set("role", "admin")
	h.ListUsersHandler(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestCreateUserHandler_branches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("no mgr", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{"username":"a","password":"b"}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("role", "admin")
		h.CreateUserHandler(c)
		if w.Code != http.StatusNotFound {
			t.Fatal(w.Code)
		}
	})
	t.Run("not admin", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{"username":"a","password":"b"}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("role", "viewer")
		h.CreateUserHandler(c)
		if w.Code != http.StatusForbidden {
			t.Fatal(w.Code)
		}
	})
	t.Run("bad body", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("role", "admin")
		h.CreateUserHandler(c)
		if w.Code != http.StatusBadRequest {
			t.Fatal(w.Code)
		}
	})
	t.Run("create error", func(t *testing.T) {
		m := &errUserMgr{createErr: errors.New("fail")}
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), m, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(`{"username":"a","password":"b","role":"viewer"}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("role", "admin")
		h.CreateUserHandler(c)
		if w.Code != http.StatusInternalServerError {
			t.Fatal(w.Code)
		}
	})
}

func TestUpdateUserHandler_branches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("no mgr", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/users/x", bytes.NewReader([]byte(`{}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "username", Value: "x"}}
		c.Set("role", "admin")
		h.UpdateUserHandler(c)
		if w.Code != http.StatusNotFound {
			t.Fatal(w.Code)
		}
	})
	t.Run("empty username param", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/users/", bytes.NewReader([]byte(`{}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "username", Value: ""}}
		c.Set("role", "admin")
		h.UpdateUserHandler(c)
		if w.Code != http.StatusBadRequest {
			t.Fatal(w.Code)
		}
	})
	t.Run("not admin", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/users/x", bytes.NewReader([]byte(`{}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "username", Value: "x"}}
		c.Set("role", "viewer")
		h.UpdateUserHandler(c)
		if w.Code != http.StatusForbidden {
			t.Fatal(w.Code)
		}
	})
	t.Run("invalid json", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/users/x", bytes.NewReader([]byte(`{`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "username", Value: "x"}}
		c.Set("role", "admin")
		h.UpdateUserHandler(c)
		if w.Code != http.StatusBadRequest {
			t.Fatal(w.Code)
		}
	})
	t.Run("role invalid coerced and update err", func(t *testing.T) {
		m := &errUserMgr{updateErr: errors.New("u")}
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), m, nil, nil) //nolint:staticcheck
		body := `{"role":"not-a-real-role"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/users/x", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "username", Value: "x"}}
		c.Set("role", "admin")
		h.UpdateUserHandler(c)
		if w.Code != http.StatusInternalServerError {
			t.Fatal(w.Code)
		}
	})
	t.Run("password only ok", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		body := `{"password":"new"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/users/x", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "username", Value: "bob"}}
		c.Set("role", "admin")
		h.UpdateUserHandler(c)
		if w.Code != http.StatusOK {
			t.Fatal(w.Body.String())
		}
	})
	t.Run("password set error", func(t *testing.T) {
		m := &errUserMgr{setPassErr: errors.New("p")}
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), m, nil, nil) //nolint:staticcheck
		body := `{"password":"new"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPatch, "/api/users/x", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "username", Value: "bob"}}
		c.Set("role", "admin")
		h.UpdateUserHandler(c)
		if w.Code != http.StatusInternalServerError {
			t.Fatal(w.Code)
		}
	})
}

func TestDeleteUserHandler_branches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("no mgr", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/users/x", nil)
		c.Params = gin.Params{{Key: "username", Value: "x"}}
		c.Set("role", "admin")
		h.DeleteUserHandler(c)
		if w.Code != http.StatusNotFound {
			t.Fatal(w.Code)
		}
	})
	t.Run("delete self", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/users/me", nil)
		c.Params = gin.Params{{Key: "username", Value: "me"}}
		c.Set("role", "admin")
		c.Set("username", "me")
		h.DeleteUserHandler(c)
		if w.Code != http.StatusBadRequest {
			t.Fatal(w.Code)
		}
	})
	t.Run("delete err", func(t *testing.T) {
		m := &errUserMgr{deleteErr: errors.New("d")}
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), m, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/users/z", nil)
		c.Params = gin.Params{{Key: "username", Value: "z"}}
		c.Set("role", "admin")
		c.Set("username", "admin")
		h.DeleteUserHandler(c)
		if w.Code != http.StatusInternalServerError {
			t.Fatal(w.Code)
		}
	})
}

func TestChangePasswordHandler_branches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("no mgr", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), nil, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/me/pass", bytes.NewReader([]byte(`{"current_password":"a","new_password":"b"}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		h.ChangePasswordHandler(c)
		if w.Code != http.StatusNotFound {
			t.Fatal(w.Code)
		}
	})
	t.Run("no username in context", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/me/pass", bytes.NewReader([]byte(`{"current_password":"a","new_password":"b"}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		h.ChangePasswordHandler(c)
		if w.Code != http.StatusUnauthorized {
			t.Fatal(w.Code)
		}
	})
	t.Run("bad json", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/me/pass", bytes.NewReader([]byte(`{`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("username", "u")
		h.ChangePasswordHandler(c)
		if w.Code != http.StatusBadRequest {
			t.Fatal(w.Code)
		}
	})
	t.Run("get user error", func(t *testing.T) {
		m := &errUserMgr{getUserErr: errors.New("no")}
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), m, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/me/pass", bytes.NewReader([]byte(`{"current_password":"oldpass","new_password":"x"}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("username", "u")
		h.ChangePasswordHandler(c)
		if w.Code != http.StatusInternalServerError {
			t.Fatal(w.Code)
		}
	})
	t.Run("wrong current password", func(t *testing.T) {
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/me/pass", bytes.NewReader([]byte(`{"current_password":"bad","new_password":"x"}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("username", "alice")
		h.ChangePasswordHandler(c)
		if w.Code != http.StatusUnauthorized {
			t.Fatal(w.Code)
		}
	})
	t.Run("set password error", func(t *testing.T) {
		m := &errUserMgr{setPassErr: errors.New("sp")}
		h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), m, nil, nil) //nolint:staticcheck
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/me/pass", bytes.NewReader([]byte(`{"current_password":"oldpass","new_password":"x"}`)))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("username", "alice")
		h.ChangePasswordHandler(c)
		if w.Code != http.StatusInternalServerError {
			t.Fatal(w.Code)
		}
	})
}

func TestUpdateUserHandler_roleAdminExplicit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(fake.NewClientset(), metricsfake.NewSimpleClientset(), &mockUserMgr{}, nil, nil) //nolint:staticcheck
	body := `{"role":"admin"}`
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
