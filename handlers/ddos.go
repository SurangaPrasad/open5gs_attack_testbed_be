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

// DDoSRequest represents the request payload for DDoS attack operations
type DDoSRequest struct {
	PodName  string `json:"podName" binding:"required"`
	TargetIP string `json:"targetIP" binding:"required"`
}

// consoleLog is a helper function to write logs with forced flush
func consoleLog(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Print(message)
	os.Stdout.Sync() // Force flush
}

// RunICMPDDoSAttack handles executing an ICMP DDoS attack from the pod
func RunICMPDDoSAttack(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DDoSRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
			return
		}

		consoleLog("[DDOS] Starting ICMP DDoS attack setup for pod: %s\n", req.PodName)

		// Step 1: Install required tools
		consoleLog("[DDOS] Installing required tools in pod: %s\n", req.PodName)
		installCmd := exec.Command("kubectl", "exec", req.PodName, "--", "apt-get", "update")
		if output, err := installCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error updating apt: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to update apt",
				"details": string(output),
			})
			return
		}

		// Install Python and hping3
		installCmd = exec.Command("kubectl", "exec", req.PodName, "--", "apt", "install", "-y", "python3", "hping3")
		if output, err := installCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error installing tools: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to install required tools",
				"details": string(output),
			})
			return
		}

		// Step 2: Create directory for attack script
		consoleLog("[DDOS] Creating directory for attack script...\n")
		mkdirCmd := exec.Command("kubectl", "exec", req.PodName, "--", "mkdir", "-p", "/ddos_attack")
		if output, err := mkdirCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error creating directory: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create directory",
				"details": string(output),
			})
			return
		}

		// Step 3: Copy ICMP attack script to pod
		consoleLog("[DDOS] Copying ICMP attack script to pod...\n")
		copyCmd := exec.Command("kubectl", "cp",
			"/home/open5gs1/Documents/5g_attack_dataset/utills/DDoS Attack/icmp_attack.py",
			fmt.Sprintf("%s:/ddos_attack/icmp_attack.py", req.PodName))
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
			consoleLog("[DDOS] Setting target IP to %s in the script...\n", req.TargetIP)
			updateTargetCmd := exec.Command("kubectl", "exec", req.PodName, "--", "sed", "-i",
				fmt.Sprintf("s/TARGET_IP = \".*\"/TARGET_IP = \"%s\"/", req.TargetIP),
				"/ddos_attack/icmp_attack.py")
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
		consoleLog("[DDOS] Creating launcher script...\n")
		launcherCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c",
			"cat > /ddos_attack/launcher.sh << 'EOF'\n#!/bin/bash\npython3 /ddos_attack/icmp_attack.py > /dev/null 2>&1 &\necho $!\nEOF\n")
		if output, err := launcherCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error creating launcher script: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create launcher script",
				"details": string(output),
			})
			return
		}

		// Make launcher script executable
		chmodCmd := exec.Command("kubectl", "exec", req.PodName, "--", "chmod", "+x", "/ddos_attack/launcher.sh")
		if output, err := chmodCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error setting script permissions: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to set launcher script permissions",
				"details": string(output),
			})
			return
		}

		// Step 6: Launch the attack script using the launcher script
		consoleLog("[DDOS] Starting ICMP attack...\n")
		runCmd := exec.Command("kubectl", "exec", req.PodName, "--", "/ddos_attack/launcher.sh")
		output, err := runCmd.CombinedOutput()
		if err != nil {
			consoleLog("[ERROR] Error running attack: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to run ICMP DDoS attack",
				"details": string(output),
			})
			return
		}

		// Save the process ID to a file for easier management
		pid := strings.TrimSpace(string(output))
		if pid != "" {
			pidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c",
				fmt.Sprintf("echo '%s' > /ddos_attack/attack.pid", pid))
			pidCmd.CombinedOutput() // We don't need to check for errors here
		}

		consoleLog("[SUCCESS] ICMP DDoS attack started successfully with PID: %s!\n", pid)
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("ICMP DDoS attack started successfully against %s", req.TargetIP),
			"pid":     pid,
		})
	}
}

// StopDDoSAttack handles stopping the running DDoS attack
func StopDDoSAttack(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DDoSRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
			return
		}

		consoleLog("[DDOS] Stopping DDoS attack for pod: %s\n", req.PodName)

		// Check if we have a saved PID file
		checkPidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "cat", "/ddos_attack/attack.pid")
		pidBytes, err := checkPidCmd.CombinedOutput()

		if err == nil && len(pidBytes) > 0 {
			// If we have a saved PID, use it to directly kill the process
			pid := strings.TrimSpace(string(pidBytes))
			consoleLog("[DDOS] Found saved PID: %s. Killing process...\n", pid)
			killCmd := exec.Command("kubectl", "exec", req.PodName, "--", "kill", "-9", pid)
			killCmd.CombinedOutput()
		}

		// Find and kill any Python processes running the attack script
		consoleLog("[DDOS] Finding other attack processes...\n")
		findCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "python3.*icmp_attack.py")
		pids, err := findCmd.CombinedOutput()
		if err != nil {
			// If there's no process running, that's fine
			if strings.Contains(string(pids), "No such process") {
				consoleLog("[DDOS] No attack processes found. Attack might have already stopped.\n")
			} else {
				consoleLog("[ERROR] Error finding process: %v\nOutput: %s\n", err, pids)
			}
		} else {
			// Kill all found processes
			for _, pid := range strings.Split(strings.TrimSpace(string(pids)), "\n") {
				if pid == "" {
					continue
				}
				consoleLog("[DDOS] Killing process with PID: %s\n", pid)
				killCmd := exec.Command("kubectl", "exec", req.PodName, "--", "kill", "-9", pid)
				killCmd.CombinedOutput()
			}
		}

		// Also kill any hping3 processes that might be running
		findHpingCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "hping3")
		hpingPids, _ := findHpingCmd.CombinedOutput()
		if len(hpingPids) > 0 {
			for _, pid := range strings.Split(strings.TrimSpace(string(hpingPids)), "\n") {
				if pid == "" {
					continue
				}
				consoleLog("[DDOS] Killing hping3 process with PID: %s\n", pid)
				killCmd := exec.Command("kubectl", "exec", req.PodName, "--", "kill", "-9", pid)
				killCmd.CombinedOutput()
			}
		}

		consoleLog("[SUCCESS] DDoS attack stopped successfully!\n")
		c.JSON(http.StatusOK, gin.H{
			"message": "DDoS attack stopped successfully",
		})
	}
}

// CheckDDoSAttackStatus checks if the DDoS attack is running
func CheckDDoSAttackStatus(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DDoSRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
			return
		}

		consoleLog("[STATUS] Checking DDoS attack status for pod: %s\n", req.PodName)

		// First check if we have a saved PID file
		checkPidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c",
			"if [ -f /ddos_attack/attack.pid ]; then cat /ddos_attack/attack.pid; else echo ''; fi")
		pidBytes, err := checkPidCmd.CombinedOutput()

		if err == nil && len(pidBytes) > 0 {
			pid := strings.TrimSpace(string(pidBytes))
			if pid != "" {
				// Check if the process with this PID is still running
				checkRunningCmd := exec.Command("kubectl", "exec", req.PodName, "--", "ps", "-p", pid)
				if _, err := checkRunningCmd.CombinedOutput(); err == nil {
					consoleLog("[STATUS] DDoS attack is running with PID: %s\n", pid)
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
		checkCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "python3.*icmp_attack.py")
		if output, err := checkCmd.CombinedOutput(); err != nil {
			if strings.Contains(string(output), "No such process") {
				consoleLog("[STATUS] No DDoS attack is currently running.\n")
				c.JSON(http.StatusOK, gin.H{
					"status": "not running",
				})
			} else {
				consoleLog("[ERROR] Error checking process status: %v\nOutput: %s\n", err, output)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Failed to check DDoS attack status",
					"details": string(output),
				})
			}
			return
		}

		// If we get here, the process is running but we don't have a PID file
		consoleLog("[STATUS] DDoS attack is running.\n")
		c.JSON(http.StatusOK, gin.H{
			"status": "running",
		})
	}
}
