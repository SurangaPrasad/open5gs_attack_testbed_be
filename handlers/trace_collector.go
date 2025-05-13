package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"context"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Configuration struct for trace collector
type TraceCollectorConfig struct {
	Namespace            string
	ContainerName        string
	DestinationPath      string
	LocalDestination     string
	ProcessedDestination string
	FlowOutputDirectory  string
	CheckIntervalSecs    int
	IsRunning            bool
	StopChan             chan struct{}
	StripeUtilityPath    string
	CICFlowMeterPath     string
}

// Global configuration for trace collector
var traceConfig = TraceCollectorConfig{
	Namespace:            "default",
	ContainerName:        "trace-collector",
	DestinationPath:      "/usr/src/app/pcap_files",
	LocalDestination:     "./pcap_files",
	ProcessedDestination: "./pcap_files_processed",
	FlowOutputDirectory:  "./flow_output",
	CheckIntervalSecs:    10,
	IsRunning:            false,
	StopChan:             make(chan struct{}),
	StripeUtilityPath:    "/home/open5gs1/Documents/open5gs-be/stripe/stripe",
	CICFlowMeterPath:     "/home/open5gs1/Documents/open5gs-be/CICFlowMeter",
}

// Added structures to work with decision tree models
type DecisionTreeNode struct {
	Node      int                `json:"node"`
	Feature   string             `json:"feature"`
	Threshold float64            `json:"threshold"`
	Value     [][]float64        `json:"value"`
	Children  []DecisionTreeNode `json:"children"`
}

// Mapping of class indices to attack types
var ATTACK_CLASSES = map[int]string{
	0: "BENIGN",
	1: "Intra_UPF_UE_DoS",
	2: "DDoS",
	3: "GTP_ENCAPSULATION",
	4: "GTP_ENCAPSULATION",
}

// Cache to track analyzed files and avoid redundant processing
var analyzedFiles = make(map[string]bool)
var analyzedFilesMutex sync.Mutex

// StartTraceCollector initializes and starts the trace collector
func StartTraceCollector(clientset *kubernetes.Clientset) gin.HandlerFunc {
	return func(c *gin.Context) {
		if traceConfig.IsRunning {
			c.JSON(http.StatusOK, gin.H{
				"message": "Trace collector is already running",
			})
			return
		}

		// Ensure local destination directories exist
		if err := os.MkdirAll(traceConfig.LocalDestination, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create local directory: %v", err),
			})
			return
		}

		if err := os.MkdirAll(traceConfig.ProcessedDestination, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create processed directory: %v", err),
			})
			return
		}

		if err := os.MkdirAll(traceConfig.FlowOutputDirectory, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create flow output directory: %v", err),
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
				"namespace":             traceConfig.Namespace,
				"container_name":        traceConfig.ContainerName,
				"destination_path":      traceConfig.DestinationPath,
				"local_destination":     traceConfig.LocalDestination,
				"processed_destination": traceConfig.ProcessedDestination,
				"flow_output_directory": traceConfig.FlowOutputDirectory,
				"check_interval":        traceConfig.CheckIntervalSecs,
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
				"namespace":             traceConfig.Namespace,
				"container_name":        traceConfig.ContainerName,
				"destination_path":      traceConfig.DestinationPath,
				"local_destination":     traceConfig.LocalDestination,
				"processed_destination": traceConfig.ProcessedDestination,
				"flow_output_directory": traceConfig.FlowOutputDirectory,
				"check_interval":        traceConfig.CheckIntervalSecs,
				"stripe_utility_path":   traceConfig.StripeUtilityPath,
				"cicflowmeter_path":     traceConfig.CICFlowMeterPath,
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
		if config.ProcessedDestination != "" {
			traceConfig.ProcessedDestination = config.ProcessedDestination
		}
		if config.FlowOutputDirectory != "" {
			traceConfig.FlowOutputDirectory = config.FlowOutputDirectory
		}
		if config.CheckIntervalSecs > 0 {
			traceConfig.CheckIntervalSecs = config.CheckIntervalSecs
		}
		if config.StripeUtilityPath != "" {
			traceConfig.StripeUtilityPath = config.StripeUtilityPath
		}
		if config.CICFlowMeterPath != "" {
			traceConfig.CICFlowMeterPath = config.CICFlowMeterPath
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Configuration updated successfully",
			"config": gin.H{
				"namespace":             traceConfig.Namespace,
				"container_name":        traceConfig.ContainerName,
				"destination_path":      traceConfig.DestinationPath,
				"local_destination":     traceConfig.LocalDestination,
				"processed_destination": traceConfig.ProcessedDestination,
				"flow_output_directory": traceConfig.FlowOutputDirectory,
				"check_interval":        traceConfig.CheckIntervalSecs,
				"stripe_utility_path":   traceConfig.StripeUtilityPath,
				"cicflowmeter_path":     traceConfig.CICFlowMeterPath,
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

// generateFlowSessions processes a PCAP file with CICFlowMeter to generate network flow sessions
func generateFlowSessions(inputFile string, consoleLog func(format string, args ...interface{})) {
	baseName := filepath.Base(inputFile) // e.g., gtp_removed_capture_20250507_122302.pcap

	consoleLog("[TRACE] Generating flow sessions for %s using pre-built CICFlowMeter...\n", inputFile)

	// Ensure the configured flow output directory exists
	if err := os.MkdirAll(traceConfig.FlowOutputDirectory, 0755); err != nil {
		consoleLog("[TRACE-ERROR] Failed to create flow output directory '%s': %v\n", traceConfig.FlowOutputDirectory, err)
		return
	}

	// CICFlowMeter typically creates output files named like <input_pcap_filename>_Flow.csv
	// For example, if inputFile is "mycapture.pcap", output is "mycapture.pcap_Flow.csv"
	outputFilename := baseName + "_Flow.csv" // e.g., gtp_removed_capture_20250507_122302.pcap_Flow.csv
	targetOutputFile := filepath.Join(traceConfig.FlowOutputDirectory, outputFilename)

	// Check if already processed by looking for the specific output file
	if _, err := os.Stat(targetOutputFile); err == nil {
		consoleLog("[TRACE] Flow file %s already exists in output directory '%s', skipping generation...\n", outputFilename, traceConfig.FlowOutputDirectory)
		analyzeFlowFile(targetOutputFile, consoleLog) // Still analyze if it exists
		return
	}

	// Get absolute paths for the script arguments
	absInputFile, err := filepath.Abs(inputFile)
	if err != nil {
		consoleLog("[TRACE-ERROR] Failed to get absolute path for input file '%s': %v\n", inputFile, err)
		return
	}

	absOutputDir, err := filepath.Abs(traceConfig.FlowOutputDirectory)
	if err != nil {
		consoleLog("[TRACE-ERROR] Failed to get absolute path for output directory '%s': %v\n", traceConfig.FlowOutputDirectory, err)
		return
	}

	// Use the new direct wrapper script
	wrapperScriptPath := filepath.Join(traceConfig.CICFlowMeterPath, "run_cfm_direct.sh")
	if _, err := os.Stat(wrapperScriptPath); os.IsNotExist(err) {
		consoleLog("[TRACE-ERROR] CICFlowMeter direct wrapper script not found at %s. Run setup first.\n", wrapperScriptPath)
		return
	}

	consoleLog("[TRACE] Running CICFlowMeter direct wrapper script '%s' for pcap '%s' outputting to '%s'\n", wrapperScriptPath, absInputFile, absOutputDir)
	cmd := exec.Command(wrapperScriptPath, absInputFile, absOutputDir)
	cmdOutput, cmdErr := cmd.CombinedOutput()

	if cmdErr != nil {
		consoleLog("[TRACE-ERROR] Error running CICFlowMeter direct wrapper script: %v\nOutput:\n%s\n", cmdErr, string(cmdOutput))
		// Even if there's an error, check if the target file was created, as cfm might still produce output
		if _, statErr := os.Stat(targetOutputFile); statErr == nil {
			consoleLog("[TRACE] Target output file '%s' was found despite script error. Attempting to analyze.\n", targetOutputFile)
			analyzeFlowFile(targetOutputFile, consoleLog)
		} else {
			consoleLog("[TRACE] Target output file '%s' not found after script error.\n", targetOutputFile)
		}
		return // Stop further processing for this file if script failed significantly
	}

	consoleLog("[TRACE] CICFlowMeter direct wrapper script execution successful.\nOutput:\n%s\n", string(cmdOutput))

	// Verify the specific target output file was created
	if _, err := os.Stat(targetOutputFile); os.IsNotExist(err) {
		consoleLog("[TRACE-WARNING] Expected output file '%s' not found in '%s' after CICFlowMeter execution.\n", outputFilename, absOutputDir)
		// Attempt to find any CSV in case of naming variations, though less ideal
		files, globErr := filepath.Glob(filepath.Join(absOutputDir, "*.csv"))
		if globErr != nil || len(files) == 0 {
			consoleLog("[TRACE-WARNING] No CSV files found in output directory '%s'.\n", absOutputDir)
			// Create a simple CSV with headers as fallback if no file is found at all
			// This part might be removed if not desired for cfm direct execution
			defaultOutputFile := filepath.Join(absOutputDir, baseName+"_Flow_default.csv") // different name for default
			headers := "Flow ID,Src IP,Src Port,Dst IP,Dst Port,Protocol,Timestamp,Flow Duration,Tot Fwd Pkts,Tot Bwd Pkts,TotLen Fwd Pkts,TotLen Bwd Pkts,Fwd Pkt Len Max,Fwd Pkt Len Min,Fwd Pkt Len Mean,Fwd Pkt Len Std,Bwd Pkt Len Max,Bwd Pkt Len Min,Bwd Pkt Len Mean,Bwd Pkt Len Std,Flow Byts/s,Flow Pkts/s,Flow IAT Mean,Flow IAT Std,Flow IAT Max,Flow IAT Min,Fwd IAT Tot,Fwd IAT Mean,Fwd IAT Std,Fwd IAT Max,Fwd IAT Min,Bwd IAT Tot,Bwd IAT Mean,Bwd IAT Std,Bwd IAT Max,Bwd IAT Min,Fwd PSH Flags,Bwd PSH Flags,Fwd URG Flags,Bwd URG Flags,Fwd Header Len,Bwd Header Len,Fwd Pkts/s,Bwd Pkts/s,Pkt Len Min,Pkt Len Max,Pkt Len Mean,Pkt Len Std,Pkt Len Var,FIN Flag Cnt,SYN Flag Cnt,RST Flag Cnt,PSH Flag Cnt,ACK Flag Cnt,URG Flag Cnt,CWE Flag Count,ECE Flag Cnt,Down/Up Ratio,Pkt Size Avg,Fwd Seg Size Avg,Bwd Seg Size Avg,Fwd Byts/b Avg,Fwd Pkts/b Avg,Fwd Blk Rate Avg,Bwd Byts/b Avg,Bwd Pkts/b Avg,Bwd Blk Rate Avg,Subflow Fwd Pkts,Subflow Fwd Byts,Subflow Bwd Pkts,Subflow Bwd Byts,Init Fwd Win Byts,Init Bwd Win Byts,Fwd Act Data Pkts,Fwd Seg Size Min,Active Mean,Active Std,Active Max,Active Min,Idle Mean,Idle Std,Idle Max,Idle Min\n"
			if writeErr := os.WriteFile(defaultOutputFile, []byte(headers), 0644); writeErr != nil {
				consoleLog("[TRACE-ERROR] Failed to create default flow file: %v\n", writeErr)
			} else {
				consoleLog("[TRACE] Created default (empty) flow file: %s\n", defaultOutputFile)
			}
			return // Don't analyze if the expected file isn't there
		} else {
			consoleLog("[TRACE] Found other CSV files: %v. Attempting to analyze the first one if relevant, but expected '%s'.\n", files, outputFilename)
			// This logic might need refinement if multiple unexpected files are created.
			// For now, we will proceed to analyze the specific targetOutputFile if it was found, otherwise warn.
		}
	}

	// Analyze the generated flow file if it exists
	if _, err := os.Stat(targetOutputFile); err == nil {
		analyzeFlowFile(targetOutputFile, consoleLog)
	} else {
		consoleLog("[TRACE-WARNING] Final check: Expected output file '%s' still not found. Cannot analyze.\n", targetOutputFile)
	}
}

// processTraceFile processes a trace file to remove GTP headers using the stripe utility
func processTraceFile(inputFile string, consoleLog func(format string, args ...interface{})) {
	// Convert relative paths to absolute paths
	absInputFile, err := filepath.Abs(inputFile)
	if err != nil {
		consoleLog("[TRACE-ERROR] Failed to get absolute path for input file: %v\n", err)
		return
	}

	// Generate output file name by prepending "gtp_removed_" to the base filename
	baseName := filepath.Base(absInputFile)

	// Convert relative ProcessedDestination to absolute path
	absProcessedDir, err := filepath.Abs(traceConfig.ProcessedDestination)
	if err != nil {
		consoleLog("[TRACE-ERROR] Failed to get absolute path for processed directory: %v\n", err)
		return
	}

	// Build the absolute output file path
	outputFile := filepath.Join(absProcessedDir, "gtp_removed_"+baseName)

	// Check if output file already exists
	if _, err := os.Stat(outputFile); err == nil {
		consoleLog("[TRACE] Processed file %s already exists, skipping processing...\n", outputFile)

		// Even if the file exists, we still want to generate flow sessions
		generateFlowSessions(outputFile, consoleLog)
		return
	}

	// Ensure the processed destination directory exists with correct permissions
	if err := os.MkdirAll(absProcessedDir, 0755); err != nil {
		consoleLog("[TRACE-ERROR] Failed to create or verify processed directory: %v\n", err)
		return
	}

	// Set directory permissions to ensure we can write to it
	if err := os.Chmod(absProcessedDir, 0755); err != nil {
		consoleLog("[TRACE-WARNING] Failed to set permissions on processed directory: %v\n", err)
	}

	// Pre-create an empty output file with correct permissions
	emptyFile, err := os.Create(outputFile)
	if err != nil {
		consoleLog("[TRACE-ERROR] Failed to pre-create output file %s: %v\n", outputFile, err)
		return
	}
	emptyFile.Close()

	if err := os.Chmod(outputFile, 0644); err != nil {
		consoleLog("[TRACE-WARNING] Failed to set permissions on output file: %v\n", err)
	}

	consoleLog("[TRACE] Processing file %s to remove GTP headers...\n", absInputFile)

	// Verify that the stripe utility exists
	if _, err := os.Stat(traceConfig.StripeUtilityPath); os.IsNotExist(err) {
		consoleLog("[TRACE-ERROR] Stripe utility not found at %s\n", traceConfig.StripeUtilityPath)
		return
	}

	// Build and execute the stripe command with absolute paths
	cmd := exec.Command(traceConfig.StripeUtilityPath,
		"-r", absInputFile,
		"-w", outputFile)

	// Set command working directory to ensure relative paths work
	cmd.Dir = filepath.Dir(traceConfig.StripeUtilityPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		consoleLog("[TRACE-ERROR] Error processing file %s: %v\nOutput: %s\n",
			absInputFile, err, output)

		// Check if output file was created despite error
		if _, statErr := os.Stat(outputFile); statErr == nil {
			fileInfo, _ := os.Stat(outputFile)
			if fileInfo.Size() > 0 {
				consoleLog("[TRACE] Output file was created with size %d bytes. Proceeding despite error.\n", fileInfo.Size())
				generateFlowSessions(outputFile, consoleLog)
				return
			}
		}

		return
	}

	consoleLog("[TRACE] Successfully processed %s -> %s\n", absInputFile, outputFile)

	// Generate flow sessions from the processed file
	generateFlowSessions(outputFile, consoleLog)
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

	// Track files that have been processed
	processedFiles := make(map[string]bool)

	// Install CICFlowMeter dependencies if needed
	if err := setupCICFlowMeter(consoleLog); err != nil {
		consoleLog("[TRACE-ERROR] Failed to setup CICFlowMeter: %v\n")
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

					// Process the file if it hasn't been processed yet
					if _, processed := processedFiles[traceFile]; !processed {
						go processTraceFile(localPath, consoleLog)
						processedFiles[traceFile] = true
					}

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

					// Process the newly copied file to remove GTP headers
					go processTraceFile(localPath, consoleLog)
					processedFiles[traceFile] = true
				}
			}

			if filesCopied > 0 {
				consoleLog("[TRACE] [%s] Copied %d new trace files.\n",
					time.Now().Format("2006-01-02 15:04:05"), filesCopied)
			} else {
				consoleLog("[TRACE] [%s] No new trace files to copy.\n",
					time.Now().Format("2006-01-02 15:04:05"))
			}

			// Process any existing files that haven't been processed yet
			existingFiles, err := filepath.Glob(filepath.Join(traceConfig.LocalDestination, "*.pcap"))
			if err == nil {
				for _, file := range existingFiles {
					baseName := filepath.Base(file)
					if _, processed := processedFiles[baseName]; !processed {
						go processTraceFile(file, consoleLog)
						processedFiles[baseName] = true
					}
				}
			}
		}
	}
}

// setupCICFlowMeter ensures the environment is ready to run the pre-built CICFlowMeter.
func setupCICFlowMeter(consoleLog func(format string, args ...interface{})) error {
	consoleLog("[TRACE] Verifying pre-built CICFlowMeter setup...\n")

	// Make stripe utility executable
	consoleLog("[TRACE] Verifying stripe utility at %s\n", traceConfig.StripeUtilityPath)
	if _, err := os.Stat(traceConfig.StripeUtilityPath); os.IsNotExist(err) {
		consoleLog("[TRACE-ERROR] Stripe utility not found at %s\n", traceConfig.StripeUtilityPath)
		return fmt.Errorf("stripe utility not found at %s", traceConfig.StripeUtilityPath)
	}
	if err := os.Chmod(traceConfig.StripeUtilityPath, 0755); err != nil {
		consoleLog("[TRACE-WARNING] Failed to make stripe executable (may proceed if already executable): %v\n", err)
	} else {
		consoleLog("[TRACE] Stripe utility is executable.\n")
	}

	// Define the path to the pre-built cfm executable
	// This path is fixed based on the user's successful manual execution.
	cfmExecutableDir := "/home/open5gs1/Documents/open5gs-be/CICFlowMeter_extracted/CICFlowMeter-4.0/bin/"
	cfmExecutableName := "cfm"
	cfmExecutablePath := filepath.Join(cfmExecutableDir, cfmExecutableName)

	// Check if cfm executable exists
	if _, err := os.Stat(cfmExecutablePath); os.IsNotExist(err) {
		consoleLog("[TRACE-ERROR] Pre-built CICFlowMeter executable not found at %s.\n", cfmExecutablePath)
		return fmt.Errorf("pre-built CICFlowMeter executable not found at %s", cfmExecutablePath)
	}
	consoleLog("[TRACE] Found CICFlowMeter executable at %s\n", cfmExecutablePath)

	// Ensure cfm executable is actually executable
	if err := os.Chmod(cfmExecutablePath, 0755); err != nil {
		consoleLog("[TRACE-WARNING] Failed to make CICFlowMeter executable at %s (may proceed if already executable): %v\n", cfmExecutablePath, err)
	} else {
		consoleLog("[TRACE] CICFlowMeter executable at %s is now executable.\n")
	}

	// Path for the new wrapper script that generateFlowSessions will call
	// This script will be placed in traceConfig.CICFlowMeterPath (e.g., /home/open5gs1/Documents/open5gs-be/CICFlowMeter/run_cfm_direct.sh)
	wrapperScriptPath := filepath.Join(traceConfig.CICFlowMeterPath, "run_cfm_direct.sh")

	// Content for the run_cfm_direct.sh wrapper script
	wrapperContent := fmt.Sprintf(`#!/bin/bash
# This script runs the pre-built CICFlowMeter directly.

CFM_DIR="%s"
CFM_EXEC_NAME="%s"
PCAP_FILE="$1"      # Absolute path to the input PCAP file
OUTPUT_DIR="$2"     # Absolute path to the output directory

# Log received arguments
echo "run_cfm_direct.sh: Received PCAP_FILE: ${PCAP_FILE}"
echo "run_cfm_direct.sh: Received OUTPUT_DIR: ${OUTPUT_DIR}"

if [ -z "${PCAP_FILE}" ]; then
    echo "run_cfm_direct.sh: Error - PCAP_FILE argument is missing."
    exit 1
fi

if [ -z "${OUTPUT_DIR}" ]; then
    echo "run_cfm_direct.sh: Error - OUTPUT_DIR argument is missing."
    exit 1
fi

if [ ! -f "${CFM_DIR}/${CFM_EXEC_NAME}" ]; then
    echo "run_cfm_direct.sh: Error - CICFlowMeter executable not found at ${CFM_DIR}/${CFM_EXEC_NAME}"
    exit 1
fi

echo "run_cfm_direct.sh: Changing directory to ${CFM_DIR}"
cd "${CFM_DIR}" || { echo "run_cfm_direct.sh: Failed to cd to ${CFM_DIR}"; exit 1; }

echo "run_cfm_direct.sh: Executing: ./${CFM_EXEC_NAME} \"${PCAP_FILE}\" \"${OUTPUT_DIR}\""
./${CFM_EXEC_NAME} "${PCAP_FILE}" "${OUTPUT_DIR}"
EXIT_CODE=$?
echo "run_cfm_direct.sh: CICFlowMeter finished with exit code ${EXIT_CODE}"

exit ${EXIT_CODE}
`, cfmExecutableDir, cfmExecutableName)

	if err := os.WriteFile(wrapperScriptPath, []byte(wrapperContent), 0755); err != nil {
		consoleLog("[TRACE-ERROR] Failed to create CICFlowMeter direct wrapper script at %s: %v\n", wrapperScriptPath, err)
		return fmt.Errorf("failed to create CICFlowMeter direct wrapper script: %v", err)
	}
	consoleLog("[TRACE] Successfully created/updated CICFlowMeter direct wrapper script at %s\n", wrapperScriptPath)

	consoleLog("[TRACE] CICFlowMeter direct execution setup completed.\n")
	return nil
}

// analyzeFlowFile checks if a CSV flow file contains network anomalies or attacks
func analyzeFlowFile(filePath string, consoleLog func(format string, args ...interface{})) {
	// Check if this file has already been analyzed
	analyzedFilesMutex.Lock()
	if _, exists := analyzedFiles[filePath]; exists {
		consoleLog("[TRACE-ANALYZE] File %s was already analyzed, skipping.\n", filePath)
		analyzedFilesMutex.Unlock()
		return
	}
	analyzedFiles[filePath] = true
	analyzedFilesMutex.Unlock()

	consoleLog("[TRACE-ANALYZE] Analyzing flow file: %s\n", filePath)

	// Load decision tree model
	tree, err := loadDecisionTree()
	if err != nil {
		consoleLog("[TRACE-ANALYZE-ERROR] Failed to load decision tree model: %v\n", err)
		return
	}

	// Read CSV file
	data, err := os.ReadFile(filePath)
	if err != nil {
		consoleLog("[TRACE-ANALYZE-ERROR] Failed to read CSV file: %v\n", err)
		return
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		consoleLog("[TRACE-ANALYZE-WARNING] CSV file has no data rows\n")
		return
	}

	// Parse header to get column indices
	header := strings.Split(lines[0], ",")
	// Declare featureIndices as a local variable
	featureIndices := make(map[string]int)
	for i, column := range header {
		featureIndices[column] = i
	}

	// Statistics for attack detection
	totalFlows := 0
	attackFlows := 0
	attackTypes := make(map[string]int)

	// Process each flow (row in the CSV)
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		totalFlows++

		// Parse the flow record
		flowRecord := strings.Split(line, ",")

		// Get prediction for this flow
		classIndex, attackType := predictFlow(tree, flowRecord, featureIndices)

		// If not normal traffic, count as attack
		if classIndex != 0 {
			attackFlows++
			attackTypes[attackType]++
		}
	}

	// Calculate attack percentage
	attackPercentage := 0.0
	if totalFlows > 0 {
		attackPercentage = float64(attackFlows) / float64(totalFlows) * 100.0
	}

	// Log the results
	consoleLog("[TRACE-ANALYZE] Analysis results for %s:\n", filepath.Base(filePath))
	consoleLog("[TRACE-ANALYZE] Total flows: %d\n", totalFlows)
	consoleLog("[TRACE-ANALYZE] Attack flows: %d (%.2f%%)\n", attackFlows, attackPercentage)

	// Log each attack type found
	for attackType, count := range attackTypes {
		consoleLog("[TRACE-ANALYZE] %s attacks: %d (%.2f%%)\n",
			attackType, count, float64(count)/float64(totalFlows)*100.0)
	}

	// Write to attack detection log
	logPath := filepath.Join(filepath.Dir(traceConfig.FlowOutputDirectory), "logs", "attack_detection.log")
	os.MkdirAll(filepath.Dir(logPath), 0755)

	attackTypesStr := ""
	for attackType, count := range attackTypes {
		if len(attackTypesStr) > 0 {
			attackTypesStr += ", "
		}
		attackTypesStr += fmt.Sprintf("%s:%d", attackType, count)
	}

	// Convert timestamp to date-time string
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("%s,%s,%d,%d,%.2f,%s\n",
		timestamp, filepath.Base(filePath), totalFlows, attackFlows, attackPercentage, attackTypesStr)

	// Append to log file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		if _, err := f.WriteString(logLine); err != nil {
			consoleLog("[TRACE-ANALYZE-ERROR] Failed to write to log file: %v\n", err)
		}
	} else {
		consoleLog("[TRACE-ANALYZE-ERROR] Failed to open log file: %v\n", err)
	}

	// Alert if attack percentage is high
	if attackPercentage > 30.0 {
		consoleLog("[TRACE-ANALYZE-ALERT] HIGH ATTACK PERCENTAGE (%.2f%%) DETECTED in %s\n",
			attackPercentage, filepath.Base(filePath))
	}
}

// predictFlow applies the decision tree to classify a flow record
func predictFlow(tree DecisionTreeNode, flowRecord []string, featureIndices map[string]int) (int, string) {
	node := tree

	for len(node.Children) > 0 {
		feature := node.Feature
		threshold := node.Threshold

		// Handle leaf nodes
		if feature == "leaf" {
			break
		}

		// Get feature index
		featureIndex, exists := featureIndices[feature]
		if !exists {
			// Feature not found, go left
			node = node.Children[0]
			continue
		}

		// Get the feature value from the flow record
		var value float64 = 0
		if featureIndex < len(flowRecord) {
			value, _ = strconv.ParseFloat(flowRecord[featureIndex], 64)
		}

		// Determine which branch to take
		if value <= threshold {
			node = node.Children[0]
		} else {
			node = node.Children[1]
		}
	}

	// Get the predicted class from the leaf node
	probabilities := node.Value[0]
	maxProbability := 0.0
	predictedClass := 0

	for i, probability := range probabilities {
		if probability > maxProbability {
			maxProbability = probability
			predictedClass = i
		}
	}

	return predictedClass, ATTACK_CLASSES[predictedClass]
}

// loadDecisionTree reads and parses the decision tree model from JSON file
func loadDecisionTree() (DecisionTreeNode, error) {
	var tree DecisionTreeNode

	// Construct path to decision tree JSON file
	treePath := filepath.Join(filepath.Dir(traceConfig.FlowOutputDirectory), "utils", "decision_tree.json")

	// Read the file
	data, err := ioutil.ReadFile(treePath)
	if err != nil {
		return tree, fmt.Errorf("failed to read decision tree file: %v", err)
	}

	// Parse JSON
	err = json.Unmarshal(data, &tree)
	if err != nil {
		return tree, fmt.Errorf("failed to parse decision tree JSON: %v", err)
	}

	return tree, nil
}

// Add a function to monitor the flow output directory for new files
func monitorFlowOutputDirectory(consoleLog func(format string, args ...interface{})) {
	// Create a ticker to periodically check the directory
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	processedFiles := make(map[string]bool)

	for {
		select {
		case <-ticker.C:
			// Find all CSV files in the flow output directory
			files, err := filepath.Glob(filepath.Join(traceConfig.FlowOutputDirectory, "*.csv"))
			if err != nil {
				consoleLog("[TRACE-MONITOR-ERROR] Error scanning flow directory: %v\n", err)
				continue
			}

			// Process any new files
			for _, file := range files {
				baseFileName := filepath.Base(file)
				if _, exists := processedFiles[baseFileName]; !exists {
					// Mark as processed and analyze
					processedFiles[baseFileName] = true
					consoleLog("[TRACE-MONITOR] Found new flow file: %s\n", baseFileName)

					// Run analysis in a goroutine to avoid blocking
					go analyzeFlowFile(file, consoleLog)
				}
			}
		}
	}
}

// Start the flow output directory monitor when the trace collector starts
func init() {
	// This function will be called when the package is initialized
	go func() {
		// Wait a bit for configuration to be properly set
		time.Sleep(5 * time.Second)

		// Create console log function for the monitor
		consoleLog := func(format string, args ...interface{}) {
			message := fmt.Sprintf(format, args...)
			fmt.Print(message)
			os.Stdout.Sync() // Force flush
		}

		// Ensure log directory exists
		os.MkdirAll(filepath.Join(filepath.Dir(traceConfig.FlowOutputDirectory), "logs"), 0755)

		// Start monitoring
		monitorFlowOutputDirectory(consoleLog)
	}()
}
