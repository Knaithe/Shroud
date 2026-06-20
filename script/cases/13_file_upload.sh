#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="13_file_upload"

trap cleanup EXIT

PORT=$((PORT_BASE + 13))
TEST_CONTENT="shroud_upload_test_$(date +%s)"
LOCAL_FILE="/tmp/shroud_testfile.txt"
REMOTE_NAME="testfile_remote.txt"

# Create test file on admin machine (A)
echo "$TEST_CONTENT" > "$LOCAL_FILE"

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

# Upload file to agent B
admin_cmd "upload ${LOCAL_FILE} ${REMOTE_NAME}"

# Wait for file transfer to complete
sleep 5

# Verify file exists on B and content matches
REMOTE_CONTENT=$(ssh_cmd B "cat ${REMOTE_NAME} 2>/dev/null" || true)

if [ -z "$REMOTE_CONTENT" ]; then
  fail "$TEST_NAME" "Uploaded file not found on agent B"
  exit 1
fi

if [ "$REMOTE_CONTENT" != "$TEST_CONTENT" ]; then
  fail "$TEST_NAME" "File content mismatch: expected '${TEST_CONTENT}', got '${REMOTE_CONTENT}'"
  exit 1
fi

# Cleanup remote file
ssh_cmd B "rm -f ${REMOTE_NAME}" 2>/dev/null || true
rm -f "$LOCAL_FILE"

pass "$TEST_NAME"
