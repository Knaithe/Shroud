#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="17_node_offline"

trap cleanup EXIT

PORT=$((PORT_BASE + 17))
LISTEN_PORT=$((PORT_BASE + 17 + 100))

# Start admin listening
start_admin_listen "$PORT" "$SECRET"

# Start agent B connecting to admin
start_agent_remote B "-c ${IP_A}:${PORT} -s ${SECRET}"

# Wait for agent B connection
if ! wait_for_log "new connection" "$CONNECT_TIMEOUT"; then
  fail "$TEST_NAME" "Agent B did not connect"
  exit 1
fi

# Have agent B listen for downstream agents
admin_cmd "use 1"
admin_cmd "listen ${LISTEN_PORT}"
sleep 2
admin_cmd "back"

# Start agent C connecting to agent B
start_agent_remote C "-c ${IP_B}:${LISTEN_PORT} -s ${SECRET}"

# Wait for agent C to appear (admin should show new node)
sleep 5

# Verify both nodes are visible
admin_cmd "topo"
sleep 2

OUTPUT_BEFORE=$(admin_output)
NODE_COUNT_BEFORE=$(echo "$OUTPUT_BEFORE" | grep -oiE "node [0-9]+" | sort -u | wc -l)

if [ "$NODE_COUNT_BEFORE" -lt 3 ]; then
  fail "$TEST_NAME" "Expected 3 nodes before killing C, found ${NODE_COUNT_BEFORE}"
  exit 1
fi

# Kill agent C
stop_agent_remote C

# Wait for offline detection
sleep 5

# Assert admin output shows offline for node C
if ! assert_output_contains "offline"; then
  fail "$TEST_NAME" "Admin did not detect node C going offline"
  exit 1
fi

# Verify agent B is still connected - use node 1
admin_cmd "use 1"
admin_cmd "back"
sleep 1

# Node 1 (B) should still be usable without error
if assert_output_contains "Node 1 seems offline" 2>/dev/null; then
  fail "$TEST_NAME" "Agent B went offline unexpectedly"
  exit 1
fi

pass "$TEST_NAME"
