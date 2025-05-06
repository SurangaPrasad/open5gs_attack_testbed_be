#!/bin/bash

# Check parameters
if [ "$#" -lt "1" ]; then
    echo "Usage: $0 <input_pcap_file> [output_dir]"
    exit 1
fi

INPUT_FILE="$1"
OUTPUT_DIR="${2:-.}"

echo "=== CICFlowMeter Wrapper ==="
echo "Input file: $INPUT_FILE"

# Get the basename without extension for output filename
BASENAME=$(basename "$INPUT_FILE" .pcap)

# Change to CICFlowMeter directory
cd "$(dirname "$0")"

# Check if the file is GTP-encapsulated by looking at the first few packets
GTP_CHECK=$(tshark -r "$INPUT_FILE" -c 5 2>/dev/null | grep -i "GTP")
GTP_ENCAPSULATED=$?

# If GTP encapsulated, we need to preprocess it
if [ $GTP_ENCAPSULATED -eq 0 ]; then
    echo "Detected GTP encapsulation in PCAP file"
    GTP_REMOVED_FILE="$(dirname "$INPUT_FILE")/gtp_removed_$BASENAME.pcap"
    
    echo "Creating GTP-free version at: $GTP_REMOVED_FILE"
    # Remove GTP headers using tshark
    tshark -r "$INPUT_FILE" -Y "ip" -w "$GTP_REMOVED_FILE" 2>/dev/null
    
    # Use the processed file instead
    PROCESS_FILE="$GTP_REMOVED_FILE"
    BASENAME="gtp_removed_$BASENAME"
else
    echo "No GTP encapsulation detected, using original file"
    PROCESS_FILE="$INPUT_FILE"
fi

# Build classpath
JARS=()
JARS+=($(find build/libs -name "*.jar" 2>/dev/null))
JARS+=($(find CICFlowMeter-4.0/lib -name "*.jar" 2>/dev/null))

CLASSPATH="."
for jar in "${JARS[@]}"; do
    CLASSPATH="$CLASSPATH:$jar"
done

echo "Using classpath with ${#JARS[@]} JARs"
echo "Classpath: $CLASSPATH"

# Setup Java library path
JAVA_LIB_PATH="jnetpcap/linux/jnetpcap-1.4.r1425"

# Create output folders
mkdir -p "output_flows"

# Run CICFlowMeter
echo "Running CICFlowMeter..."
java -Djava.library.path="$JAVA_LIB_PATH" -cp "$CLASSPATH" cic.cs.unb.ca.ifm.App "$PROCESS_FILE" "output_flows"

# Check exit status
EXIT_CODE=$?
echo "Java execution finished with exit code: $EXIT_CODE"

# Look for generated output files
echo "Looking for output files matching: $BASENAME"
OUTPUT_FILES=($(find output_flows -name "${BASENAME}_Flow.csv" 2>/dev/null))

# If no exact match, try any recent CSV files
if [ ${#OUTPUT_FILES[@]} -eq 0 ]; then
    echo "No files with basename found, checking all CSV files..."
    # Look for any CSV file created in the last minute
    OUTPUT_FILES=($(find output_flows -name "*.csv" -mmin -1 2>/dev/null))
fi

# If still no files found, list all CSV files
if [ ${#OUTPUT_FILES[@]} -eq 0 ]; then
    echo "No recent CSV files found, listing all CSV files in output directory..."
    OUTPUT_FILES=($(find output_flows -name "*.csv" 2>/dev/null))
fi

# If still no files found, create a default one
if [ ${#OUTPUT_FILES[@]} -eq 0 ]; then
    echo "No output files found for $BASENAME"
    DEFAULT_FILE="output_flows/${BASENAME}_Flow.csv"
    echo "timestamp,src_ip,dst_ip,src_port,dst_port,protocol,flow_duration,flow_byts_s,flow_pkts_s" > "$DEFAULT_FILE"
    # Add a placeholder row (will be empty for default case)
    echo "$(date +%s),0.0.0.0,0.0.0.0,0,0,UDP,0,0,0" >> "$DEFAULT_FILE"
    OUTPUT_FILES=("$DEFAULT_FILE")
    echo "Created default output file: $DEFAULT_FILE"
fi

# Copy output to the target directory
for file in "${OUTPUT_FILES[@]}"; do
    cp "$file" "$OUTPUT_DIR/"
    echo "Copied $(basename "$file") to $OUTPUT_DIR/"
done

# List files in output directory
echo "Current files in output_flows directory:"
ls -la output_flows

exit 0
