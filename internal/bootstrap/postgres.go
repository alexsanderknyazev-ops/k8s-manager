package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"k8s-manager/internal/k8s"
)

const (
	serviceName   = "postgres"
	labelApp      = "app"
	labelAppValue = "postgres"
)

// PostgresDSNIfReady возвращает DSN и true, если Postgres уже развёрнут в кластере и готов (под не создаём).
func PostgresDSNIfReady(ctx context.Context, clientset kubernetes.Interface, namespace, user, password, db string) (dsn string, ready bool, err error) {
	if namespace == "" {
		namespace = "default"
	}
	dep, err := clientset.AppsV1().Deployments(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", false, nil // нет деплоймента — не готов
	}
	if dep.Status.AvailableReplicas < 1 {
		return "", false, nil
	}
	dsn = buildDSN(user, password, serviceName, namespace, db)
	return dsn, true, nil
}

// EnsurePostgresInCluster создаёт в кластере Namespace (если нужно), Secret, ConfigMap, PVC, Service и Deployment
// для PostgreSQL, ждёт готовности пода и возвращает DSN для подключения.
// Ресурсы уже существующие не перезаписываются (Create с AlreadyExists).
// namespace, user, password, db — параметры БД; useExistingNamespace — не создавать namespace, если уже есть.
func EnsurePostgresInCluster(ctx context.Context, clientset kubernetes.Interface, namespace, user, password, db string, useExistingNamespace bool) (dsn string, err error) {
	if namespace == "" {
		namespace = "default"
	}
	if user == "" {
		user = "k8smanager"
	}
	if db == "" {
		db = "k8smanager"
	}

	// Создаём namespace, если не default и не используем существующий
	if namespace != "default" && !useExistingNamespace {
		_, err = clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}, metav1.CreateOptions{})
		if err != nil && !isAlreadyExists(err) {
			return "", fmt.Errorf("create namespace %s: %w", namespace, err)
		}
		if err == nil {
			slog.Info("created namespace for postgres", "namespace", namespace)
		}
	}

	secretName := "postgres-secret"
	configMapName := "postgres-config"
	pvcName := "postgres-pvc"

	// Secret с паролем
	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"password": []byte(password)},
	}, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return "", fmt.Errorf("create secret: %w", err)
	}
	if err == nil {
		slog.Info("created postgres secret", "namespace", namespace)
	}

	// ConfigMap
	_, err = clientset.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data: map[string]string{
			"POSTGRES_DB":   db,
			"POSTGRES_USER": user,
		},
	}, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return "", fmt.Errorf("create configmap: %w", err)
	}
	if err == nil {
		slog.Info("created postgres configmap", "namespace", namespace)
	}

	// PVC
	_, err = clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: pvcName},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: mustParseQuantity("1Gi"),
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return "", fmt.Errorf("create pvc: %w", err)
	}
	if err == nil {
		slog.Info("created postgres pvc", "namespace", namespace)
	}

	// Service
	_, err = clientset.CoreV1().Services(namespace).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: serviceName},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 5432, TargetPort: intstr.FromInt(5432), Protocol: corev1.ProtocolTCP},
			},
			Selector: map[string]string{labelApp: labelAppValue},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return "", fmt.Errorf("create service: %w", err)
	}
	if err == nil {
		slog.Info("created postgres service", "namespace", namespace)
	}

	// Deployment
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: serviceName},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: labelAppValue}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelApp: labelAppValue}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: "postgres:16-alpine",
							Ports: []corev1.ContainerPort{{ContainerPort: 5432}},
							Env: []corev1.EnvVar{
								{Name: "POSTGRES_DB", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: configMapName}, Key: "POSTGRES_DB"}}},
								{Name: "POSTGRES_USER", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: configMapName}, Key: "POSTGRES_USER"}}},
								{Name: "POSTGRES_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "password"}}},
							},
							VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/postgresql/data"}},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(5432)}},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(5432)}},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: mustParseQuantity("256Mi"),
									corev1.ResourceCPU:    mustParseQuantity("250m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: mustParseQuantity("512Mi"),
									corev1.ResourceCPU:    mustParseQuantity("500m"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}},
					},
				},
			},
		},
	}
	_, err = clientset.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return "", fmt.Errorf("create deployment: %w", err)
	}
	if err == nil {
		slog.Info("created postgres deployment", "namespace", namespace)
	}

	// Ждём, пока PVC станет Bound (иначе под будет в Pending и 5 мин пройдут впустую)
	if err := waitForPVCBound(ctx, clientset, namespace, pvcName, 90*time.Second); err != nil {
		slog.Warn("postgres pvc not bound in time, continuing anyway", "err", err)
	}

	// Ждём готовности (5 мин: первый pull образа и initdb на Minikube могут быть долгими)
	dsn = buildDSN(user, password, serviceName, namespace, db)
	if err := waitForPostgresReady(ctx, clientset, namespace, 5*time.Minute); err != nil {
		return dsn, err // возвращаем dsn, чтобы можно было повторить подключение
	}
	return dsn, nil
}

func buildDSN(user, password, svc, namespace, db string) string {
	password = url.QueryEscape(password)
	// In-cluster: postgres.namespace.svc.cluster.local
	host := fmt.Sprintf("%s.%s.svc.cluster.local", svc, namespace)
	return fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", user, password, host, db)
}

func waitForPVCBound(ctx context.Context, clientset kubernetes.Interface, namespace, pvcName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			select { case <-ctx.Done(): return ctx.Err(); case <-ticker.C: }
			continue
		}
		if pvc.Status.Phase == corev1.ClaimBound {
			slog.Info("postgres pvc bound", "namespace", namespace)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return fmt.Errorf("pvc %s not bound within %v", pvcName, timeout)
}

func waitForPostgresReady(ctx context.Context, clientset kubernetes.Interface, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	logTicker := time.NewTicker(15 * time.Second) // диагностика раз в 15 сек
	defer ticker.Stop()
	defer logTicker.Stop()
	for {
		if time.Now().After(deadline) {
			logPostgresStatus(ctx, clientset, namespace)
			return fmt.Errorf("postgres not ready within %v (check: kubectl get pods -n %s -l app=postgres; kubectl describe pvc -n %s)", timeout, namespace, namespace)
		}
		dep, err := clientset.AppsV1().Deployments(namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				continue
			}
		}
		if dep.Status.AvailableReplicas >= 1 {
			slog.Info("postgres deployment is ready", "namespace", namespace)
			return nil
		}
		// Периодически логировать статус (раз в 15 сек)
		select {
		case <-logTicker.C:
			logPostgresStatus(ctx, clientset, namespace)
		default:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// logPostgresStatus пишет в лог статус Deployment и подов Postgres для диагностики.
func logPostgresStatus(ctx context.Context, clientset kubernetes.Interface, namespace string) {
	dep, err := clientset.AppsV1().Deployments(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		slog.Warn("postgres status: get deployment failed", "err", err)
		return
	}
	slog.Info("postgres status",
		"replicas", dep.Status.Replicas,
		"ready", dep.Status.ReadyReplicas,
		"available", dep.Status.AvailableReplicas,
		"updated", dep.Status.UpdatedReplicas)
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelApp + "=" + labelAppValue})
	if err != nil {
		return
	}
	for _, p := range pods.Items {
		reason := ""
		for _, c := range p.Status.Conditions {
			if c.Status != corev1.ConditionTrue && c.Message != "" {
				reason = c.Reason + ": " + c.Message
				break
			}
		}
		if reason == "" && len(p.Status.ContainerStatuses) > 0 {
			cs := p.Status.ContainerStatuses[0]
			if !cs.Ready && cs.State.Waiting != nil {
				reason = cs.State.Waiting.Reason + ": " + cs.State.Waiting.Message
			}
		}
		slog.Info("postgres pod", "name", p.Name, "phase", p.Status.Phase, "reason", reason)
	}
}

func isAlreadyExists(err error) bool {
	return apierrors.IsAlreadyExists(err)
}

func mustParseQuantity(s string) resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}

const bootstrapPFID = "bootstrap-postgres"
const localPostgresPort = 15432

// StartLocalPortForwardToPostgres находит под Postgres в namespace, поднимает port-forward
// localPort -> pod:5432 и возвращает DSN для подключения к localhost. Используется при запуске
// приложения вне кластера после bootstrap. Сессия регистрируется в k8s.GetPortForwardManager()
// и будет остановлена при StopAllSessions (например при shutdown).
func StartLocalPortForwardToPostgres(ctx context.Context, clientset kubernetes.Interface, namespace, user, password, db string) (localDSN string, err error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelApp + "=" + labelAppValue})
	if err != nil {
		return "", fmt.Errorf("list postgres pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no postgres pod found in namespace %s", namespace)
	}
	var podName string
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning {
			podName = p.Name
			break
		}
	}
	if podName == "" {
		podName = pods.Items[0].Name
	}
	localPort := localPostgresPort
	if k8s.IsPortInUse(localPort) {
		return "", fmt.Errorf("local port %d already in use", localPort)
	}
	session := &k8s.PortForwardSession{
		ID:         bootstrapPFID,
		Pod:        podName,
		Namespace:  namespace,
		LocalPort:  localPort,
		RemotePort: 5432,
		Status:     "starting",
		CreatedAt:  time.Now(),
		StopChan:   make(chan struct{}),
	}
	k8s.GetPortForwardManager().AddSession(session)
	go k8s.StartPortForward(session, clientset)
	// Ждём, пока порт начнёт принимать соединения
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", localPort)), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			password = url.QueryEscape(password)
			localDSN = fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s?sslmode=disable", user, password, localPort, db)
			slog.Info("port-forward to postgres ready", "local_port", localPort, "pod", podName)
			return localDSN, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("port-forward to postgres did not become ready in time")
}
