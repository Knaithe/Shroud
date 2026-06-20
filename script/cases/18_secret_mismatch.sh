#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="18_secret_mismatch"

trap cleanup EXIT

PORT=$((PORT_BASE + 18))

# Start admin listening with one secret
start_admin_listen "$PORT" "secret_one"

# Start agent B connecting with a DIFFERENT secret
start_agent_remote B "-c ${IP_A}:${PORT} -s secret_two"

# Wait enough time for a connection attempt to occur (and fail)
sleep $((CONNECT_TIMEOUT + 2))

# Assert admin log does NOT show a successful connection
if ! assert_output_not_contains "new connection"; then
  fail "$TEST_NAME" "Connection succeeded despite secret mismatch"
  exit 1
fi

# Double-check: no node should appear in topology
admin_cmd "topo"
sleep 2

# The topo output should only show the admin itself (node 0), not any agent
OUTPUT=$(admin_output)
AGENT_NODES=$(echo "$OUTPUT" | grep -oiE "node [1-9][0-9]*" | wc -l)

if [ "$AGENT_NODES" -gt 0 ]; then
  fail "$TEST_NAME" "Agent nodes appeared despite secret mismatch"
  exit 1
fi

pass "$TEST_NAME"
