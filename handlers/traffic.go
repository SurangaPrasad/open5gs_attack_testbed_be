package handlers

import (
	// "context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// corev1 "k8s.io/api/core/v1"
)

type TrafficTestRequest struct {
	PodName string `json:"podName" binding:"required"`
}

// getPodIP gets the IP address of the uesimtun0 interface in the pod
func getPodIP(clientset *kubernetes.Clientset, podName string) (string, error) {
	// Execute command to get IP address
	cmd := exec.Command("kubectl", "exec", podName, "--", "ip", "addr", "show", "uesimtun0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get IP address: %v", err)
	}

	// Parse the output to get the IP address
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "inet ") {
			// Extract IP address from the line
			// Example: inet 10.45.0.9/32 scope global uesimtun0
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, "/") {
					return strings.Split(part, "/")[0], nil
				}
			}
		}
	}
	return "", fmt.Errorf("could not find IP address for uesimtun0")
}

// RunBinningTrafficTest handles the traffic test execution
func RunBinningTrafficTest(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TrafficTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
			return
		}

		// Direct console output with forced flush
		consoleLog := func(format string, args ...interface{}) {
			message := fmt.Sprintf(format, args...)
			fmt.Print(message)
			os.Stdout.Sync() // Force flush
		}

		// Step 1: Install required tools
		consoleLog("[TRAFFIC] Installing required tools in pod: %s\n", req.PodName)
		installCmd := exec.Command("kubectl", "exec", req.PodName, "--", "apt-get", "update")
		if output, err := installCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error updating apt: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to update apt",
				"details": string(output),
			})
			return
		}

		installCmd = exec.Command("kubectl", "exec", req.PodName, "--", "apt", "install", "-y", "iperf3", "python3")
		if output, err := installCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error installing tools: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to install required tools",
				"details": string(output),
			})
			return
		}

		// Step 2: Get pod IP address
		consoleLog("[TRAFFIC] Getting pod IP address...\n")
		podIP, err := getPodIP(clientset, req.PodName)
		if err != nil {
			consoleLog("[ERROR] Error getting pod IP: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to get pod IP address",
				"details": err.Error(),
			})
			return
		}
		consoleLog("[TRAFFIC] Pod IP: %s\n", podIP)

		// Step 3: Add route
		consoleLog("[TRAFFIC] Checking existing routes in pod...\n")
		// First, check if the route already exists
		checkRouteCmd := exec.Command("kubectl", "exec", req.PodName, "--", "ip", "route", "show")
		output, err := checkRouteCmd.CombinedOutput()
		if err != nil {
			consoleLog("[ERROR] Error checking routes: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to check existing routes",
				"details": string(output),
			})
			return
		}

		// Check if the specific route already exists
		routeExists := strings.Contains(string(output), "10.42.0.99")

		if routeExists {
			consoleLog("[TRAFFIC] Route already exists. Skipping route addition.\n")
		} else {
			// If route does not exist, add it
			consoleLog("[TRAFFIC] Route not found, proceeding to add route...\n")
			routeCmd := exec.Command("kubectl", "exec", req.PodName, "--", "ip", "route", "add", "10.42.0.99", "via", podIP)
			output, err = routeCmd.CombinedOutput()

			// Check if the error is because the route already exists (RTNETLINK answers: File exists)
			if err != nil && strings.Contains(string(output), "File exists") {
				consoleLog("[TRAFFIC] Route already exists (detected from error message). Continuing...\n")
			} else if err != nil {
				// Handle other errors
				consoleLog("[ERROR] Error adding route: %v\nOutput: %s\n", err, output)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Failed to add route",
					"details": string(output),
				})
				return
			} else {
				consoleLog("[TRAFFIC] Route added successfully.\n")
			}
		}

		// Step 4: Copy and run Python script
		// First, copy the script to the pod
		consoleLog("[TRAFFIC] Copying Python script to pod...\n")
		copyCmd := exec.Command("kubectl", "cp", "/home/open5gs1/Documents/5g_attack_dataset/utills/binning_traffic.py", fmt.Sprintf("%s:/binning_traffic.py", req.PodName))
		if output, err := copyCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error copying script: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to copy Python script",
				"details": string(output),
			})
			return
		}

		// Run the Python script
		consoleLog("[TRAFFIC] Starting Python script...\n")
		runCmd := exec.Command("kubectl", "exec", req.PodName, "--", "python3", "/binning_traffic.py")
		consoleLog("[TRAFFIC] Running command: %s\n", runCmd.String())
		output, err = runCmd.CombinedOutput()
		if err != nil {
			consoleLog("[ERROR] Error running script: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to run traffic test",
				"details": string(output),
			})
			return
		}

		consoleLog("[SUCCESS] Traffic test started successfully!\n")
		c.JSON(http.StatusOK, gin.H{
			"message": "Traffic test started successfully",
			"output":  string(output),
		})
	}
}

// StopBinningTrafficTest handles stopping the running traffic test
func StopBinningTrafficTest(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TrafficTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
			return
		}

		// Direct console output with forced flush
		consoleLog := func(format string, args ...interface{}) {
			message := fmt.Sprintf(format, args...)
			fmt.Print(message)
			os.Stdout.Sync() // Force flush
		}

		consoleLog("[TRAFFIC] Stopping traffic test for pod: %s\n", req.PodName)

		// Find and kill the Python process running binning_traffic.py
		// First, find the process ID
		consoleLog("[TRAFFIC] Finding process ID...\n")
		findCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "python3.*binning_traffic.py")
		pid, err := findCmd.CombinedOutput()
		if err != nil {
			consoleLog("[ERROR] Error finding process: %v\nOutput: %s\n", err, pid)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to find running traffic test process",
				"details": string(pid),
			})
			return
		}

		// Kill the process
		consoleLog("[TRAFFIC] Killing process with PID: %s\n", strings.TrimSpace(string(pid)))
		killCmd := exec.Command("kubectl", "exec", req.PodName, "--", "kill", "-9", strings.TrimSpace(string(pid)))
		output, err := killCmd.CombinedOutput()
		if err != nil {
			consoleLog("[ERROR] Error killing process: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to stop traffic test process",
				"details": string(output),
			})
			return
		}

		consoleLog("[SUCCESS] Traffic test stopped successfully!\n")
		c.JSON(http.StatusOK, gin.H{
			"message": "Traffic test stopped successfully",
			"output":  string(output),
		})
	}
}

// CheckBinningTrafficTestStatus checks if the binning traffic test is running
func CheckBinningTrafficTestStatus(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TrafficTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
			return
		}

		// Direct console output with forced flush
		consoleLog := func(format string, args ...interface{}) {
			message := fmt.Sprintf(format, args...)
			fmt.Print(message)
			os.Stdout.Sync() // Force flush
		}

		consoleLog("[STATUS] Checking traffic test status for pod: %s\n", req.PodName)

		// Check if the Python process is running
		checkCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "python3.*binning_traffic.py")
		if output, err := checkCmd.CombinedOutput(); err != nil {
			if strings.Contains(string(output), "No such process") {
				consoleLog("[STATUS] Traffic test is not running.\n")
				c.JSON(http.StatusOK, gin.H{
					"status": "not running",
				})
			} else {
				consoleLog("[ERROR] Error checking process status: %v\nOutput: %s\n", err, output)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Failed to check traffic test status",
					"details": string(output),
				})
			}
			return
		}

		consoleLog("[STATUS] Traffic test is running.\n")
		c.JSON(http.StatusOK, gin.H{
			"status": "running",
		})
	}
}
