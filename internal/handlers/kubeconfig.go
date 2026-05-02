package handlers

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// GetContextsHandler returns current kubeconfig context and list of context names.
// Kubeconfig path is passed via the handler closure in routes (not stored on Handler).
func GetContextsHandler(kubeconfigPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var config *clientcmdapi.Config
		var err error
		path := strings.TrimSpace(kubeconfigPath)
		if path != "" {
			path, err = filepath.Abs(path)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			config, err = clientcmd.LoadFromFile(path)
		} else {
			rules := clientcmd.NewDefaultClientConfigLoadingRules()
			config, err = rules.Load()
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		current := config.CurrentContext
		contexts := make([]string, 0, len(config.Contexts))
		for name := range config.Contexts {
			contexts = append(contexts, name)
		}
		c.JSON(http.StatusOK, gin.H{
			"current":  current,
			"contexts": contexts,
		})
	}
}
