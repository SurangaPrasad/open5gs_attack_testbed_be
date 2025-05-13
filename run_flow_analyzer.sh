#!/bin/bash

# Service script to run the flow analyzer in the background
# This script starts, stops, and checks the status of the flow analyzer

FLOW_ANALYZER_PATH="/home/open5gs1/Documents/open5gs-be/utils/flow_analyzer.py"
FLOW_ANALYZER_LOG="/home/open5gs1/Documents/open5gs-be/logs/flow_analyzer_service.log"
PID_FILE="/home/open5gs1/Documents/open5gs-be/flow_analyzer.pid"

# Make sure dependencies are installed
check_dependencies() {
    echo "Checking dependencies..."
    python3 -c "import pandas, watchdog" 2>/dev/null || {
        echo "Installing required Python packages..."
        pip3 install pandas watchdog
    }
}

# Start the flow analyzer service
start_analyzer() {
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if ps -p "$pid" > /dev/null; then
            echo "Flow analyzer is already running (PID: $pid)"
            return 0
        else
            echo "Stale PID file found, removing..."
            rm -f "$PID_FILE"
        fi
    fi
    
    # Make the analyzer script executable
    chmod +x "$FLOW_ANALYZER_PATH"
    
    # Make sure log directory exists
    mkdir -p $(dirname "$FLOW_ANALYZER_LOG")
    
    echo "Starting flow analyzer service..."
    nohup python3 "$FLOW_ANALYZER_PATH" > "$FLOW_ANALYZER_LOG" 2>&1 &
    pid=$!
    echo $pid > "$PID_FILE"
    echo "Flow analyzer started with PID: $pid"
}

# Stop the flow analyzer service
stop_analyzer() {
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if ps -p "$pid" > /dev/null; then
            echo "Stopping flow analyzer (PID: $pid)..."
            kill -15 "$pid"
            sleep 2
            if ps -p "$pid" > /dev/null; then
                echo "Force stopping flow analyzer..."
                kill -9 "$pid"
            fi
            rm -f "$PID_FILE"
            echo "Flow analyzer stopped"
        else
            echo "No running flow analyzer found"
            rm -f "$PID_FILE"
        fi
    else
        echo "No PID file found, flow analyzer may not be running"
    fi
}

# Check the status of the flow analyzer
status_analyzer() {
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if ps -p "$pid" > /dev/null; then
            echo "Flow analyzer is running (PID: $pid)"
            echo "Last 5 log entries:"
            tail -n 5 "$FLOW_ANALYZER_LOG"
            echo ""
            echo "Last 5 attack detections:"
            tail -n 5 "/home/open5gs1/Documents/open5gs-be/logs/attack_detection.log"
        else
            echo "PID file exists but flow analyzer is not running"
        fi
    else
        echo "Flow analyzer is not running"
    fi
}

# Show usage if no args
show_usage() {
    echo "Usage: $0 {start|stop|restart|status}"
    echo "  start   - Start the flow analyzer service"
    echo "  stop    - Stop the flow analyzer service"
    echo "  restart - Restart the flow analyzer service"
    echo "  status  - Show the flow analyzer status"
}

# Main execution logic
case "$1" in
    start)
        check_dependencies
        start_analyzer
        ;;
    stop)
        stop_analyzer
        ;;
    restart)
        stop_analyzer
        sleep 2
        check_dependencies
        start_analyzer
        ;;
    status)
        status_analyzer
        ;;
    *)
        show_usage
        exit 1
        ;;
esac

exit 0