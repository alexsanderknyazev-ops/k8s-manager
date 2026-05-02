package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	Kubeconfig  string
	Auth        AuthConfig
	PostgresDSN                string // если задан — логин/пароль/роли из БД
	BootstrapPostgresInCluster bool   // при старте развернуть PostgreSQL в кластере
	BootstrapPostgresNamespace string // namespace для развёрнутого Postgres
	ReadOnly                   bool
	LogLevel                   string // debug, info, warn, error
	LogFormat   string // json, text
	RateLimit   RateLimitConfig
	RBACLegacyAdminBypass bool
}

type AuthConfig struct {
	Enabled      bool
	Username     string
	Password     string
	PasswordHash string
	OIDC         OIDCConfig
}

type OIDCConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Issuer       string
	LogoutURL    string
	AllowedDomains []string
}

type RateLimitConfig struct {
	LoginPerMin int // запросов на /api/login с одного IP в минуту
	APIPerMin   int // запросов на /api/* с одного IP в минуту (0 = без лимита)
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	postgresDSN := os.Getenv("POSTGRES_DSN")
	if postgresDSN == "" {
		postgresDSN = os.Getenv("DATABASE_URL")
	}
	// По умолчанию поднимаем Postgres в кластере, если DSN не задан; отключить: BOOTSTRAP_POSTGRES_IN_CLUSTER=false
	bootstrapEnv := os.Getenv("BOOTSTRAP_POSTGRES_IN_CLUSTER")
	bootstrapPostgres := bootstrapEnv != "false" && bootstrapEnv != "0" && (bootstrapEnv == "true" || bootstrapEnv == "1" || (postgresDSN == "" && bootstrapEnv == ""))
	bootstrapNS := os.Getenv("BOOTSTRAP_POSTGRES_NAMESPACE")
	if bootstrapNS == "" {
		bootstrapNS = "default"
	}
	authUser := os.Getenv("AUTH_USER")
	authPass := os.Getenv("AUTH_PASSWORD")
	authHash := os.Getenv("AUTH_PASSWORD_HASH")
	oidcClientID := os.Getenv("OIDC_GOOGLE_CLIENT_ID")
	oidcClientSecret := os.Getenv("OIDC_GOOGLE_CLIENT_SECRET")
	oidcRedirectURL := os.Getenv("OIDC_REDIRECT_URL")
	oidcIssuer := os.Getenv("OIDC_ISSUER")
	if oidcIssuer == "" {
		oidcIssuer = "https://accounts.google.com"
	}
	oidcEnabled := (oidcClientID != "" && oidcClientSecret != "" && oidcRedirectURL != "")
	oidcLogoutURL := os.Getenv("OIDC_LOGOUT_URL")
	var oidcAllowedDomains []string
	if v := os.Getenv("OIDC_ALLOWED_EMAIL_DOMAINS"); v != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(strings.ToLower(p))
			if p != "" {
				oidcAllowedDomains = append(oidcAllowedDomains, p)
			}
		}
	}
	authEnabled := oidcEnabled || postgresDSN != "" || bootstrapPostgres || (authUser != "" && (authPass != "" || authHash != ""))

	readOnly := os.Getenv("READ_ONLY") == "true" || os.Getenv("READ_ONLY") == "1"

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "text"
	}

	loginPerMin := 10
	if v := os.Getenv("RATE_LIMIT_LOGIN_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			loginPerMin = n
		}
	}
	apiPerMin := 300
	if v := os.Getenv("RATE_LIMIT_API_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			apiPerMin = n
		}
	}
	rbacLegacyBypass := os.Getenv("RBAC_LEGACY_ADMIN_BYPASS") == "true" || os.Getenv("RBAC_LEGACY_ADMIN_BYPASS") == "1"

	return &Config{
		Port:                       port,
		Kubeconfig:                 kubeconfig,
		Auth: AuthConfig{
			Enabled:      authEnabled,
			Username:     authUser,
			Password:     authPass,
			PasswordHash: authHash,
			OIDC: OIDCConfig{
				Enabled:      oidcEnabled,
				ClientID:     oidcClientID,
				ClientSecret: oidcClientSecret,
				RedirectURL:  oidcRedirectURL,
				Issuer:       oidcIssuer,
				LogoutURL:    oidcLogoutURL,
				AllowedDomains: oidcAllowedDomains,
			},
		},
		PostgresDSN:                postgresDSN,
		BootstrapPostgresInCluster: bootstrapPostgres,
		BootstrapPostgresNamespace: bootstrapNS,
		ReadOnly:                   readOnly,
		LogLevel:                   logLevel,
		LogFormat:                  logFormat,
		RateLimit:                  RateLimitConfig{LoginPerMin: loginPerMin, APIPerMin: apiPerMin},
		RBACLegacyAdminBypass:      rbacLegacyBypass,
	}
}
