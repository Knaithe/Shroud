#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="19_path_traversal"

trap cleanup EXIT

PORT=$((PORT_BASE + 19))
STOLEN_FILE="/tmp/stolen"

# Remove any leftover file from previous runs
rm -f "$STOLEN_FILE"

# Start admin listening
start_admin_listen "$PORT" "$SECRET"

# Start agent B connecting to admin
start_agent_remote B "-c ${IP_A}:${PORT} -s ${SECRET}"

# Wait for agent connection
if ! wait_for_log "new connection" "$CONNECT_TIMEOUT"; then
  fail "$TEST_NAME" "Agent B did not connect"
  exit 1
fi

# Select node 1 (agent B)
admin_cmd "use 1"

# Attempt path traversal download
admin_cmd "download ../../../etc/shadow ${STOLEN_FILE}"

# Wait for the operation to be processed
sleep 5

# Assert agent rejects the request - admin output should contain an error
OUTPUT=$(admin_output)
if echo "$OUTPUT" | grep -qiE "not allowed|error|traversal|denied|unable"; then
  : # Expected: error message in output
elif ! [ -f "$STOLEN_FILE" ]; then
  : # Also acceptable: file simply was not created (silent rejection)
else
  fail "$TEST_NAME" "Path traversal was not rejected and file was created"
  exit 1
fi

# Verify the stolen file does NOT exist on admin machine
if [ -f "$STOLEN_FILE" ]; then
  rm -f "$STOLEN_FILE"
  fail "$TEST_NAME" "/tmp/stolen was created - path traversal succeeded"
  exit 1
fi

pass "$TEST_NAME"
