package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	"k8s-status-api/k8s"
)

// GetCoreNetworkPods handles requests for core network pods
func GetCoreNetworkPods(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		podList, err := k8s.GetPodsByPrefix(clientset, "open5gs")
		if err != nil {
			log.Println("Error fetching core network pods:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch core network pods"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"pods": podList})
	}
}

// GetAccessNetworkPods handles requests for access network pods
func GetAccessNetworkPods(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		podList, err := k8s.GetPodsByPrefix(clientset, "ueransim")
		if err != nil {
			log.Println("Error fetching access network pods:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch access network pods"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"pods": podList})
	}
}

// GetMonitoringPods handles requests for monitoring pods
func GetMonitoringPods(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		podList, err := k8s.GetPodsByPrefix(clientset, "prometheus")
		if err != nil {
			log.Println("Error fetching monitoring pods:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch monitoring pods"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"pods": podList})
	}
} 