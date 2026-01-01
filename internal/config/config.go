package config

import "os"

type Config struct {
	Port       string
	Kubeconfig string
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	kubeconfig := os.Getenv("KUBECONFIG")

	return &Config{
		Port:       port,
		Kubeconfig: kubeconfig,
	}
}
