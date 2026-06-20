#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="14_file_download"

trap cleanup EXIT

PORT=$((PORT_BASE + 14))
TEST_CONTENT="shroud_download_test_$(date +%s)"
REMOTE_FILE="/tmp/shroud_download_src.txt"
LOCAL_NAME="shroud_downloaded.txt"

# Create test file on agent B via SSH
ssh_cmd B "echo '${TEST_CONTENT}' > ${REMOTE_FILE}"

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

# Download file from agent B
admin_cmd "download ${REMOTE_FILE} ${LOCAL_NAME}"

# Wait for file transfer to complete
sleep 5

# Verify downloaded file exists locally and content matches
if [ ! -f "$LOCAL_NAME" ]; then
  fail "$TEST_NAME" "Downloaded file not found locally"
  exit 1
fi

LOCAL_CONTENT=$(cat "$LOCAL_NAME")

if [ "$LOCAL_CONTENT" != "$TEST_CONTENT" ]; then
  fail "$TEST_NAME" "File content mismatch: expected '${TEST_CONTENT}', got '${LOCAL_CONTENT}'"
  exit 1
fi

# Cleanup
rm -f "$LOCAL_NAME"
ssh_cmd B "rm -f ${REMOTE_FILE}" 2>/dev/null || true

pass "$TEST_NAME"
