#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="11_ssh"

trap cleanup EXIT

PORT=$((PORT_BASE + 11))

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

# Start SSH to B's local SSH server
admin_cmd "ssh ${IP_B}:22"

# Choose method 1 (username/password)
admin_cmd "1"

# Enter username
admin_cmd "$SSH_TEST_USER"

# Enter password
admin_cmd "$SSH_TEST_PASS"

# Leave fingerprint blank (TOFU mode)
admin_cmd ""

# Wait for SSH session to establish
sleep 3

# Assert agent log contains TOFU message
if ! wait_for_remote_log B "TOFU" 10; then
  fail "$TEST_NAME" "Agent log does not contain TOFU message"
  exit 1
fi

# Send exit to close SSH session
admin_cmd "exit"
sleep 2

pass "$TEST_NAME"
