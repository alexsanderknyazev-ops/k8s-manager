package main

import (
	"log"
	//"os"

	"k8s-manager/api"
	"k8s-manager/internal/config"
	"k8s-manager/internal/k8s"

	"github.com/gin-gonic/gin"
)

func main() {
	// Инициализация конфигурации
	cfg := config.Load()

	// Инициализация Gin
	r := gin.Default()

	// Загружаем HTML шаблоны
	r.LoadHTMLGlob("templates/*.html")
	r.Static("/static", "./static")

	// Middleware
	r.Use(CORSMiddleware())

	// Инициализация Kubernetes клиента
	k8sClient, metricsClient := k8s.InitK8s()
	if k8sClient == nil {
		log.Fatal("❌ Failed to initialize Kubernetes client")
	}

	// Настройка маршрутов
	api.SetupRoutes(r, k8sClient, metricsClient)

	// Запуск сервера
	port := cfg.Port
	log.Printf("🚀 K8s Manager started on :%s", port)
	log.Printf("📊 Dashboard: http://localhost:%s/ui/dashboard", port)
	log.Printf("🚀 Applications: http://localhost:%s/ui/applications", port)
	log.Printf("🔧 Pods: http://localhost:%s/ui/pods", port)
	log.Printf("⚙️  Deployments: http://localhost:%s/ui/deployments", port)
	log.Printf("🛠️  Configuration: http://localhost:%s/ui/config", port)
	log.Printf("📚 API: http://localhost:%s/api", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatal("❌ Failed to start server:", err)
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
