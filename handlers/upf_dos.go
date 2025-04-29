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

// UPFDosAttackRequest represents the request payload for UPF DoS attack operations
type UPFDosAttackRequest struct {
	PodName  string `json:"podName" binding:"required"`
	TargetIP string `json:"targetIP" binding:"required"`
}

// RunUPFDosAttack handles executing an Intra-UPF UE DoS Attack from the pod
func RunUPFDosAttack(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UPFDosAttackRequest
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

		consoleLog("[UPF-DOS] Starting Intra-UPF UE DoS Attack setup for pod: %s\n", req.PodName)

		// Step 1: Install required tools
		consoleLog("[UPF-DOS] Installing required tools in pod: %s\n", req.PodName)
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
		consoleLog("[UPF-DOS] Creating directory for attack script...\n")
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
		consoleLog("[UPF-DOS] Copying attack script to pod...\n")
		copyCmd := exec.Command("kubectl", "cp", 
			"/home/open5gs1/Documents/5g_attack_dataset/utills/Intra-UPF UE DoS Attack/amplified_traffic_attack.py", 
			fmt.Sprintf("%s:/attack_scripts/upf_dos_attack.py", req.PodName))
		if output, err := copyCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error copying script: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to copy attack script",
				"details": string(output),
			})
			return
		}

		// Step 4: Update the target IP in the script
		consoleLog("[UPF-DOS] Setting target IP to %s in the script...\n", req.TargetIP)
		updateTargetCmd := exec.Command("kubectl", "exec", req.PodName, "--", "sed", "-i", 
			fmt.Sprintf("s/TARGET_IP = \".*\"/TARGET_IP = \"%s\"/", req.TargetIP), 
			"/attack_scripts/upf_dos_attack.py")
		if output, err := updateTargetCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error updating target IP: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to update target IP in script",
				"details": string(output),
			})
			return
		}

		// Step 5: Create a launch script that will properly daemonize the process
		consoleLog("[UPF-DOS] Creating launcher script...\n")
		launcherCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c",
			"cat > /attack_scripts/upf_dos_launcher.sh << 'EOF'\n#!/bin/bash\npython3 /attack_scripts/upf_dos_attack.py > /dev/null 2>&1 &\necho $!\nEOF\n")
		if output, err := launcherCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error creating launcher script: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create launcher script",
				"details": string(output),
			})
			return
		}

		// Make launcher script executable
		chmodCmd := exec.Command("kubectl", "exec", req.PodName, "--", "chmod", "+x", "/attack_scripts/upf_dos_launcher.sh")
		if output, err := chmodCmd.CombinedOutput(); err != nil {
			consoleLog("[ERROR] Error setting script permissions: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to set launcher script permissions",
				"details": string(output),
			})
			return
		}

		// Step 6: Launch the attack script using the launcher script
		consoleLog("[UPF-DOS] Starting Intra-UPF UE DoS Attack...\n")
		runCmd := exec.Command("kubectl", "exec", req.PodName, "--", "/attack_scripts/upf_dos_launcher.sh")
		output, err := runCmd.CombinedOutput()
		if err != nil {
			consoleLog("[ERROR] Error running attack: %v\nOutput: %s\n", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to run Intra-UPF UE DoS Attack",
				"details": string(output),
			})
			return
		}

		// Save the process ID to a file for easier management
		pid := strings.TrimSpace(string(output))
		if pid != "" {
			pidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c", 
				fmt.Sprintf("echo '%s' > /attack_scripts/upf_dos.pid", pid))
			pidCmd.CombinedOutput() // We don't need to check for errors here
		}

		consoleLog("[SUCCESS] Intra-UPF UE DoS Attack started successfully with PID: %s!\n", pid)
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Intra-UPF UE DoS Attack started successfully against %s", req.TargetIP),
			"pid":     pid,
		})
	}
}

// StopUPFDosAttack handles stopping the running Intra-UPF UE DoS Attack
func StopUPFDosAttack(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UPFDosAttackRequest
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

		consoleLog("[UPF-DOS] Stopping Intra-UPF UE DoS Attack for pod: %s\n", req.PodName)

		// Check if we have a saved PID file
		checkPidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c", 
			"if [ -f /attack_scripts/upf_dos.pid ]; then cat /attack_scripts/upf_dos.pid; else echo ''; fi")
		pidBytes, err := checkPidCmd.CombinedOutput()
		
		if err == nil && len(pidBytes) > 0 {
			// If we have a saved PID, use it to directly kill the process
			pid := strings.TrimSpace(string(pidBytes))
			if pid != "" {
				consoleLog("[UPF-DOS] Found saved PID: %s. Killing process...\n", pid)
				killCmd := exec.Command("kubectl", "exec", req.PodName, "--", "kill", "-9", pid)
				killCmd.CombinedOutput()
			}
		}

		// Find and kill any Python processes running the attack script
		consoleLog("[UPF-DOS] Finding other attack processes...\n")
		findCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "python3.*upf_dos_attack.py")
		pids, err := findCmd.CombinedOutput()
		if err == nil || !strings.Contains(string(pids), "No such process") {
			// Kill all found processes
			for _, pid := range strings.Split(strings.TrimSpace(string(pids)), "\n") {
				if pid == "" {
					continue
				}
				consoleLog("[UPF-DOS] Killing process with PID: %s\n", pid)
				killCmd := exec.Command("kubectl", "exec", req.PodName, "--", "kill", "-9", pid)
				killCmd.CombinedOutput()
			}
		}

		consoleLog("[SUCCESS] Intra-UPF UE DoS Attack stopped successfully!\n")
		c.JSON(http.StatusOK, gin.H{
			"message": "Intra-UPF UE DoS Attack stopped successfully",
		})
	}
}

// CheckUPFDosAttackStatus checks if the Intra-UPF UE DoS Attack is running
func CheckUPFDosAttackStatus(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UPFDosAttackRequest
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

		consoleLog("[STATUS] Checking Intra-UPF UE DoS Attack status for pod: %s\n", req.PodName)

		// First check if we have a saved PID file
		checkPidCmd := exec.Command("kubectl", "exec", req.PodName, "--", "bash", "-c", 
			"if [ -f /attack_scripts/upf_dos.pid ]; then cat /attack_scripts/upf_dos.pid; else echo ''; fi")
		pidBytes, err := checkPidCmd.CombinedOutput()
		
		if err == nil && len(pidBytes) > 0 {
			pid := strings.TrimSpace(string(pidBytes))
			if pid != "" {
				// Check if the process with this PID is still running
				checkRunningCmd := exec.Command("kubectl", "exec", req.PodName, "--", "ps", "-p", pid)
				if _, err := checkRunningCmd.CombinedOutput(); err == nil {
					consoleLog("[STATUS] Intra-UPF UE DoS Attack is running with PID: %s\n", pid)
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
		checkCmd := exec.Command("kubectl", "exec", req.PodName, "--", "pgrep", "-f", "python3.*upf_dos_attack.py")
		if output, err := checkCmd.CombinedOutput(); err != nil {
			if strings.Contains(string(output), "No such process") {
				consoleLog("[STATUS] No Intra-UPF UE DoS Attack is currently running.\n")
				c.JSON(http.StatusOK, gin.H{
					"status": "not running",
				})
			} else {
				consoleLog("[ERROR] Error checking process status: %v\nOutput: %s\n", err, output)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Failed to check UPF DoS attack status",
					"details": string(output),
				})
			}
			return
		}

		// If we get here, the process is running but we don't have a PID file
		consoleLog("[STATUS] Intra-UPF UE DoS Attack is running.\n")
		c.JSON(http.StatusOK, gin.H{
			"status": "running",
		})
	}
}