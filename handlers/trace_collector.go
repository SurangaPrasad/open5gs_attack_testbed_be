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
	baseName := filepath.Base(inputFile)
	baseNameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	consoleLog("[TRACE] Generating flow sessions for %s using CICFlowMeter...\n", inputFile)

	// Get absolute paths
	absInputFile, err := filepath.Abs(inputFile)
	if err != nil {
		consoleLog("[TRACE-ERROR] Failed to get absolute path for input: %v\n", err)
		return
	}

	// Use absolute path for flow output directory
	absFlowOutputDir := "/home/open5gs1/Documents/open5gs-be/flow_output"

	// Create the flow output directory if it does not exist
	if err := os.MkdirAll(absFlowOutputDir, 0755); err != nil {
		consoleLog("[TRACE-ERROR] Failed to create flow output directory: %v\n", err)
		return
	}

	// Check if already processed
	outputFilename := baseNameWithoutExt + "_Flow.csv"
	targetOutputFile := filepath.Join(absFlowOutputDir, outputFilename)

	if _, err := os.Stat(targetOutputFile); err == nil {
		consoleLog("[TRACE] Flow file %s already exists in output directory, skipping...\n", outputFilename)
		return
	}

	// Use our wrapper script with absolute paths
	wrapperScript := filepath.Join(traceConfig.CICFlowMeterPath, "run_cicflow.sh")
	if _, err := os.Stat(wrapperScript); err != nil {
		consoleLog("[TRACE-ERROR] Wrapper script not found: %v\n", err)
		return
	}

	consoleLog("[TRACE] Running CICFlowMeter wrapper script...\n")
	cmd := exec.Command(wrapperScript, absInputFile, absFlowOutputDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		consoleLog("[TRACE-ERROR] Error running CICFlowMeter: %v\nOutput: %s\n", err, output)
	} else {
		consoleLog("[TRACE] CICFlowMeter execution successful\n%s\n", string(output))
	}

	// Verify files were created in our flow output directory
	files, err := filepath.Glob(filepath.Join(absFlowOutputDir, "*.csv"))
	if err != nil {
		consoleLog("[TRACE-ERROR] Error checking output directory: %v\n", err)
	} else if len(files) > 0 {
		consoleLog("[TRACE] Found %d CSV files in output directory\n", len(files))
	} else {
		consoleLog("[TRACE-WARNING] No CSV files found in output directory, creating default\n")

		// Create a simple CSV with headers as fallback
		defaultOutputFile := filepath.Join(absFlowOutputDir, baseNameWithoutExt+"_Flow.csv")
		headers := "timestamp,src_ip,dst_ip,src_port,dst_port,protocol,flow_duration,flow_byts_s,flow_pkts_s\n"
		if err := os.WriteFile(defaultOutputFile, []byte(headers), 0644); err != nil {
			consoleLog("[TRACE-ERROR] Failed to create default flow file: %v\n", err)
		} else {
			consoleLog("[TRACE] Created default flow file: %s\n", defaultOutputFile)
		}
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
		consoleLog("[TRACE-WARNING] Failed to set permissions on processed directory: %v\n")
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
		consoleLog("[TRACE-ERROR] Failed to setup CICFlowMeter: %v\n", err)
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

// setupCICFlowMeter ensures CICFlowMeter is properly installed and ready to use
func setupCICFlowMeter(consoleLog func(format string, args ...interface{})) error {
	// Check if CICFlowMeter directory exists
	if _, err := os.Stat(traceConfig.CICFlowMeterPath); os.IsNotExist(err) {
		consoleLog("[TRACE-ERROR] CICFlowMeter directory not found at %s\n", traceConfig.CICFlowMeterPath)
		return err
	}

	// Make stripe utility executable
	consoleLog("[TRACE] Verifying stripe utility at %s\n", traceConfig.StripeUtilityPath)
	if _, err := os.Stat(traceConfig.StripeUtilityPath); os.IsNotExist(err) {
		consoleLog("[TRACE-ERROR] Stripe utility not found at %s\n", traceConfig.StripeUtilityPath)
		return fmt.Errorf("stripe utility not found at %s", traceConfig.StripeUtilityPath)
	} else {
		// Make stripe executable
		if err := os.Chmod(traceConfig.StripeUtilityPath, 0755); err != nil {
			consoleLog("[TRACE-WARNING] Failed to make stripe executable: %v\n", err)
			return fmt.Errorf("failed to make stripe executable: %v", err)
		} else {
			consoleLog("[TRACE] Stripe utility is executable.\n")
		}
	}

	// Change to CICFlowMeter directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}

	if err := os.Chdir(traceConfig.CICFlowMeterPath); err != nil {
		return fmt.Errorf("failed to change to CICFlowMeter directory: %v", err)
	}

	// Make gradlew executable if it's not already
	gradlewPath := filepath.Join(traceConfig.CICFlowMeterPath, "gradlew")
	if err := os.Chmod(gradlewPath, 0755); err != nil {
		consoleLog("[TRACE-WARNING] Failed to make gradlew executable: %v\n", err)
	} else {
		consoleLog("[TRACE] gradlew is now executable.\n")
	}

	consoleLog("[TRACE] Setting up CICFlowMeter...\n")

	// Check if the output_flows directory exists, if not create it
	outputFlowsDir := filepath.Join(traceConfig.CICFlowMeterPath, "output_flows")
	if _, err := os.Stat(outputFlowsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(outputFlowsDir, 0755); err != nil {
			consoleLog("[TRACE-WARNING] Failed to create output_flows directory: %v\n", err)
		} else {
			consoleLog("[TRACE] Created output_flows directory.\n")
		}
	}

	// Check if jnetpcap is installed
	consoleLog("[TRACE] Checking jnetpcap installation...\n")

	// Detect OS type
	var jnetpcapPath string
	if _, err := os.Stat("/etc/os-release"); err == nil {
		// Linux
		jnetpcapPath = filepath.Join(traceConfig.CICFlowMeterPath, "jnetpcap", "linux", "jnetpcap-1.4.r1425")
	} else {
		// Assume Windows
		jnetpcapPath = filepath.Join(traceConfig.CICFlowMeterPath, "jnetpcap", "win", "jnetpcap-1.4.r1425")
	}

	// Check if jar file exists
	jarPath := filepath.Join(jnetpcapPath, "jnetpcap.jar")
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		consoleLog("[TRACE-ERROR] jnetpcap.jar not found at %s\n", jarPath)
		os.Chdir(currentDir)
		return fmt.Errorf("jnetpcap.jar not found at %s", jarPath)
	} else {
		consoleLog("[TRACE] Found jnetpcap.jar at %s\n", jarPath)
	}

	// Install jnetpcap to local Maven repo
	consoleLog("[TRACE] Installing jnetpcap to local Maven repository...\n")
	cmd := exec.Command("mvn", "install:install-file",
		"-Dfile="+jarPath,
		"-DgroupId=org.jnetpcap",
		"-DartifactId=jnetpcap",
		"-Dversion=1.4.1",
		"-Dpackaging=jar")

	if output, err := cmd.CombinedOutput(); err != nil {
		consoleLog("[TRACE-ERROR] Failed to install jnetpcap: %v\nOutput: %s\n", err, output)
		// Continue anyway, as it might be already installed
	} else {
		consoleLog("[TRACE] jnetpcap installed successfully.\n")
	}

	// Modify the build.gradle file to fix Java version issues
	buildGradlePath := filepath.Join(traceConfig.CICFlowMeterPath, "build.gradle")
	if _, err := os.Stat(buildGradlePath); err == nil {
		consoleLog("[TRACE] Modifying build.gradle to fix Java version issues...\n")

		// Read the current build.gradle file
		buildGradleContent, err := os.ReadFile(buildGradlePath)
		if err != nil {
			consoleLog("[TRACE-ERROR] Failed to read build.gradle: %v\n", err)
		} else {
			// Add Java compatibility settings
			modifiedContent := string(buildGradleContent)
			if !strings.Contains(modifiedContent, "sourceCompatibility") {
				insertPoint := strings.Index(modifiedContent, "apply plugin: 'java'")
				if insertPoint >= 0 {
					javaSettings := `
apply plugin: 'java'
sourceCompatibility = '1.8'
targetCompatibility = '1.8'
`
					modifiedContent = strings.Replace(modifiedContent, "apply plugin: 'java'", javaSettings, 1)

					if err := os.WriteFile(buildGradlePath, []byte(modifiedContent), 0644); err != nil {
						consoleLog("[TRACE-ERROR] Failed to update build.gradle: %v\n", err)
					} else {
						consoleLog("[TRACE] Successfully updated build.gradle with Java version settings.\n")
					}
				}
			}
		}
	}

	// Create a simple wrapper script to run CICFlowMeter
	wrapperScript := filepath.Join(traceConfig.CICFlowMeterPath, "run_flowmeter.sh")
	wrapperContent := `#!/bin/bash
cd "$(dirname "$0")"
java -Djava.library.path=./jnetpcap/linux/jnetpcap-1.4.r1425 -jar build/libs/CICFlowMeter.jar "$@"
`
	if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
		consoleLog("[TRACE-ERROR] Failed to create wrapper script: %v\n", err)
	} else {
		consoleLog("[TRACE] Created CICFlowMeter wrapper script.\n")
	}

	// Try building the project
	consoleLog("[TRACE] Building CICFlowMeter project with gradle...\n")
	buildCmd := exec.Command("./gradlew", "build", "-x", "test")

	if output, err := buildCmd.CombinedOutput(); err != nil {
		consoleLog("[TRACE-ERROR] Failed to build CICFlowMeter: %v\nOutput: %s\n", err, output)
		// Continue anyway, as it might already be built
	} else {
		consoleLog("[TRACE] CICFlowMeter built successfully.\n")
	}

	// Check for compiled jar file
	jarFilePath := filepath.Join(traceConfig.CICFlowMeterPath, "build", "libs", "CICFlowMeter.jar")
	if _, err := os.Stat(jarFilePath); os.IsNotExist(err) {
		consoleLog("[TRACE-WARNING] CICFlowMeter jar file not found at %s\n", jarFilePath)
	} else {
		consoleLog("[TRACE] Found CICFlowMeter jar file at %s\n", jarFilePath)
	}

	// Check if run_cicflow.sh script exists and is executable
	cicFlowRunScript := filepath.Join(traceConfig.CICFlowMeterPath, "run_cicflow.sh")
	if _, err := os.Stat(cicFlowRunScript); os.IsNotExist(err) {
		consoleLog("[TRACE-ERROR] CICFlowMeter run script not found at %s\n", cicFlowRunScript)
	} else {
		// Make script executable
		if err := os.Chmod(cicFlowRunScript, 0755); err != nil {
			consoleLog("[TRACE-WARNING] Failed to make CICFlowMeter run script executable: %v\n", err)
		} else {
			consoleLog("[TRACE] CICFlowMeter run script is executable.\n")
		}
	}

	// Change back to original directory
	if err := os.Chdir(currentDir); err != nil {
		return fmt.Errorf("failed to change back to original directory: %v", err)
	}

	consoleLog("[TRACE] CICFlowMeter setup completed.\n")
	return nil
}
