#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="12_sshtunnel"

trap cleanup EXIT

PORT=$((PORT_BASE + 12))
TUNNEL_PORT=$((PORT_BASE + 1))

# Start admin listening
start_admin_listen "$PORT" "$SECRET"

# Start agent B connecting to admin
start_agent_remote B "-c ${IP_A}:${PORT} -s ${SECRET}"

# Wait for agent B connection
if ! wait_for_log "new connection" "$CONNECT_TIMEOUT"; then
  fail "$TEST_NAME" "Agent B did not connect"
  exit 1
fi

# Start agent C listening on TUNNEL_PORT on machine C
start_agent_remote C "-l ${TUNNEL_PORT} -s ${SECRET}"

# Wait for agent C to be listening
if ! wait_for_port C "$TUNNEL_PORT" "$CONNECT_TIMEOUT"; then
  fail "$TEST_NAME" "Agent C not listening on port ${TUNNEL_PORT}"
  exit 1
fi

# Select node 1 (agent B)
admin_cmd "use 1"

# Start SSH tunnel: sshtunnel <ssh_addr> <agent_listen_port>
admin_cmd "sshtunnel ${IP_B}:22 ${TUNNEL_PORT}"

# Choose method 1 (username/password)
admin_cmd "1"

# Enter username
admin_cmd "$SSH_TEST_USER"

# Enter password
admin_cmd "$SSH_TEST_PASS"

# Leave fingerprint blank (TOFU mode)
admin_cmd ""

# Wait for tunnel to establish and new node to appear
sleep 5

# Go back to main panel
admin_cmd "back"

# Run topo to see topology
admin_cmd "topo"
sleep 2

# Assert 3 nodes visible (admin + B + C via tunnel)
OUTPUT=$(admin_output)

# The topo output should show node 0, node 1, and node 2 (3 nodes total)
# Count unique node references in output
NODE_COUNT=$(echo "$OUTPUT" | grep -oiE "node [0-9]+" | sort -u | wc -l)

if [ "$NODE_COUNT" -lt 3 ]; then
  fail "$TEST_NAME" "Expected 3 nodes in topology, found ${NODE_COUNT}"
  exit 1
fi

pass "$TEST_NAME"
