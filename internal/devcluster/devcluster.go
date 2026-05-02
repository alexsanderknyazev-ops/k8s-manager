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

	// Порядок драйверов: на Mac ARM часто стабильнее qemu2; можно задать MINIKUBE_DRIVER
	drivers := []string{}
	if d := os.Getenv("MINIKUBE_DRIVER"); d != "" {
		drivers = []string{d}
	} else if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		drivers = []string{"qemu2", "docker"}
	} else {
		drivers = []string{"docker", "qemu2"}
	}

	// Явные ресурсы уменьшают "container exited unexpectedly"
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
		// При сбое — удаляем и пробуем ещё раз с тем же драйвером (часто помогает после delete)
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

// RunStart поднимает minikube (если не запущен), затем разворачивает в нём Zookeeper, Kafka и Postgres для тестов.
// Postgres при первом запуске приложения поднимется сам (bootstrap). Здесь только minikube + Kafka + Zookeeper.
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

	// Стабильный запуск minikube: явные ресурсы, выбор драйвера, повтор при сбое
	if err := startMinikubeStable(ctx); err != nil {
		return err
	}

	// Ждём, пока kubectl начнёт отвечать
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

	// Namespace, затем Zookeeper, затем Kafka (порядок важен: namespace должен быть первым)
	slog.Info("deploying namespace, Zookeeper and Kafka...")
	for _, name := range []string{"namespace.yaml", "zookeeper.yaml", "kafka.yaml"} {
		path := filepath.Join(absDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		apply := exec.CommandContext(ctx, "kubectl", "apply", "-f", path)
		apply.Stdout = os.Stdout
		apply.Stderr = os.Stderr
		if err := apply.Run(); err != nil {
			return fmt.Errorf("kubectl apply -f %s: %w", name, err)
		}
	}

	slog.Info("waiting for Zookeeper and Kafka pods (up to 2 min)...")
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	_ = exec.CommandContext(waitCtx, "kubectl", "wait", "--namespace=market", "--for=condition=ready", "pod", "-l", "app=zookeeper", "--timeout=120s").Run()
	_ = exec.CommandContext(waitCtx, "kubectl", "wait", "--namespace=market", "--for=condition=ready", "pod", "-l", "app=kafka-service", "--timeout=120s").Run()

	// Предзагрузка образа Postgres в ноду minikube (не создаём под — не нужен ServiceAccount)
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
		"kafka", "market/kafka-service",
		"zookeeper", "market/zookeeper",
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
