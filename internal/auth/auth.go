package auth

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	"k8s-manager/internal/middleware"
	"k8s-manager/internal/sessionstore"
)

// UserStore — источник пользователей (логин, хэш пароля, роль) для проверки при логине.
type UserStore interface {
	GetUser(ctx context.Context, username string) (passwordHash, role string, err error)
}

const (
	cookieName = "k8s_manager_session"
	cookiePath = "/"
	maxAge     = 24 * 60 * 60 // 24 hours
)

var sessionStore sessionstore.Store = sessionstore.NewMemoryStore()
var oidcCfg *OIDCConfig

type OIDCConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Issuer       string
	LogoutURL    string
	AllowedDomains []string
}

var (
	oidcStateMu sync.Mutex
	oidcStates  = map[string]oidcState{}
	oidcVerifier *gooidc.IDTokenVerifier
)

type oidcState struct {
	Next      string
	Nonce     string
	ExpiresAt time.Time
}

// SetSessionStore задаёт хранилище сессий (по умолчанию — память; при Postgres — auth.SetSessionStore(pgStore)).
func SetSessionStore(s sessionstore.Store) {
	if s != nil {
		sessionStore = s
	}
}

func CreateSession(ctx context.Context, username, role string) (string, error) {
	return sessionStore.CreateSession(ctx, username, role)
}

func GetSession(ctx context.Context, sessionID string) (username, role string, ok bool) {
	return sessionStore.GetSession(ctx, sessionID)
}

func DeleteSession(ctx context.Context, sessionID string) {
	sessionStore.DeleteSession(ctx, sessionID)
}

type sessionInvalidator interface {
	DeleteSessionsByUsername(ctx context.Context, username string) error
}

func InvalidateUserSessions(ctx context.Context, username string) error {
	if username == "" {
		return nil
	}
	if si, ok := sessionStore.(sessionInvalidator); ok {
		return si.DeleteSessionsByUsername(ctx, username)
	}
	return nil
}

func SetOIDCConfig(cfg *OIDCConfig) error {
	oidcCfg = cfg
	if cfg == nil {
		oidcVerifier = nil
		return nil
	}
	issuer := cfg.Issuer
	if issuer == "" {
		issuer = "https://accounts.google.com"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	provider, err := gooidc.NewProvider(ctx, issuer)
	if err != nil {
		return err
	}
	oidcVerifier = provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID})
	return nil
}

// Public paths — без проверки сессии (логин, health для k8s проб, статика).
func isPublicPath(path string) bool {
	if path == "/login" || path == "/favicon.ico" ||
		path == "/apple-touch-icon.png" || path == "/apple-touch-icon-precomposed.png" {
		return true
	}
	if path == "/api/health" || path == "/metrics" {
		return true
	}
	if len(path) >= 7 && path[:7] == "/static" {
		return true
	}
	if len(path) >= 10 && path[:10] == "/api/login" {
		return true
	}
	if len(path) >= 11 && path[:11] == "/api/logout" {
		return true
	}
	if len(path) >= 14 && path[:14] == "/api/auth/oidc" {
		return true
	}
	return false
}

// Middleware returns a Gin handler that requires a valid session.
// If no session or invalid, redirects to /login (HTML) or 401 (API).
// Публичные пути (login, static, api/login, api/logout) пропускаются.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		cookie, err := c.Cookie(cookieName)
		if err != nil || cookie == "" {
			redirectOr401(c)
			return
		}
		username, role, ok := GetSession(c.Request.Context(), cookie)
		if !ok {
			redirectOr401(c)
			return
		}
		c.Set("username", username)
		c.Set("role", role)
		c.Next()
	}
}

func rejectLogin(c *gin.Context) {
	wantJSON := c.GetHeader("Accept") == "application/json" || c.ContentType() == "application/json"
	if wantJSON {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title": "Login",
		"Error": "Invalid username or password",
		"Next":  c.Query("next"),
	})
}

func redirectOr401(c *gin.Context) {
	if isAPIRequest(c.Request) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Redirect(http.StatusFound, "/login?next="+c.Request.URL.Path)
	c.Abort()
}

func isAPIRequest(r *http.Request) bool {
	return len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api"
}

func cookieSecure() bool {
	v := os.Getenv("COOKIE_SECURE")
	return v == "true" || v == "1"
}

func SetSessionCookie(c *gin.Context, sessionID string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     cookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(),
	})
}

func ClearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     cookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// CheckPassword сравнивает пароль с plain или bcrypt-хэшем из конфига.
func CheckPassword(password, expectedPlain, expectedHash string) bool {
	if expectedHash != "" {
		err := bcrypt.CompareHashAndPassword([]byte(expectedHash), []byte(password))
		return err == nil
	}
	return password == expectedPlain
}

// Login checks credentials and creates a session. Accepts form or JSON.
// If userStore != nil — логин/пароль/роль из БД; иначе expectedUser, expectedPass, expectedHash из конфига.
func Login(c *gin.Context, userStore UserStore, expectedUser, expectedPass, expectedHash string) {
	var username, password string
	if c.ContentType() == "application/json" {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		username, password = body.Username, body.Password
	} else {
		username = c.PostForm("username")
		password = c.PostForm("password")
	}

	var role string
	if userStore != nil {
		hash, r, err := userStore.GetUser(c.Request.Context(), username)
		if err != nil || hash == "" {
			rejectLogin(c)
			return
		}
		if !CheckPassword(password, "", hash) {
			rejectLogin(c)
			return
		}
		role = r
	} else {
		if username != expectedUser || !CheckPassword(password, expectedPass, expectedHash) {
			rejectLogin(c)
			return
		}
		role = "" // env user = полные права (как admin)
	}

	sessionID, err := CreateSession(c.Request.Context(), username, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session error"})
		return
	}

	SetSessionCookie(c, sessionID)
	if token, err := middleware.GenerateCSRFToken(); err == nil {
		middleware.SetCSRFCookie(c, token, cookieSecure())
	}
	next := c.Query("next")
	if next == "" || next[0] != '/' {
		next = "/ui/dashboard"
	}

	if c.ContentType() == "application/json" || c.GetHeader("Accept") == "application/json" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "redirect": next})
		return
	}
	c.Redirect(http.StatusFound, next)
}

// Logout clears session and cookie.
func Logout(c *gin.Context) {
	cookie, _ := c.Cookie(cookieName)
	if cookie != "" {
		DeleteSession(c.Request.Context(), cookie)
	}
	ClearSessionCookie(c)
	if c.GetHeader("Accept") == "application/json" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	if oidcCfg != nil && oidcCfg.LogoutURL != "" {
		c.Redirect(http.StatusFound, oidcCfg.LogoutURL)
		return
	}
	c.Redirect(http.StatusFound, "/login")
}

func oidcOAuthConfig() *oauth2.Config {
	if oidcCfg == nil {
		return nil
	}
	return &oauth2.Config{
		ClientID:     oidcCfg.ClientID,
		ClientSecret: oidcCfg.ClientSecret,
		RedirectURL:  oidcCfg.RedirectURL,
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
}

func OIDCLogin(c *gin.Context) {
	cfg := oidcOAuthConfig()
	if cfg == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "oidc not configured"})
		return
	}
	next := c.Query("next")
	if next == "" || next[0] != '/' {
		next = "/ui/dashboard"
	}
	state, err := middleware.GenerateCSRFToken()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "oidc state generation failed"})
		return
	}
	nonce, err := middleware.GenerateCSRFToken()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "oidc nonce generation failed"})
		return
	}
	oidcStateMu.Lock()
	oidcStates[state] = oidcState{Next: next, Nonce: nonce, ExpiresAt: time.Now().Add(10 * time.Minute)}
	oidcStateMu.Unlock()
	c.Redirect(http.StatusFound, cfg.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("nonce", nonce)))
}

func OIDCCallback(c *gin.Context) {
	cfg := oidcOAuthConfig()
	if cfg == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "oidc not configured"})
		return
	}
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing oidc state or code"})
		return
	}
	oidcStateMu.Lock()
	s, ok := oidcStates[state]
	delete(oidcStates, state)
	oidcStateMu.Unlock()
	if !ok || time.Now().After(s.ExpiresAt) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid oidc state"})
		return
	}
	tok, err := cfg.Exchange(c.Request.Context(), code)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "oidc exchange failed"})
		return
	}
	rawIDToken, _ := tok.Extra("id_token").(string)
	if rawIDToken == "" || oidcVerifier == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "oidc id_token missing"})
		return
	}
	idToken, err := oidcVerifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "oidc token verify failed"})
		return
	}
	var info struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Nonce         string `json:"nonce"`
		HostedDomain  string `json:"hd"`
	}
	if err := idToken.Claims(&info); err != nil || info.Email == "" || !info.EmailVerified {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "oidc email claim missing"})
		return
	}
	if info.Nonce == "" || info.Nonce != s.Nonce {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "oidc nonce mismatch"})
		return
	}
	if len(oidcCfg.AllowedDomains) > 0 {
		emailDomain := ""
		if i := strings.LastIndex(info.Email, "@"); i >= 0 && i < len(info.Email)-1 {
			emailDomain = strings.ToLower(info.Email[i+1:])
		}
		hd := strings.ToLower(info.HostedDomain)
		allowed := false
		for _, d := range oidcCfg.AllowedDomains {
			if emailDomain == d || hd == d {
				allowed = true
				break
			}
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "oidc domain is not allowed"})
			return
		}
	}
	sessionID, err := CreateSession(c.Request.Context(), info.Email, "")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "session error"})
		return
	}
	SetSessionCookie(c, sessionID)
	if token, err := middleware.GenerateCSRFToken(); err == nil {
		middleware.SetCSRFCookie(c, token, cookieSecure())
	}
	c.Redirect(http.StatusFound, s.Next)
}
