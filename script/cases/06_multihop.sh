#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="06_multihop"
trap cleanup EXIT

PORT_HOP2=$((PORT_BASE + 1))

# A listens (admin)
start_admin_listen "$PORT_BASE" "$SECRET"

# B connects to A as agent1
start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET -l $PORT_HOP2"

# Wait for B to connect to A
if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "agent1 (B) did not connect to admin"
  exit 1
fi

# C connects to B as agent2
start_agent_remote C "-c ${IP_B}:${PORT_HOP2} -s $SECRET"

# Wait for second node to appear
if ! wait_for_log "new connection" 20 "$ADMIN_LOG"; then
  # Give extra time for multihop registration
  sleep 3
fi

# Send topo command and check for 2 nodes
admin_cmd "topo"
sleep 2

# Topo output should show 2 nodes (node 0 and node 1)
if assert_output_contains "node 0" && assert_output_contains "node 1"; then
  pass "$TEST_NAME"
else
  fail "$TEST_NAME" "topo does not show 2 nodes"
  exit 1
fi
