package devcluster

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// startMinikubeStable поднимает minikube с явными ресурсами; при сбое пробует другой драйвер.
// MINIKUBE_DRIVER=qemu2|docker — задать драйвер вручную. На darwin/arm64 по умолчанию пробуем qemu2 (стабильнее Docker).
func startMinikubeStable(ctx context.Context) error {
	if out, err := exec.CommandContext(ctx, "kubectl", "get", "nodes", "-o", "name").Output(); err == nil && len(out) > 0 {
		slog.Info("minikube already running, continuing")
		return nil
	}

	var drivers []string
	if d := os.Getenv("MINIKUBE_DRIVER"); d != "" {
		drivers = []string{d}
	} else if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		drivers = []string{"qemu2", "docker"}
	} else {
		drivers = []string{"docker", "qemu2"}
	}

	args := []string{"start", "--memory=4g", "--cpus=2", "--disk-size=20g"}

	for _, driver := range drivers {
		slog.Info("starting minikube", "driver", driver, "memory", "4g", "cpus", "2")
		cmd := exec.CommandContext(ctx, "minikube", append(append([]string{}, args...), "--driver="+driver)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err == nil {
			slog.Info("minikube started", "driver", driver)
			return nil
		}
		slog.Warn("minikube start failed, deleting and retrying once", "driver", driver, "err", err)
		_ = exec.CommandContext(ctx, "minikube", "delete").Run()
		time.Sleep(3 * time.Second)
		cmd2 := exec.CommandContext(ctx, "minikube", append(append([]string{}, args...), "--driver="+driver)...)
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		if err2 := cmd2.Run(); err2 == nil {
			slog.Info("minikube started on retry", "driver", driver)
			return nil
		}
		slog.Warn("minikube retry failed, trying next driver", "driver", driver)
	}
	return fmt.Errorf("minikube failed to start with all drivers (tried: %v). Set MINIKUBE_DRIVER=qemu2 or docker", drivers)
}

func kubectlApply(ctx context.Context, path string) error {
	apply := exec.CommandContext(ctx, "kubectl", "apply", "-f", path)
	apply.Stdout = os.Stdout
	apply.Stderr = os.Stderr
	if err := apply.Run(); err != nil {
		return fmt.Errorf("kubectl apply -f %s: %w", path, err)
	}
	return nil
}

func waitPodsReady(ctx context.Context, namespace, labelSelector, timeout string) {
	cmd := exec.CommandContext(ctx, "kubectl", "wait",
		"--namespace="+namespace,
		"--for=condition=ready",
		"pod",
		"-l", labelSelector,
		"--timeout="+timeout,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Warn("wait for pods", "namespace", namespace, "selector", labelSelector, "err", err)
	}
}

// deployObservability поднимает Prometheus (monitoring) и Grafana (k8s-manager) для dev.
func deployObservability(ctx context.Context, devClusterDir, deployDir string) error {
	prometheusPath := filepath.Join(devClusterDir, "prometheus.yaml")
	grafanaPath := filepath.Join(deployDir, "grafana-provisioning.yaml")

	slog.Info("deploying Prometheus and Grafana...")
	if err := kubectlApply(ctx, prometheusPath); err != nil {
		return err
	}
	if err := kubectlApply(ctx, grafanaPath); err != nil {
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	waitPodsReady(waitCtx, "monitoring", "app=prometheus-server", "180s")
	waitPodsReady(waitCtx, "k8s-manager", "app=grafana", "180s")

	slog.Info("observability ready",
		"prometheus", "monitoring/prometheus-server:80",
		"grafana", "k8s-manager/grafana:3000 (admin/admin)",
		"grafana_port_forward", "kubectl -n k8s-manager port-forward svc/grafana 3000:3000",
	)
	return nil
}

// RunStart поднимает minikube (если не запущен), Prometheus и Grafana.
// Postgres в кластере создаётся при старте приложения (bootstrap).
func RunStart(ctx context.Context, manifestsDir string) error {
	if manifestsDir == "" {
		manifestsDir = "deploy/dev-cluster"
	}
	absDir, err := filepath.Abs(manifestsDir)
	if err != nil {
		return fmt.Errorf("manifests dir: %w", err)
	}
	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		return fmt.Errorf("manifests dir not found: %s", absDir)
	}
	deployDir := filepath.Dir(absDir)

	if err := startMinikubeStable(ctx); err != nil {
		return err
	}

	slog.Info("waiting for cluster...")
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		out, err := exec.CommandContext(ctx, "kubectl", "get", "nodes", "-o", "name").Output()
		if err == nil && len(out) > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err := deployObservability(ctx, absDir, deployDir); err != nil {
		return err
	}

	slog.Info("pre-pulling postgres image (so app bootstrap is faster)...")
	pullCtx, pullCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer pullCancel()
	prepull := exec.CommandContext(pullCtx, "minikube", "image", "pull", "postgres:16-alpine")
	prepull.Stdout = os.Stdout
	prepull.Stderr = os.Stderr
	if err := prepull.Run(); err != nil {
		slog.Warn("pre-pull postgres image failed (app will pull when needed)", "err", err)
	} else {
		slog.Info("postgres image pre-pulled")
	}

	slog.Info("dev cluster started",
		"minikube", "running",
		"prometheus", "monitoring/prometheus-server",
		"grafana", "k8s-manager/grafana",
		"postgres", "run the app to bootstrap in default namespace",
	)
	return nil
}

// RunStop останавливает и удаляет кластер minikube.
func RunStop(ctx context.Context) error {
	slog.Info("stopping minikube cluster (minikube delete)...")
	cmd := exec.CommandContext(ctx, "minikube", "delete")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("minikube delete: %w", err)
	}
	slog.Info("minikube cluster deleted")
	return nil
}
