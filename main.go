package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"k8s-status-api/handlers"
	"k8s-status-api/k8s"

	"github.com/gin-gonic/gin"
)

// Create a custom writer that forces flush after each write
type FlushWriter struct {
	w io.Writer
}

func (fw *FlushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

func main() {
	// Force unbuffered output for printing directly to terminal
	os.Stdout.Sync()

	// Force Gin to use unbuffered output
	gin.DefaultWriter = os.Stdout
	gin.DefaultErrorWriter = os.Stderr

	// Configure logger to output to stdout with no buffering
	logOutput := os.Stdout
	logger := log.New(logOutput, "", log.LstdFlags|log.Lshortfile)
	log.SetOutput(logOutput)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Print something immediately to test terminal output
	fmt.Println("===============================================")
	fmt.Println("STARTING SERVER - TESTING TERMINAL OUTPUT")
	fmt.Println(time.Now().Format("2006/01/02 - 15:04:05"))
	fmt.Println("===============================================")
	os.Stdout.Sync() // Force flush

	// Initialize Kubernetes client
	logger.Println("Initializing Kubernetes client...")
	clientset, err := k8s.GetKubeClient()
	if err != nil {
		logger.Fatalf("Failed to create Kubernetes client: %v", err)
	}
	logger.Println("Kubernetes client initialized successfully!")

	// Set Gin mode to debug for maximum logging
	gin.SetMode(gin.DebugMode)
	logger.Println("Gin mode set to DebugMode for verbose logging")

	r := gin.New()

	// Add recovery middleware
	r.Use(gin.Recovery())

	// Add custom logger middleware that bypasses buffer
	r.Use(func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Log request details immediately with explicit flush
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		message := fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %s\n",
			time.Now().Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path)

		os.Stdout.WriteString(message)
		os.Stdout.Sync() // Force flush
	})

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	logger.Println("Routes configuration...")
	// Routes
	r.GET("/core-network", handlers.GetCoreNetworkPods(clientset))
	r.GET("/access-network", handlers.GetAccessNetworkPods(clientset))
	r.GET("/monitoring", handlers.GetMonitoringPods(clientset))
	r.POST("/install-ueransim", handlers.InstallUERANSIM())
	r.POST("/uninstall-ueransim", handlers.UninstallUERANSIM())
	r.POST("/run-traffic-test", handlers.RunBinningTrafficTest(clientset))
	r.POST("/stop-traffic-test", handlers.StopBinningTrafficTest(clientset))
	r.GET("/traffic-test-status", handlers.CheckBinningTrafficTestStatus(clientset))

	// URL List
	// http://localhost:8081/core-network
	// http://localhost:8081/access-network
	// http://localhost:8081/monitoring
	// http://localhost:8081/install-ueransim
	// http://localhost:8081/uninstall-ueransim
	// http://localhost:8081/run-traffic-test
	// http://localhost:8081/stop-traffic-test
	// http://localhost:8081/traffic-test-status

	logger.Println("Starting server with forced terminal output...")
	fmt.Println("Server ready to accept connections")
	os.Stdout.Sync() // Force flush

	// Try multiple ports
	ports := []string{":8081", ":8082", ":8083", ":8084", ":8085"}
	for _, port := range ports {
		fmt.Printf("Attempting to start server on port %s\n", port)
		os.Stdout.Sync() // Force flush
		err = r.Run(port)
		if err == nil {
			break
		}
		if err.Error() == "listen tcp "+port+": bind: address already in use" {
			fmt.Printf("Port %s is already in use, trying next port...\n", port)
			os.Stdout.Sync() // Force flush
			continue
		}
		// If it's a different error, log it and exit
		logger.Fatalf("Failed to start server: %v", err)
	}
}
