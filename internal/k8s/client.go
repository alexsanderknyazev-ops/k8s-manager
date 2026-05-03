package k8s

import (
	"context"
	"log"
	// "os"
	// "path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	// "k8s.io/client-go/rest"
	// "k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	clientset     *kubernetes.Clientset
	metricsClient *metricsv.Clientset
)

func InitK8s() (*kubernetes.Clientset, *metricsv.Clientset) {
	log.Println("🔧 Initializing Kubernetes client...")

	config, err := getK8sConfig()
	if err != nil {
		log.Printf("❌ Failed to get kubeconfig: %v", err)
		return nil, nil
	}

	// Создать клиент
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("❌ Failed to create clientset: %v", err)
		return nil, nil
	}

	// Создать клиент для метрик
	metricsClient, err = metricsv.NewForConfig(config)
	if err != nil {
		log.Printf("⚠️  Failed to create metrics client: %v", err)
		log.Println("ℹ️  Make sure Metrics Server is installed: kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml")
	} else {
		log.Println("📊 Metrics client initialized")
	}

	// Тест подключения
	if err := testConnection(clientset); err != nil {
		log.Printf("⚠️  Connection test failed: %v", err)
	} else {
		log.Println("🔗 Successfully connected to Kubernetes API")
	}

	return clientset, metricsClient
}

func GetClient() *kubernetes.Clientset {
	return clientset
}

func GetMetricsClient() *metricsv.Clientset {
	return metricsClient
}

// func getK8sConfig() (*rest.Config, error) {
// 	// Попробовать получить конфиг из кластера
// 	config, err := rest.InClusterConfig()
// 	if err == nil {
// 		return config, nil
// 	}

// 	// Использовать локальный kubeconfig
// 	kubeconfig := os.Getenv("KUBECONFIG")
// 	if kubeconfig == "" {
// 		home, _ := os.UserHomeDir()
// 		kubeconfig = filepath.Join(home, ".kube", "config")
// 	}

// 	return clientcmd.BuildConfigFromFlags("", kubeconfig)
// }

func testConnection(clientset kubernetes.Interface) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	return err
}
