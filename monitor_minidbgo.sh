#!/bin/bash

# --- Configuration ---
PROCESS_NAME="MiniDBGo" # Name used in 'go run ./cmd/MiniDBGo'
# If you run the compiled binary, change this to:
# PROCESS_NAME="minidbgo"

INTERVAL=2 # Check every 2 seconds

# --- Find the Process ID (PID) ---
echo "Searching for MiniDBGo process ($PROCESS_NAME)..."
PID=$(pgrep -f "$PROCESS_NAME" | head -n 1) # Find PID containing the name, take the first one

if [ -z "$PID" ]; then
  echo "Error: MiniDBGo process ($PROCESS_NAME) not found running."
  echo "Please start MiniDBGo first (e.g., 'go run ./cmd/MiniDBGo')."
  exit 1
fi

echo "Found MiniDBGo process with PID: $PID"
echo "Monitoring CPU and RAM usage every $INTERVAL seconds. Press Ctrl+C to stop."
echo "-----------------------------------------------------"
printf "%-20s %-10s %-10s %-12s\n" "Timestamp" "%CPU" "%MEM" "RSS (KB)"
echo "-----------------------------------------------------"

# --- Monitoring Loop ---
while true; do
  # Check if process still exists
  if ! ps -p "$PID" > /dev/null; then
    echo "MiniDBGo process (PID: $PID) terminated. Exiting monitor."
    exit 0
  fi

  # Get CPU, %Memory, and Resident Set Size (RSS in KB) using ps
  # The exact 'ps' options might vary slightly between Linux/macOS
  # This version tries to be compatible with both
  STATS=$(ps -p "$PID" -o %cpu=,%mem=,rss= | awk '{printf "%.1f %.1f %d", $1, $2, $3}')

  CPU=$(echo "$STATS" | awk '{print $1}')
  MEM_PERCENT=$(echo "$STATS" | awk '{print $2}')
  RSS_KB=$(echo "$STATS" | awk '{print $3}')
  TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

  # Print the stats
  printf "%-20s %-10s %-10s %-12s\n" "$TIMESTAMP" "$CPU" "$MEM_PERCENT" "$RSS_KB"

  # Wait for the next interval
  sleep "$INTERVAL"
done

# Trap Ctrl+C for clean exit
trap "echo '\nMonitoring stopped.'; exit 0" INT