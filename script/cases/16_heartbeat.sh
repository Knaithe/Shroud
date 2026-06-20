#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="16_heartbeat"

trap cleanup EXIT

PORT=$((PORT_BASE + 16))

# Start admin listening with heartbeat enabled
start_admin_listen "$PORT" "$SECRET" "--heartbeat"

# Start agent B connecting to admin
start_agent_remote B "-c ${IP_A}:${PORT} -s ${SECRET}"

# Wait for agent connection
if ! wait_for_log "new connection" "$CONNECT_TIMEOUT"; then
  fail "$TEST_NAME" "Agent B did not connect"
  exit 1
fi

# Wait 15 seconds for heartbeat to keep connection alive
sleep 15

# Assert no offline messages in admin log
if ! assert_output_not_contains "offline"; then
  fail "$TEST_NAME" "Unexpected offline message found while heartbeat is active"
  exit 1
fi

# Verify connection is still alive by running topo
admin_cmd "topo"
sleep 2

# Should still show node 1
if ! assert_output_contains "node"; then
  fail "$TEST_NAME" "Node disappeared despite heartbeat"
  exit 1
fi

pass "$TEST_NAME"
