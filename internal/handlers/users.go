package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"k8s-manager/internal/store"
)

func (h *Handler) ListUsersHandler(c *gin.Context) {
	if h.userManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user management not available"})
		return
	}
	if role, _ := c.Get("role"); role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}
	list, err := h.userManager.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, u := range list {
		out = append(out, gin.H{"username": u.Username, "role": u.Role})
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h *Handler) CreateUserHandler(c *gin.Context) {
	if h.userManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user management not available"})
		return
	}
	if role, _ := c.Get("role"); role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}
	if err := h.userManager.CreateUser(c.Request.Context(), body.Username, body.Password, body.Role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "username": body.Username})
}

func (h *Handler) UpdateUserHandler(c *gin.Context) {
	if h.userManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user management not available"})
		return
	}
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username required"})
		return
	}
	currentRole, _ := c.Get("role")
	if currentRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}
	var body struct {
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if body.Role != nil {
		if *body.Role != store.RoleAdmin && *body.Role != store.RoleViewer {
			*body.Role = store.RoleViewer
		}
		if err := h.userManager.UpdateRole(c.Request.Context(), username, *body.Role); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if body.Password != nil && *body.Password != "" {
		if err := h.userManager.SetPassword(c.Request.Context(), username, *body.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) DeleteUserHandler(c *gin.Context) {
	if h.userManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user management not available"})
		return
	}
	if role, _ := c.Get("role"); role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username required"})
		return
	}
	if currentUser, _ := c.Get("username"); currentUser == username {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}
	if err := h.userManager.DeleteUser(c.Request.Context(), username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) ChangePasswordHandler(c *gin.Context) {
	if h.userManager == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "change password not available"})
		return
	}
	username, _ := c.Get("username")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uname, _ := username.(string)
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current_password and new_password required"})
		return
	}
	hash, _, err := h.userManager.GetUser(c.Request.Context(), uname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.CurrentPassword)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid current password"})
		return
	}
	if err := h.userManager.SetPassword(c.Request.Context(), uname, body.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
