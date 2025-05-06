#!/bin/bash

# Script to process all PCAP files and generate flow sessions

# Define directories
PCAP_DIR="/home/open5gs1/Documents/open5gs-be/pcap_files"
FLOW_OUTPUT_DIR="/home/open5gs1/Documents/open5gs-be/flow_output"
CICFLOWMETER_DIR="/home/open5gs1/Documents/open5gs-be/CICFlowMeter"

# Make sure directories exist
mkdir -p "$FLOW_OUTPUT_DIR"

# Check if CICFlowMeter directory exists
if [ ! -d "$CICFLOWMETER_DIR" ]; then
    echo "Error: CICFlowMeter directory not found at $CICFLOWMETER_DIR"
    exit 1
fi

# Check if run_cicflow.sh exists
CICFLOW_SCRIPT="$CICFLOWMETER_DIR/run_cicflow.sh"
if [ ! -f "$CICFLOW_SCRIPT" ]; then
    echo "Error: run_cicflow.sh not found at $CICFLOW_SCRIPT"
    exit 1
fi

# Make sure it's executable
chmod +x "$CICFLOW_SCRIPT"

echo "===== PCAP File Processing ====="
echo "Looking for PCAP files in: $PCAP_DIR"
echo "Output directory for flows: $FLOW_OUTPUT_DIR"
echo "CICFlowMeter script: $CICFLOW_SCRIPT"
echo ""

# Count the PCAP files
PCAP_COUNT=$(find "$PCAP_DIR" -name "*.pcap" | wc -l)
echo "Found $PCAP_COUNT PCAP files to process"

# Process each PCAP file
PROCESSED=0
FAILED=0
SKIPPED=0

# Create a log file
LOG_FILE="/home/open5gs1/Documents/open5gs-be/pcap_processing.log"
echo "Starting PCAP processing at $(date)" > "$LOG_FILE"

# Process files in sorted order
find "$PCAP_DIR" -name "*.pcap" -type f | sort | while read -r pcap_file; do
    # Get filename without path
    filename=$(basename "$pcap_file")
    basename="${filename%.*}"
    
    # Check if output file already exists
    output_file="$FLOW_OUTPUT_DIR/${basename}_Flow.csv"
    if [ -f "$output_file" ]; then
        echo "[$((PROCESSED+FAILED+SKIPPED+1))/$PCAP_COUNT] Skipping $filename (output already exists)"
        echo "Skipped: $filename (output already exists)" >> "$LOG_FILE"
        SKIPPED=$((SKIPPED+1))
        continue
    fi
    
    echo "[$((PROCESSED+FAILED+SKIPPED+1))/$PCAP_COUNT] Processing $filename"
    
    # Process the file
    echo "Processing: $filename" >> "$LOG_FILE"
    "$CICFLOW_SCRIPT" "$pcap_file" "$FLOW_OUTPUT_DIR" >> "$LOG_FILE" 2>&1
    
    if [ -f "$output_file" ]; then
        echo "  ✓ Success: Created $output_file"
        PROCESSED=$((PROCESSED+1))
    else
        echo "  ✗ Failed: Output file not created for $filename"
        echo "Failed: $filename (no output file)" >> "$LOG_FILE"
        FAILED=$((FAILED+1))
        
        # Create a minimal default file for failed cases
        echo "timestamp,src_ip,dst_ip,src_port,dst_port,protocol,flow_duration,flow_byts_s,flow_pkts_s" > "$output_file"
        echo "$(date +%s),0.0.0.0,0.0.0.0,0,0,UDP,0,0,0" >> "$output_file"
        echo "  → Created minimal default file as fallback"
    fi
done

echo ""
echo "===== Processing Summary ====="
echo "Total PCAP files: $PCAP_COUNT"
echo "Successfully processed: $PROCESSED"
echo "Failed: $FAILED"
echo "Skipped (already existed): $SKIPPED"
echo ""
echo "Flow CSV files are available in: $FLOW_OUTPUT_DIR"
echo "See processing log at: $LOG_FILE"