package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s-manager/api"
	"k8s-manager/internal/audit"
	"k8s-manager/internal/auth"
	"k8s-manager/internal/bootstrap"
	"k8s-manager/internal/config"
	"k8s-manager/internal/devcluster"
	"k8s-manager/internal/k8s"
	"k8s-manager/internal/rbac"
	"k8s-manager/internal/store"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

func main() {
	// Подкоманда dev-cluster: поднять/убить minikube + Kafka + Zookeeper для тестов (Postgres поднимется при старте приложения).
	if len(os.Args) >= 2 && os.Args[1] == "dev-cluster" {
		args := os.Args[2:]
		if len(args) >= 1 && args[0] == "run" {
			_ = os.Setenv("SEED_TEST_USERS", "true")
			// Одна команда: поднять кластер (minikube + Kafka + Zookeeper), затем запустить приложение (Postgres поднимется сам).
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			manifestsDir := ""
			if len(args) > 1 {
				manifestsDir = args[1]
			}
			if err := devcluster.RunStart(ctx, manifestsDir); err != nil {
				cancel()
				slog.Error("dev-cluster run: start failed", "err", err)
				os.Exit(1)
			}
			cancel()
			slog.Info("dev-cluster ready, starting application...")
		} else {
			runDevCluster(args)
			os.Exit(0)
		}
	}

	runServer()
}

func runServer() {
	cfg := config.Load()

	if !cfg.Auth.Enabled {
		slog.Error("auth required: set OIDC_GOOGLE_CLIENT_ID/OIDC_GOOGLE_CLIENT_SECRET/OIDC_REDIRECT_URL (or legacy AUTH_*)")
		os.Exit(1)
	}
	if cfg.Auth.OIDC.Enabled {
		if err := auth.SetOIDCConfig(&auth.OIDCConfig{
			ClientID:     cfg.Auth.OIDC.ClientID,
			ClientSecret: cfg.Auth.OIDC.ClientSecret,
			RedirectURL:  cfg.Auth.OIDC.RedirectURL,
			Issuer:       cfg.Auth.OIDC.Issuer,
			LogoutURL:    cfg.Auth.OIDC.LogoutURL,
			AllowedDomains: cfg.Auth.OIDC.AllowedDomains,
		}); err != nil {
			slog.Error("oidc init failed", "err", err)
			os.Exit(1)
		}
		if cfg.PostgresDSN == "" && !cfg.BootstrapPostgresInCluster {
			slog.Error("oidc requires postgres for user_permissions (set POSTGRES_DSN or enable BOOTSTRAP_POSTGRES_IN_CLUSTER)")
			os.Exit(1)
		}
	}
	rbac.SetLegacyAdminBypass(cfg.RBACLegacyAdminBypass)

	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	if err != nil {
		slog.Error("kubeconfig failed", "err", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		slog.Error("kubernetes client failed", "err", err)
		os.Exit(1)
	}

	metricsClient, err := metricsv.NewForConfig(kubeConfig)
	if err != nil {
		slog.Warn("metrics client failed, metrics disabled", "err", err)
		metricsClient = nil
	}

	postgresDSN := cfg.PostgresDSN
	if postgresDSN == "" && cfg.BootstrapPostgresInCluster {
		pgUser := os.Getenv("POSTGRES_USER")
		if pgUser == "" {
			pgUser = "k8smanager"
		}
		pgPassword := os.Getenv("POSTGRES_PASSWORD")
		if pgPassword == "" {
			pgPassword = "secret"
		}
		pgDB := os.Getenv("POSTGRES_DB")
		if pgDB == "" {
			pgDB = "k8smanager"
		}
		checkCtx, checkCancel := context.WithTimeout(context.Background(), 10*time.Second)
		existingDSN, ready, _ := bootstrap.PostgresDSNIfReady(checkCtx, clientset, cfg.BootstrapPostgresNamespace, pgUser, pgPassword, pgDB)
		checkCancel()
		if ready {
			postgresDSN = existingDSN
			slog.Info("using existing postgres in cluster", "namespace", cfg.BootstrapPostgresNamespace)
		} else {
			bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			dsn, err := bootstrap.EnsurePostgresInCluster(bootstrapCtx, clientset, cfg.BootstrapPostgresNamespace, pgUser, pgPassword, pgDB, true)
			bootstrapCancel()
			if err != nil {
				slog.Error("bootstrap postgres in cluster failed", "err", err)
				os.Exit(1)
			}
			postgresDSN = dsn
			slog.Info("postgres bootstrapped in cluster", "namespace", cfg.BootstrapPostgresNamespace)
		}
		// Если приложение запущено вне кластера, in-cluster DSN недоступен — поднимаем port-forward на localhost
		tryCtx, tryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, tryErr := store.NewPostgresStore(tryCtx, postgresDSN)
		tryCancel()
		if tryErr != nil {
			slog.Info("cannot reach in-cluster postgres (running outside cluster?), starting port-forward to localhost")
			pfCtx, pfCancel := context.WithTimeout(context.Background(), 30*time.Second)
			localDSN, pfErr := bootstrap.StartLocalPortForwardToPostgres(pfCtx, clientset, cfg.BootstrapPostgresNamespace, pgUser, pgPassword, pgDB)
			pfCancel()
			if pfErr != nil {
				slog.Error("port-forward to postgres failed", "err", pfErr)
				os.Exit(1)
			}
			postgresDSN = localDSN
		}
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")
	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	r.StaticFile("/apple-touch-icon.png", "./static/apple-touch-icon.png")
	r.StaticFile("/apple-touch-icon-precomposed.png", "./static/apple-touch-icon-precomposed.png")

	var userStore auth.UserStore
	if postgresDSN != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pgStore, err := store.NewPostgresStore(ctx, postgresDSN)
		cancel()
		if err != nil {
			slog.Error("postgres failed", "err", err)
			os.Exit(1)
		}
		defer pgStore.Close()
		userStore = pgStore
		seedCtx, seedCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = pgStore.SeedDefaultUsersIfEmpty(seedCtx,
			os.Getenv("FIRST_ADMIN_USER"), os.Getenv("FIRST_ADMIN_PASSWORD"),
			os.Getenv("FIRST_VIEWER_USER"), os.Getenv("FIRST_VIEWER_PASSWORD"))
		seedCancel()
		if os.Getenv("SEED_TEST_USERS") == "true" || os.Getenv("SEED_TEST_USERS") == "1" {
			testCtx, testCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = pgStore.SeedTestUsersAndPermissions(testCtx)
			testCancel()
		}
		auth.SetSessionStore(pgStore)
		audit.SetPersistentStore(pgStore)
		rbac.SetPermissionStore(pgStore)
		permCtx, permCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = pgStore.SeedOIDCTestPermissions(permCtx, os.Getenv("OIDC_TEST_ADMIN_EMAIL"), os.Getenv("OIDC_TEST_VIEWER_EMAIL"))
		permCancel()
	}

	var userManager store.UserManager
	if pgStore, _ := userStore.(*store.PostgresStore); pgStore != nil {
		userManager = pgStore
	}
	var permManager store.PermissionManager
	if pgStore, _ := userStore.(*store.PostgresStore); pgStore != nil {
		permManager = pgStore
	}
	api.SetupRoutes(r, clientset, metricsClient, kubeConfig, cfg, userStore, userManager, permManager)

	addr := ":" + cfg.Port
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		slog.Info("starting server", "addr", addr, "auth", cfg.Auth.Enabled)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	k8s.GetPortForwardManager().StopAllSessions()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "err", err)
	}
	slog.Info("stopped")
}

func runDevCluster(args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	sub := "start"
	if len(args) > 0 {
		sub = args[0]
	}
	manifestsDir := ""
	if len(args) > 1 {
		manifestsDir = args[1]
	}

	switch sub {
	case "start":
		if err := devcluster.RunStart(ctx, manifestsDir); err != nil {
			slog.Error("dev-cluster start failed", "err", err)
			os.Exit(1)
		}
	case "stop", "delete":
		if err := devcluster.RunStop(ctx); err != nil {
			slog.Error("dev-cluster stop failed", "err", err)
			os.Exit(1)
		}
	default:
		slog.Error("usage: dev-cluster run [manifests-dir] | dev-cluster start [manifests-dir] | dev-cluster stop")
		os.Exit(1)
	}
}
