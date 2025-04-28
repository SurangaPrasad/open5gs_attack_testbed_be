package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type HelmValues struct {
	AMF struct {
		Hostname string `json:"hostname"`
	} `json:"amf"`
	MCC string `json:"mcc"`
	MNC string `json:"mnc"`
	SST int    `json:"sst"`
	SD  string `json:"sd"`
	TAC string `json:"tac"`
	UEs struct {
		Enabled       bool   `json:"enabled"`
		Count         int    `json:"count"`
		InitialMSISDN string `json:"initialMSISDN"`
	} `json:"ues"`
}

type HelmRequest struct {
	DeploymentName string `json:"deploymentName" binding:"required"`
	InitialMSISDN  string `json:"initialMSISDN" binding:"required"`
}

type UninstallRequest struct {
	DeploymentName string `json:"deploymentName" binding:"required"`
}

// InstallUERANSIM handles the Helm installation with dynamic values
func InstallUERANSIM() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req HelmRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
			return
		}

		// Create values structure
		values := HelmValues{}
		values.AMF.Hostname = "open5gs-amf-ngap"
		values.MCC = "999"
		values.MNC = "70"
		values.SST = 1
		values.SD = "0x111111"
		values.TAC = "0001"
		values.UEs.Enabled = true
		values.UEs.Count = 1
		values.UEs.InitialMSISDN = req.InitialMSISDN

		// Create temporary directory for values file
		tempDir, err := os.MkdirTemp("", "helm-values-*")
		if err != nil {
			log.Printf("Error creating temp directory: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temporary directory"})
			return
		}
		defer os.RemoveAll(tempDir)

		// Create values file
		valuesFile := filepath.Join(tempDir, "values.yaml")
		valuesData, err := json.Marshal(values)
		if err != nil {
			log.Printf("Error marshaling values: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create values file"})
			return
		}

		// Write values to file
		if err := os.WriteFile(valuesFile, valuesData, 0644); err != nil {
			log.Printf("Error writing values file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write values file"})
			return
		}

		// Execute Helm command
		cmd := exec.Command("helm", "install", req.DeploymentName,
			"oci://registry-1.docker.io/gradiant/ueransim-gnb",
			"--version", "0.2.6",
			"--values", valuesFile)

		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Error executing Helm command: %v\nOutput: %s", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to install UERANSIM",
				"details": string(output),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "UERANSIM installed successfully",
			"output": string(output),
		})
	}
}

// UninstallUERANSIM handles the Helm uninstall command
func UninstallUERANSIM() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UninstallRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
			return
		}

		// Execute Helm uninstall command
		cmd := exec.Command("helm", "uninstall", req.DeploymentName)

		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Error executing Helm command: %v\nOutput: %s", err, output)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to uninstall UERANSIM",
				"details": string(output),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "UERANSIM uninstalled successfully",
			"output": string(output),
		})
	}
} 