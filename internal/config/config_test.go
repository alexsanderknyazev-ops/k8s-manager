package config

import (
	"testing"
)

func TestLoad_defaults(t *testing.T) {
	t.Setenv("BOOTSTRAP_POSTGRES_IN_CLUSTER", "false")
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("AUTH_USER", "")
	t.Setenv("AUTH_PASSWORD", "")
	t.Setenv("OIDC_GOOGLE_CLIENT_ID", "")
	t.Setenv("OIDC_GOOGLE_CLIENT_SECRET", "")
	t.Setenv("OIDC_REDIRECT_URL", "")

	c := Load()
	if c.Port != "8080" {
		t.Errorf("Port: want 8080, got %q", c.Port)
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel: want info, got %q", c.LogLevel)
	}
	if c.LogFormat != "text" {
		t.Errorf("LogFormat: want text, got %q", c.LogFormat)
	}
	if c.BootstrapPostgresNamespace != "default" {
		t.Errorf("BootstrapPostgresNamespace: want default, got %q", c.BootstrapPostgresNamespace)
	}
}

func TestLoad_portAndReadOnly(t *testing.T) {
	t.Setenv("BOOTSTRAP_POSTGRES_IN_CLUSTER", "false")
	t.Setenv("PORT", "9090")
	t.Setenv("READ_ONLY", "1")
	c := Load()
	if c.Port != "9090" {
		t.Errorf("Port: %q", c.Port)
	}
	if !c.ReadOnly {
		t.Error("ReadOnly: want true")
	}
}

func TestLoad_rateLimits(t *testing.T) {
	t.Setenv("BOOTSTRAP_POSTGRES_IN_CLUSTER", "false")
	t.Setenv("RATE_LIMIT_LOGIN_PER_MIN", "5")
	t.Setenv("RATE_LIMIT_API_PER_MIN", "0")
	c := Load()
	if c.RateLimit.LoginPerMin != 5 {
		t.Errorf("LoginPerMin: %d", c.RateLimit.LoginPerMin)
	}
	if c.RateLimit.APIPerMin != 0 {
		t.Errorf("APIPerMin: %d", c.RateLimit.APIPerMin)
	}
}

func TestLoad_rbacLegacyBypass(t *testing.T) {
	t.Setenv("BOOTSTRAP_POSTGRES_IN_CLUSTER", "false")
	t.Setenv("RBAC_LEGACY_ADMIN_BYPASS", "true")
	c := Load()
	if !c.RBACLegacyAdminBypass {
		t.Error("RBACLegacyAdminBypass: want true")
	}
}

func TestLoad_authFromEnv(t *testing.T) {
	t.Setenv("BOOTSTRAP_POSTGRES_IN_CLUSTER", "false")
	t.Setenv("AUTH_USER", "u")
	t.Setenv("AUTH_PASSWORD", "p")
	t.Setenv("POSTGRES_DSN", "")
	c := Load()
	if !c.Auth.Enabled {
		t.Error("Auth.Enabled: want true with AUTH_USER+AUTH_PASSWORD")
	}
	if c.Auth.Username != "u" || c.Auth.Password != "p" {
		t.Errorf("Auth creds: %+v", c.Auth)
	}
}
