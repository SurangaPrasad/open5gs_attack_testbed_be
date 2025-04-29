package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"
)

// TEIDAttackRequest represents the request payload for TEID Brute-Force attack operations
type TEIDAttackRequest struct {
	PodName  string `json:"podName" binding:"required"`
	TargetIP string `json:"targetIP"`
}

// RunTEIDBruteForceAttack handles executing a GTP-U TEID Brute-Force attack from the pod
func RunTEIDBruteForceAttack(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TEIDAttackRequest
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

		consoleLog("[TEID] Starting GTP-U TEID Brute-Force attack setup for pod: %s\n", req.PodName)

		// Step 1: Install required tools
		consoleLog("[TEID] Installing required tools in pod: %s\n", req.PodName)
		installCmd := exec.Command("kubectl", "exec", req.PodName, "--", "apt-get", "update")
		if output, err := installCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error updating apt: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to update apt",
				"details": string(output),
			})
			return
		}

		// Install Python and dependencies
		installCmd = exec.Command("kubectl", "exec", req.PodName, "--", "apt", "install", "-y", 
			"python3", "python3-pip")
		if output, err := installCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error installing tools: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to install required tools",
				"details": string(output),
			})
			return
		}

		// Install required Python packages
		pipInstallCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pip3", "install", "scapy")
		if output, err := pipInstallCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error installing Python packages: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to install required Python packages",
				"details": string(output),
			})
			return
		}

		// Step 2: Create directory for attack script
		consoleLog("[TEID] Creating directory for attack script...\n")
		mkdirCmd := exec.Command("kubectl", "exec", req.PodName, "--", "mkdir", "-p", "/attack_scripts")
		if output, err := mkdirCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error creating directory: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create directory",
				"details": string(output),
			})
			return
		}

		// Step 3: Copy attack script to pod
		consoleLog("[TEID] Copying attack script to pod...\n")
		copyCmd := exec.Command("kubectl", "cp", 
			"/home/open5gs1/Documents/5g_attack_dataset/utills/GTP-U TEID Brute-Force Attack/different_gtp_type.py", 
			fmt.Sprintf("%s:/attack_scripts/teid_bruteforce.py", req.PodName))
		if output, err := copyCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error copying script: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to copy attack script",
				"details": string(output),
			})
			return
		}

		// Step 4: Update the target IP in the script if specified
		if req.TargetIP != "" {
			consoleLog("[TEID] Setting target IP to %s in the script...\n", req.TargetIP)
			updateTargetCmd := exec.Command("kubectl", "exec", req.PodName, "--", "sed", "-i", 
				fmt.Sprintf("s/dst=\"10\\.42\\.0\\.64\"/dst=\"%s\"/", req.TargetIP), 
				"/attack_scripts/teid_bruteforce.py")
			if output, err := updateTargetCmd.CombinedOutput(); err != nil {
				consoleLog("[ERROR] Error updating target IP: %v\nOutput: %s\n", err, output)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Failed to update target IP in script",
					"details": string(output),
				})
				return
			}
		}

		// Step 5: Create a launch script that will properly daemonize the process
		consoleLog("[TEID] Creating launcher script...\n")
		launcherCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c",
			"cat > /attack_scripts/teid_launcher.sh << 'EOF'\n#!/bin/bash\npython3 /attack_scripts/teid_bruteforce.py > /dev/null 2>&1 &\necho $!\nEOF\n")
		if output, err := launcherCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error creating launcher script: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create launcher script",
				"details": string(output),
			})
			return
		}

		// Make launcher script executable
		chmodCmd := exec.Command("kubectl", "exec", req.PodName, "--", "chmod", "+x", "/attack_scripts/teid_launcher.sh")
		if output, err := chmodCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error setting script permissions: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to set launcher script permissions",
				"details": string(output),
			})
			return
		}

		// Step 6: Launch the attack script using the launcher script
		consoleLog("[TEID] Starting GTP-U TEID Brute-Force attack...\n")
		runCmd := exec.Command("kubectl", "exec", req.PodName, "--", "/attack_scripts/teid_launcher.sh")
		output, err := runCmd.CombinedOutput()
		if err != nil {
			consoleLog("[ERROR] Error running attack: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to run TEID Brute-Force attack",
				"details": string(output),
			})
			return
		}

		// Save the process ID to a file for easier management
		pid := strings.TrimSpace(string(output))
		if pid != "" {
			pidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c", 
				fmt.Sprintf("echo '%s' > /attack_scripts/teid.pid", pid))
			pidCmd.CombinedOutput() // We don't need to check for errors here
		}

		consoleLog("[SUCCESS] GTP-U TEID Brute-Force attack started successfully with PID: %s!\n", pid)
		c.JSON(http.StatusOK, gin.H{
			"message": "GTP-U TEID Brute-Force attack started successfully",
			"pid":     pid,
		})
	}
}

// StopTEIDBruteForceAttack handles stopping the running GTP-U TEID Brute-Force attack
func StopTEIDBruteForceAttack(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TEIDAttackRequest
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

		consoleLog("[TEID] Stopping GTP-U TEID Brute-Force attack for pod: %s\n", req.PodName)

		// Check if we have a saved PID file
		checkPidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c", 
			"if [ -f /attack_scripts/teid.pid ]; then cat /attack_scripts/teid.pid; else echo ''; fi")
		pidBytes, err := checkPidCmd.CombinedOutput()
		
		if err == nil && len(pidBytes) > 0 {
			// If we have a saved PID, use it to directly kill the process
			pid := strings.TrimSpace(string(pidBytes))
			if pid != "" {
				consoleLog("[TEID] Found saved PID: %s. Killing process...\n", pid)
				killCmd := exec.Command("kubectl", "exec", req.PodName, "--", "kill", "-9", pid)
				killCmd.CombinedOutput()
			}
		}

		// Find and kill any Python processes running the attack script
		consoleLog("[TEID] Finding other attack processes...\n")
		findCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "python3.*teid_bruteforce.py")
		pids, err := findCmd.CombinedOutput()
		if err == nil || !strings.Contains(string(pids), "No such process") {
			// Kill all found processes
			for _, pid := range strings.Split(strings.TrimSpace(string(pids)), "\n") {
				if pid == "" {
					continue
				}
				consoleLog("[TEID] Killing process with PID: %s\n", pid)
				killCmd := exec.Command("kubectl", "exec", req.PodName, "--", "kill", "-9", pid)
				killCmd.CombinedOutput()
			}
		}

		consoleLog("[SUCCESS] GTP-U TEID Brute-Force attack stopped successfully!\n")
		c.JSON(http.StatusOK, gin.H{
			"message": "GTP-U TEID Brute-Force attack stopped successfully",
		})
	}
}

// CheckTEIDBruteForceAttackStatus checks if the GTP-U TEID Brute-Force attack is running
func CheckTEIDBruteForceAttackStatus(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TEIDAttackRequest
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

		consoleLog("[STATUS] Checking GTP-U TEID Brute-Force attack status for pod: %s\n", req.PodName)

		// First check if we have a saved PID file
		checkPidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c", 
			"if [ -f /attack_scripts/teid.pid ]; then cat /attack_scripts/teid.pid; else echo ''; fi")
		pidBytes, err := checkPidCmd.CombinedOutput()
		
		if err == nil && len(pidBytes) > 0 {
			pid := strings.TrimSpace(string(pidBytes))
			if pid != "" {
				// Check if the process with this PID is still running
				checkRunningCmd := exec.Command("kubectl", "exec", req.PodName, "--", "ps", "-p", pid)
				if _, err := checkRunningCmd.CombinedOutput(); err == nil {
					consoleLog("[STATUS] GTP-U TEID Brute-Force attack is running with PID: %s\n", pid)
					c.JSON(http.StatusOK, gin.H{
						"status": "running",
						"pid":    pid,
					})
					return
				}
			}
		}

		// If we don't have a PID file or the saved PID doesn't correspond to a running process,
		// check for any running attack processes
		checkCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "python3.*teid_bruteforce.py")
		if output, err := checkCmd.CombinedOutput(); err != nil {
			if strings.Contains(string(output), "No such process") {
				consoleLog("[STATUS] No GTP-U TEID Brute-Force attack is currently running.\n")
				c.JSON(http.StatusOK, gin.H{
					"status": "not running",
				})
			} else {
				consoleLog("[ERROR] Error checking process status: %v\nOutput: %s\n", err, output)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Failed to check TEID Brute-Force attack status",
					"details": string(output),
				})
			}
			return
		}

		// If we get here, the process is running but we don't have a PID file
		consoleLog("[STATUS] GTP-U TEID Brute-Force attack is running.\n")
		c.JSON(http.StatusOK, gin.H{
			"status": "running",
		})
	}
}