#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="09_backward"
trap cleanup EXIT

BACK_LPORT=$((PORT_BASE + 13))
BACK_RPORT=$((PORT_BASE + 14))

# Admin listens, agent on B connects
start_admin_listen "$PORT_BASE" "$SECRET"
start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET"

if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "agent did not connect"
  exit 1
fi

# Select node 0, set up backward: remote BACK_LPORT on agent -> local BACK_RPORT on admin
admin_cmd "use 0"
admin_cmd "backward $BACK_LPORT $BACK_RPORT"

# Wait for backward to start
if ! wait_for_log "Backward start successfully" 10; then
  fail "$TEST_NAME" "backward did not start"
  exit 1
fi

# Start a TCP listener on A (admin side) at BACK_RPORT to receive data
RECV_FILE="/tmp/shroud_backward_recv_$$"
(nc -l -p "$BACK_RPORT" > "$RECV_FILE" 2>/dev/null || nc -l "$BACK_RPORT" > "$RECV_FILE" 2>/dev/null) &
PIDS_TO_KILL+=($!)
sleep 1

# From B, send data to localhost:BACK_LPORT (the backward listener on agent side)
TEST_DATA="backward_test_data_$$"
ssh_cmd B "echo '$TEST_DATA' | nc -w 3 127.0.0.1 $BACK_LPORT" 2>/dev/null || true
sleep 2

# Check if data was received on A
if grep -q "$TEST_DATA" "$RECV_FILE" 2>/dev/null; then
  pass "$TEST_NAME"
else
  fail "$TEST_NAME" "data not received through backward tunnel"
  exit 1
fi

rm -f "$RECV_FILE"
