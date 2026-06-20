#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="01_basic_raw"
trap cleanup EXIT

# Admin listens on PORT_BASE
start_admin_listen "$PORT_BASE" "$SECRET"

# Agent on B connects to admin
start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET"

# Wait for agent connection
if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "agent did not connect within timeout"
  exit 1
fi

# Select node 0 and check status
admin_cmd "use 0"
admin_cmd "status"

# Assert output contains IP_B
if assert_output_contains "$IP_B"; then
  pass "$TEST_NAME"
else
  fail "$TEST_NAME" "status output does not contain $IP_B"
  exit 1
fi
