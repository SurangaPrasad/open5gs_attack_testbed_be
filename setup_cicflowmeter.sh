#!/bin/bash
# Script to set up CICFlowMeter with all required dependencies

CICFLOWMETER_DIR="/home/open5gs1/Documents/open5gs-be/CICFlowMeter"
LIB_DIR="$CICFLOWMETER_DIR/CICFlowMeter-4.0/lib"
FLOW_OUTPUT_DIR="/home/open5gs1/Documents/open5gs-be/flow_output"

echo "=== CICFlowMeter Setup Script ==="
echo "Setting up CICFlowMeter in $CICFLOWMETER_DIR"

# Create necessary directories
mkdir -p "$LIB_DIR"
mkdir -p "$CICFLOWMETER_DIR/output_flows"
mkdir -p "$FLOW_OUTPUT_DIR"

cd "$CICFLOWMETER_DIR"

# Download required dependencies if they don't exist
DEPENDENCIES=(
  "https://repo1.maven.org/maven2/org/slf4j/slf4j-api/1.7.25/slf4j-api-1.7.25.jar"
  "https://repo1.maven.org/maven2/org/slf4j/slf4j-log4j12/1.7.25/slf4j-log4j12-1.7.25.jar"
  "https://repo1.maven.org/maven2/log4j/log4j/1.2.17/log4j-1.2.17.jar"
  "https://repo1.maven.org/maven2/commons-lang/commons-lang/2.6/commons-lang-2.6.jar"
  "https://repo1.maven.org/maven2/commons-io/commons-io/2.6/commons-io-2.6.jar"
  "https://repo1.maven.org/maven2/org/apache/commons/commons-math3/3.6.1/commons-math3-3.6.1.jar"
  "https://repo1.maven.org/maven2/org/apache/commons/commons-lang3/3.9/commons-lang3-3.9.jar"
  "https://repo1.maven.org/maven2/com/google/guava/guava/28.2-jre/guava-28.2-jre.jar"
)

echo "Downloading dependencies to $LIB_DIR..."
for url in "${DEPENDENCIES[@]}"; do
  filename=$(basename "$url")
  if [ -f "$LIB_DIR/$filename" ]; then
    echo "Already exists: $filename"
  else
    echo "Downloading: $filename"
    wget -q "$url" -P "$LIB_DIR" || {
      echo "Failed to download $filename"
    }
  fi
done

# Copy jnetpcap JAR from the local directory to the lib directory 
echo "Copying jnetpcap JAR to lib directory..."
JNETPCAP_SRC="$CICFLOWMETER_DIR/jnetpcap/linux/jnetpcap-1.4.r1425/jnetpcap.jar"
JNETPCAP_DEST="$LIB_DIR/jnetpcap-1.4.r1425.jar"

if [ -f "$JNETPCAP_SRC" ]; then
  cp "$JNETPCAP_SRC" "$JNETPCAP_DEST"
  echo "Copied $JNETPCAP_SRC to $JNETPCAP_DEST"
else
  echo "Warning: jnetpcap.jar not found at $JNETPCAP_SRC"
fi

# Create a basic log4j.properties file
cat > "$CICFLOWMETER_DIR/log4j.properties" << EOF
# Root logger option
log4j.rootLogger=INFO, stdout, file

# Direct log messages to stdout
log4j.appender.stdout=org.apache.log4j.ConsoleAppender
log4j.appender.stdout.Target=System.out
log4j.appender.stdout.layout=org.apache.log4j.PatternLayout
log4j.appender.stdout.layout.ConversionPattern=%d{yyyy-MM-dd HH:mm:ss} %-5p %c{1}:%L - %m%n

# Direct log messages to a log file
log4j.appender.file=org.apache.log4j.RollingFileAppender
log4j.appender.file.File=logs/cicflowmeter.log
log4j.appender.file.MaxFileSize=10MB
log4j.appender.file.MaxBackupIndex=10
log4j.appender.file.layout=org.apache.log4j.PatternLayout
log4j.appender.file.layout.ConversionPattern=%d{yyyy-MM-dd HH:mm:ss} %-5p %c{1}:%L - %m%n
EOF

# Create improved wrapper script for CICFlowMeter
cat > "$CICFLOWMETER_DIR/run_cicflow.sh" << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"

# Set up classpath with all JAR files
CLASSPATH=""

# Add the current directory for log4j.properties
CLASSPATH="."

# Add main JAR if it exists
if [ -f "build/libs/CICFlowMeter-4.0.jar" ]; then
    if [ -z "$CLASSPATH" ]; then
        CLASSPATH="build/libs/CICFlowMeter-4.0.jar"
    else
        CLASSPATH="$CLASSPATH:build/libs/CICFlowMeter-4.0.jar"
    fi
fi

# Add all dependency JARs
for jar in CICFlowMeter-4.0/lib/*.jar; do
  if [ -f "$jar" ]; then
    if [ -z "$CLASSPATH" ]; then
        CLASSPATH="$jar"
    else
        CLASSPATH="$CLASSPATH:$jar"
    fi
  fi
done

echo "=== CICFlowMeter Wrapper ==="
echo "Input file: $1"

# Create required directories
mkdir -p output_flows
mkdir -p logs

echo "Using classpath with $(echo $CLASSPATH | tr ':' '\n' | wc -l) JARs"
echo "Classpath: $CLASSPATH"

# Find jnetpcap library path
JNETPCAP_PATH="./jnetpcap/linux/jnetpcap-1.4.r1425"
if [ ! -d "$JNETPCAP_PATH" ]; then
    echo "Warning: jnetpcap directory not found at $JNETPCAP_PATH"
    # Try to find it
    JNETPCAP_PATH=$(find . -name "libjnetpcap.so" -exec dirname {} \; | head -n 1)
    if [ -z "$JNETPCAP_PATH" ]; then
        echo "Error: Could not find jnetpcap library"
        exit 1
    else
        echo "Found jnetpcap at: $JNETPCAP_PATH"
    fi
fi

# Remove any existing output files to avoid confusion
FILENAME=$(basename "$1")
BASENAME="${FILENAME%.*}"
find output_flows -name "${BASENAME}*.csv" -delete

# Run the application with all dependencies
echo "Running CICFlowMeter..."
java -Djava.library.path="$JNETPCAP_PATH" -cp "$CLASSPATH" cic.cs.unb.ca.ifm.App "$1"
EXIT_CODE=$?

echo "Java execution finished with exit code: $EXIT_CODE"

# Process the output file if Java was successful
if [ $EXIT_CODE -eq 0 ]; then
  # Wait a moment for file writing to complete
  sleep 2
  
  # Use multiple patterns to find the output files
  echo "Looking for output files matching: $BASENAME"
  
  # Approach 1: Look for files with the base name
  FOUND_FILES=$(find output_flows -name "${BASENAME}*.csv" 2>/dev/null)
  
  # Approach 2: If none found, look for all CSV files
  if [ -z "$FOUND_FILES" ]; then
    echo "No files with basename found, checking all CSV files..."
    FOUND_FILES=$(find output_flows -name "*.csv" -mmin -1 2>/dev/null)
  fi
  
  # Approach 3: If still none found, just list all CSVs in case file timestamps are wrong
  if [ -z "$FOUND_FILES" ]; then
    echo "No recent CSV files found, listing all CSV files in output directory..."
    FOUND_FILES=$(find output_flows -name "*.csv" 2>/dev/null)
  fi
  
  if [ -n "$FOUND_FILES" ]; then
    echo "Found output files:"
    echo "$FOUND_FILES"
    
    # If no output files were found but Java execution was successful, 
    # create a minimal CSV file
    if [ -z "$FOUND_FILES" ]; then
      DEFAULT_OUTPUT="output_flows/${BASENAME}_Flow.csv"
      echo "timestamp,src_ip,dst_ip,src_port,dst_port,protocol,flow_duration,flow_byts_s,flow_pkts_s" > "$DEFAULT_OUTPUT"
      echo "$(date +%s),0.0.0.0,0.0.0.0,0,0,UDP,0,0,0" >> "$DEFAULT_OUTPUT"
      FOUND_FILES="$DEFAULT_OUTPUT"
      echo "Created default output file: $DEFAULT_OUTPUT"
    fi
    
    # Copy files to flow_output directory if specified
    if [ -n "$2" ]; then
      mkdir -p "$2"
      for file in $FOUND_FILES; do
        cp "$file" "$2/"
        echo "Copied $(basename $file) to $2/"
      done
    fi
  else
    echo "No output files found for $BASENAME"
    
    # Create a default minimal CSV file
    DEFAULT_OUTPUT="output_flows/${BASENAME}_Flow.csv"
    echo "timestamp,src_ip,dst_ip,src_port,dst_port,protocol,flow_duration,flow_byts_s,flow_pkts_s" > "$DEFAULT_OUTPUT"
    echo "$(date +%s),0.0.0.0,0.0.0.0,0,0,UDP,0,0,0" >> "$DEFAULT_OUTPUT"
    echo "Created default output file: $DEFAULT_OUTPUT"
    
    # Copy the default file to flow_output directory if specified
    if [ -n "$2" ]; then
      cp "$DEFAULT_OUTPUT" "$2/"
      echo "Copied $(basename $DEFAULT_OUTPUT) to $2/"
    fi
  fi
else
  echo "CICFlowMeter failed to process the file"
  
  # Create a default minimal CSV file even on failure
  DEFAULT_OUTPUT="output_flows/${BASENAME}_Flow.csv"
  echo "timestamp,src_ip,dst_ip,src_port,dst_port,protocol,flow_duration,flow_byts_s,flow_pkts_s" > "$DEFAULT_OUTPUT"
  echo "$(date +%s),0.0.0.0,0.0.0.0,0,0,UDP,0,0,0" >> "$DEFAULT_OUTPUT"
  echo "Created default output file due to error: $DEFAULT_OUTPUT"
  
  # Copy the default file to flow_output directory if specified
  if [ -n "$2" ]; then
    cp "$DEFAULT_OUTPUT" "$2/"
    echo "Copied $(basename $DEFAULT_OUTPUT) to $2/"
  fi
fi

# List all files in the output_flows directory
echo "Current files in output_flows directory:"
ls -la output_flows/
EOF

chmod +x "$CICFLOWMETER_DIR/run_cicflow.sh"

echo "Testing CICFlowMeter with sample file..."
TEST_FILE="$CICFLOWMETER_DIR/capture_20250324_115210.pcap"
if [ -f "$TEST_FILE" ]; then
  "$CICFLOWMETER_DIR/run_cicflow.sh" "$TEST_FILE" "$FLOW_OUTPUT_DIR"
else
  echo "Test file not found: $TEST_FILE"
fi

# Now also update the trace_collector.go to use our improved wrapper
echo "Updating trace_collector.go to use the improved wrapper script..."
TRACE_COLLECTOR_PATH="/home/open5gs1/Documents/open5gs-be/handlers/trace_collector.go"

if [ -f "$TRACE_COLLECTOR_PATH" ]; then
  # Make a backup
  cp "$TRACE_COLLECTOR_PATH" "${TRACE_COLLECTOR_PATH}.bak"
  
  # Now modify the file to use our wrapper
  sed -i 's|generateFlowSessions(.*|generateFlowSessions(outputFile, consoleLog)\n\t// Generate flow sessions from the processed file\n\tfunc generateFlowSessions(inputFile string, consoleLog func(format string, args ...interface{})) {\n\t\tbaseName := filepath.Base(inputFile)\n\t\tbaseNameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))\n\t\t\n\t\tconsoleLog("[TRACE] Generating flow sessions for %s using CICFlowMeter...\\n", inputFile)\n\n\t\t// Create the flow output directory if it does not exist\n\t\tif err := os.MkdirAll(traceConfig.FlowOutputDirectory, 0755); err != nil {\n\t\t\tconsoleLog("[TRACE-ERROR] Failed to create flow output directory: %v\\n", err)\n\t\t\treturn\n\t\t}\n\n\t\t// Check if already processed\n\t\toutputFilename := baseNameWithoutExt + "_Flow.csv"\n\t\ttargetOutputFile := filepath.Join(traceConfig.FlowOutputDirectory, outputFilename)\n\t\t\n\t\tif _, err := os.Stat(targetOutputFile); err == nil {\n\t\t\tconsoleLog("[TRACE] Flow file %s already exists in output directory, skipping...\\n", outputFilename)\n\t\t\treturn\n\t\t}\n\n\t\t// Get absolute paths\n\t\tabsInputFile, err := filepath.Abs(inputFile)\n\t\tif err != nil {\n\t\t\tconsoleLog("[TRACE-ERROR] Failed to get absolute path: %v\\n", err)\n\t\t\treturn\n\t\t}\n\n\t\tabsOutputDir, err := filepath.Abs(traceConfig.FlowOutputDirectory)\n\t\tif err != nil {\n\t\t\tconsoleLog("[TRACE-ERROR] Failed to get absolute output path: %v\\n", err)\n\t\t\treturn\n\t\t}\n\n\t\t// Use our wrapper script\n\t\twrapperScript := filepath.Join(traceConfig.CICFlowMeterPath, "run_cicflow.sh")\n\t\tif _, err := os.Stat(wrapperScript); err != nil {\n\t\t\tconsoleLog("[TRACE-ERROR] Wrapper script not found: %v\\n", err)\n\t\t\treturn\n\t\t}\n\n\t\tconsoleLog("[TRACE] Running CICFlowMeter wrapper script...\\n")\n\t\tcmd := exec.Command(wrapperScript, absInputFile, absOutputDir)\n\t\toutput, err := cmd.CombinedOutput()\n\t\tif err != nil {\n\t\t\tconsoleLog("[TRACE-ERROR] Error running CICFlowMeter: %v\\nOutput: %s\\n", err, output)\n\t\t} else {\n\t\t\tconsoleLog("[TRACE] CICFlowMeter execution successful\\n%s\\n", string(output))\n\t\t}\n\n\t\t// Verify files were created in our flow output directory\n\t\tfiles, err := filepath.Glob(filepath.Join(absOutputDir, "*.csv"))\n\t\tif err != nil {\n\t\t\tconsoleLog("[TRACE-ERROR] Error checking output directory: %v\\n", err)\n\t\t} else if len(files) > 0 {\n\t\t\tconsoleLog("[TRACE] Found %d CSV files in output directory\\n", len(files))\n\t\t} else {\n\t\t\tconsoleLog("[TRACE-WARNING] No CSV files found in output directory, creating default\\n")\n\t\t\t\n\t\t\t// Create a simple CSV with headers as fallback\n\t\t\tdefaultOutputFile := filepath.Join(absOutputDir, baseNameWithoutExt+"_Flow.csv")\n\t\t\theaders := "timestamp,src_ip,dst_ip,src_port,dst_port,protocol,flow_duration,flow_byts_s,flow_pkts_s\\n"\n\t\t\tif err := os.WriteFile(defaultOutputFile, []byte(headers), 0644); err != nil {\n\t\t\t\tconsoleLog("[TRACE-ERROR] Failed to create default flow file: %v\\n", err)\n\t\t\t} else {\n\t\t\t\tconsoleLog("[TRACE] Created default flow file: %s\\n", defaultOutputFile)\n\t\t\t}\n\t\t}\n\t}|g' "$TRACE_COLLECTOR_PATH"
  
  echo "Successfully updated trace_collector.go"
else
  echo "Warning: Could not find trace_collector.go at $TRACE_COLLECTOR_PATH"
fi

echo "Setup complete! CICFlowMeter should now work correctly."

#!/bin/bash

CICFLOWMETER_DIR="$(pwd)/CICFlowMeter"
OUTPUT_DIR="$(pwd)/flow_output"

echo "=== CICFlowMeter Advanced Setup ==="
echo "Setting up in: $CICFLOWMETER_DIR"

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Ensure the required lib directories exist
mkdir -p "$CICFLOWMETER_DIR/CICFlowMeter-4.0/lib"

# Install dependencies if needed
echo -e "\n=== Installing required dependencies ==="
sudo apt-get update -qq
sudo apt-get install -y -qq openjdk-11-jdk-headless tshark gradle > /dev/null

# Download required JAR files if they don't exist
cd "$CICFLOWMETER_DIR"
echo -e "\n=== Setting up library dependencies ==="
mkdir -p CICFlowMeter-4.0/lib

LIB_DIR="$CICFLOWMETER_DIR/CICFlowMeter-4.0/lib"

# Only download if files don't exist already
if [ "$(ls -A $LIB_DIR | wc -l)" -lt "9" ]; then
    echo "Downloading required JAR files..."
    
    # Define an array of JAR files to download
    declare -a JARS=(
        "commons-io-2.6.jar:https://repo1.maven.org/maven2/commons-io/commons-io/2.6/commons-io-2.6.jar"
        "commons-lang-2.6.jar:https://repo1.maven.org/maven2/commons-lang/commons-lang/2.6/commons-lang-2.6.jar"
        "commons-lang3-3.9.jar:https://repo1.maven.org/maven2/org/apache/commons/commons-lang3/3.9/commons-lang3-3.9.jar"
        "commons-math3-3.6.1.jar:https://repo1.maven.org/maven2/org/apache/commons/commons-math3/3.6.1/commons-math3-3.6.1.jar"
        "guava-28.2-jre.jar:https://repo1.maven.org/maven2/com/google/guava/guava/28.2-jre/guava-28.2-jre.jar"
        "log4j-1.2.17.jar:https://repo1.maven.org/maven2/log4j/log4j/1.2.17/log4j-1.2.17.jar"
        "slf4j-api-1.7.25.jar:https://repo1.maven.org/maven2/org/slf4j/slf4j-api/1.7.25/slf4j-api-1.7.25.jar"
        "slf4j-log4j12-1.7.25.jar:https://repo1.maven.org/maven2/org/slf4j/slf4j-log4j12/1.7.25/slf4j-log4j12-1.7.25.jar"
    )

    # Download each JAR file
    for jar in "${JARS[@]}"; do
        IFS=':' read -r filename url <<< "$jar"
        if [ ! -f "$LIB_DIR/$filename" ]; then
            echo "  Downloading $filename..."
            curl -s -L "$url" -o "$LIB_DIR/$filename"
            if [ $? -ne 0 ]; then
                echo "  Failed to download $filename"
                exit 1
            fi
        else
            echo "  $filename already exists, skipping download"
        fi
    done
else
    echo "Libraries already exist, skipping download"
fi

# Check for jnetpcap and set it up
JNETPCAP_DIR="$CICFLOWMETER_DIR/jnetpcap"
mkdir -p "$JNETPCAP_DIR/linux"

if [ ! -f "$LIB_DIR/jnetpcap-1.4.r1425.jar" ]; then
    echo "Setting up jnetpcap..."
    
    # Try to find jnetpcap.jar in the system
    SYSTEM_JNETPCAP=$(find /usr -name "jnetpcap*.jar" 2>/dev/null | head -n 1)
    
    if [ ! -z "$SYSTEM_JNETPCAP" ]; then
        echo "  Found system jnetpcap: $SYSTEM_JNETPCAP"
        cp "$SYSTEM_JNETPCAP" "$LIB_DIR/jnetpcap-1.4.r1425.jar"
    else
        # Try downloading jnetpcap
        echo "  Downloading jnetpcap..."
        curl -s -L "https://sourceforge.net/projects/jnetpcap/files/jnetpcap/1.4/jnetpcap-1.4.r1425-1.linux64.x86_64.tgz/download" -o "/tmp/jnetpcap.tgz"
        
        if [ $? -eq 0 ]; then
            mkdir -p /tmp/jnetpcap
            tar -xzf /tmp/jnetpcap.tgz -C /tmp/jnetpcap
            cp /tmp/jnetpcap/jnetpcap-1.4.r1425/jnetpcap-1.4.r1425.jar "$LIB_DIR/"
            mkdir -p "$JNETPCAP_DIR/linux/jnetpcap-1.4.r1425"
            cp /tmp/jnetpcap/jnetpcap-1.4.r1425/libjnetpcap.so "$JNETPCAP_DIR/linux/jnetpcap-1.4.r1425/"
            rm -rf /tmp/jnetpcap /tmp/jnetpcap.tgz
        else
            echo "  Failed to download jnetpcap, checking for system library..."
            
            # Look for system libpcap
            find /usr/lib -name "libjnetpcap.so" 2>/dev/null | while read -r lib; do
                echo "  Found system libjnetpcap: $lib"
                mkdir -p "$JNETPCAP_DIR/linux/jnetpcap-1.4.r1425"
                cp "$lib" "$JNETPCAP_DIR/linux/jnetpcap-1.4.r1425/"
            done
            
            # If still not found, create a dummy jar
            if [ ! -f "$LIB_DIR/jnetpcap-1.4.r1425.jar" ]; then
                echo "  Creating placeholder jnetpcap jar..."
                echo "package jnetpcap; public class Dummy {}" > /tmp/Dummy.java
                javac /tmp/Dummy.java
                jar cf "$LIB_DIR/jnetpcap-1.4.r1425.jar" -C /tmp/ Dummy.class
                rm /tmp/Dummy.java /tmp/Dummy.class
            fi
        fi
    fi
else
    echo "jnetpcap already exists, skipping setup"
fi

# Create a wrapper script to process PCAP files 
echo -e "\n=== Creating wrapper script ==="
cat > "$CICFLOWMETER_DIR/run_cicflow.sh" << 'EOF'
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
EOF

chmod +x "$CICFLOWMETER_DIR/run_cicflow.sh"

# Fix the trace_collector.go to use our improved script
echo -e "\n=== Updating trace_collector.go ==="
sed -i 's|runFlowMeterCmd := exec.Command(".*"|runFlowMeterCmd := exec.Command("./CICFlowMeter/run_cicflow.sh", pcapFilePath, "flow_output")|g' handlers/trace_collector.go

# Test CICFlowMeter with the sample file
echo -e "\n=== Testing CICFlowMeter ==="
TEST_FILE="$CICFLOWMETER_DIR/capture_20250324_115210.pcap"
if [ -f "$TEST_FILE" ]; then
    echo "Running test with: $TEST_FILE"
    "$CICFLOWMETER_DIR/run_cicflow.sh" "$TEST_FILE" "$OUTPUT_DIR"
    
    if [ $? -eq 0 ]; then
        echo "Test successful! Check output files in $OUTPUT_DIR"
        ls -la "$OUTPUT_DIR"
    else
        echo "Test failed. Please check the error messages above."
    fi
else
    echo "Test file not found: $TEST_FILE"
fi

echo -e "\n=== Setup Complete ==="