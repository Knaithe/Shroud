#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="10_shell"
trap cleanup EXIT

# Admin listens, agent on B connects
start_admin_listen "$PORT_BASE" "$SECRET"
start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET"

if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "agent did not connect"
  exit 1
fi

# Select node 0 and start shell
admin_cmd "use 0"
admin_cmd "shell"

# Wait for shell to be ready
if ! wait_for_log "response" 10; then
  sleep 2
fi

# Send a command with a unique marker
admin_cmd "echo shroud_test_marker"
sleep 2

# Assert output contains the marker
if assert_output_contains "shroud_test_marker"; then
  # Exit shell mode
  admin_cmd "exit"
  sleep 1
  pass "$TEST_NAME"
else
  admin_cmd "exit"
  sleep 1
  fail "$TEST_NAME" "shell output does not contain marker"
  exit 1
fi
