package main

import (
	"log"
	//"net/http"
	"os"

	"k8s-manager/api"
	// "k8s-manager/internal/config"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

func main() {
	// Настройка клиента Kubernetes
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		log.Printf("Warning: Failed to create metrics client: %v", err)
		metricsClient = nil
	}

	// Настройка Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Загрузка HTML шаблонов
	r.LoadHTMLGlob("templates/*")

	// Статические файлы
	r.Static("/static", "./static")

	// Favicon
	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	r.StaticFile("/apple-touch-icon.png", "./static/apple-touch-icon.png")
	r.StaticFile("/apple-touch-icon-precomposed.png", "./static/apple-touch-icon-precomposed.png")

	// Настройка роутов
	api.SetupRoutes(r, clientset, metricsClient)

	// Запуск сервера
	log.Println("Starting K8s Manager on http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
