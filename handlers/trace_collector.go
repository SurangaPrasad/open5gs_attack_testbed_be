package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"context"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Configuration struct for trace collector
type TraceCollectorConfig struct {
	Namespace         string
	ContainerName     string
	DestinationPath   string
	LocalDestination  string
	CheckIntervalSecs int
	IsRunning         bool
	StopChan          chan struct{}
}

// Global configuration for trace collector
var traceConfig = TraceCollectorConfig{
	Namespace:         "default",
	ContainerName:     "trace-collector",
	DestinationPath:   "/usr/src/app/pcap_files",
	LocalDestination:  "./pcap_files",
	CheckIntervalSecs: 10,
	IsRunning:         false,
	StopChan:          make(chan struct{}),
}

// StartTraceCollector initializes and starts the trace collector
func StartTraceCollector(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		if traceConfig.IsRunning {
			c.JSON(http.StatusOK, gin.H{
				"message": "Trace collector is already running",
			})
			return
		}

		// Ensure local destination directory exists
		if err := os.MkdirAll(traceConfig.LocalDestination, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create local directory: %v", err),
			})
			return
		}

		// Create a fresh stop channel
		traceConfig.StopChan = make(chan struct{})
		traceConfig.IsRunning = true

		// Start the trace collector in a goroutine
		go collectTraces(clientset, traceConfig.StopChan)

		c.JSON(http.StatusOK, gin.H{
			"message": "Trace collector started successfully",
			"config": gin.H{
				"namespace":         traceConfig.Namespace,
				"container_name":    traceConfig.ContainerName,
				"destination_path":  traceConfig.DestinationPath,
				"local_destination": traceConfig.LocalDestination,
				"check_interval":    traceConfig.CheckIntervalSecs,
			},
		})
	}
}

// StopTraceCollector stops the trace collector
func StopTraceCollector() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !traceConfig.IsRunning {
			c.JSON(http.StatusOK, gin.H{
				"message": "Trace collector is not running",
			})
			return
		}

		// Signal the collector to stop
		close(traceConfig.StopChan)
		traceConfig.IsRunning = false

		c.JSON(http.StatusOK, gin.H{
			"message": "Trace collector stopped successfully",
		})
	}
}

// GetTraceCollectorStatus returns the current status of the trace collector
func GetTraceCollectorStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": traceConfig.IsRunning,
			"config": gin.H{
				"namespace":         traceConfig.Namespace,
				"container_name":    traceConfig.ContainerName,
				"destination_path":  traceConfig.DestinationPath,
				"local_destination": traceConfig.LocalDestination,
				"check_interval":    traceConfig.CheckIntervalSecs,
			},
		})
	}
}

// ConfigureTraceCollector allows updating the trace collector configuration
func ConfigureTraceCollector() gin.HandlerFunc {
	return func(c *gin.Context) {
		var config TraceCollectorConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid configuration parameters",
			})
			return
		}

		// Update the configuration (only non-empty fields)
		if config.Namespace != "" {
			traceConfig.Namespace = config.Namespace
		}
		if config.ContainerName != "" {
			traceConfig.ContainerName = config.ContainerName
		}
		if config.DestinationPath != "" {
			traceConfig.DestinationPath = config.DestinationPath
		}
		if config.LocalDestination != "" {
			traceConfig.LocalDestination = config.LocalDestination
		}
		if config.CheckIntervalSecs > 0 {
			traceConfig.CheckIntervalSecs = config.CheckIntervalSecs
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Configuration updated successfully",
			"config": gin.H{
				"namespace":         traceConfig.Namespace,
				"container_name":    traceConfig.ContainerName,
				"destination_path":  traceConfig.DestinationPath,
				"local_destination": traceConfig.LocalDestination,
				"check_interval":    traceConfig.CheckIntervalSecs,
			},
		})
	}
}

// findUPFPod finds a pod that starts with "open5gs-upf"
func findUPFPod(clientset *kubernetes.Clientset) (string, error) {
	// List pods in the namespace
	pods, err := clientset.CoreV1().Pods(traceConfig.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %v", err)
	}

	// Find a pod with name starting with "open5gs-upf"
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, "open5gs-upf") {
			return pod.Name, nil
		}
	}

	return "", fmt.Errorf("no pod starting with 'open5gs-upf' found in namespace %s", traceConfig.Namespace)
}

// collectTraces continuously collects trace files from the pod
func collectTraces(clientset *kubernetes.Clientset, stopChan <-chan struct{}) {
	// Use a ticker for periodic execution
	ticker := time.NewTicker(time.Duration(traceConfig.CheckIntervalSecs) * time.Second)
	defer ticker.Stop()

	// Console log helper
	consoleLog := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		fmt.Print(message)
		os.Stdout.Sync() // Force flush
	}

	for {
		select {
		case <-stopChan:
			consoleLog("[TRACE] Trace collector stopped\n")
			return
		case <-ticker.C:
			// Find the UPF pod
			podName, err := findUPFPod(clientset)
			if err != nil {
				consoleLog("[TRACE-ERROR] Failed to find UPF pod: %v\n", err)
				continue
			}

			consoleLog("[TRACE] Using pod: %s\n", podName)

			// Step 1: List trace files in the pod
			consoleLog("[TRACE] Listing trace files in pod %s (namespace: %s)...\n",
				podName, traceConfig.Namespace)

			cmd := exec.Command(
				"kubectl", "exec", "-n", traceConfig.Namespace, podName,
				"-c", traceConfig.ContainerName, "--", "ls", "-1", traceConfig.DestinationPath,
			)

			output, err := cmd.CombinedOutput()
			if err != nil {
				consoleLog("[TRACE-ERROR] Failed to list files in pod: %v\nOutput: %s\n", err, output)
				continue
			}

			// Parse the output to get trace files
			traceFiles := strings.Split(strings.TrimSpace(string(output)), "\n")

			// Filter for .pcap files and remove empty entries
			var pcapFiles []string
			for _, file := range traceFiles {
				file = strings.TrimSpace(file)
				if file != "" && strings.HasSuffix(file, ".pcap") {
					pcapFiles = append(pcapFiles, file)
				}
			}

			// Remove the last file from the list (if files exist)
			if len(pcapFiles) > 0 {
				pcapFiles = pcapFiles[:len(pcapFiles)-1]
				consoleLog("[TRACE] Trace files found in pod: %v\n", pcapFiles)
			} else {
				consoleLog("[TRACE] No trace files found in pod.\n")
				continue
			}

			// Step 2: Copy each trace file that doesn't exist locally
			filesCopied := 0
			for _, traceFile := range pcapFiles {
				localPath := filepath.Join(traceConfig.LocalDestination, traceFile)

				// Check if file already exists locally
				if _, err := os.Stat(localPath); err == nil {
					consoleLog("[TRACE] File %s already exists locally, skipping...\n", traceFile)
					continue
				}

				// Copy the file
				remotePath := fmt.Sprintf("%s:%s/%s", podName, traceConfig.DestinationPath, traceFile)
				consoleLog("[TRACE] Copying %s to %s...\n", remotePath, localPath)

				copyCmd := exec.Command(
					"kubectl", "cp", "-n", traceConfig.Namespace,
					"-c", traceConfig.ContainerName,
					remotePath, localPath,
				)

				if copyOutput, err := copyCmd.CombinedOutput(); err != nil {
					consoleLog("[TRACE-ERROR] Error copying %s: %v\nOutput: %s\n",
						traceFile, err, copyOutput)
				} else {
					consoleLog("[TRACE] Successfully copied %s\n", traceFile)
					filesCopied++
				}
			}

			if filesCopied > 0 {
				consoleLog("[TRACE] [%s] Copied %d new trace files.\n",
					time.Now().Format("2006-01-02 15:04:05"), filesCopied)
			} else {
				consoleLog("[TRACE] [%s] No new trace files to copy.\n",
					time.Now().Format("2006-01-02 15:04:05"))
			}
		}
	}
}
