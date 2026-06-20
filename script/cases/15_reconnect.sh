#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="15_reconnect"

trap cleanup EXIT

PORT=$((PORT_BASE + 15))
WAIT_SECS=$((RECONNECT_INTERVAL + 5))

# Start agent B with reconnect, connecting to admin
start_agent_remote B "-c ${IP_A}:${PORT} -s ${SECRET} --reconnect ${RECONNECT_INTERVAL}"

# Start admin listening (first session)
start_admin_listen "$PORT" "$SECRET"

# Wait for initial connection
if ! wait_for_log "new connection" "$CONNECT_TIMEOUT"; then
  fail "$TEST_NAME" "Agent B did not connect initially"
  exit 1
fi

# Kill admin process to simulate disconnect
pkill -f shroud_admin 2>/dev/null || true
for pid in "${PIDS_TO_KILL[@]}"; do
  kill "$pid" 2>/dev/null || true
done
PIDS_TO_KILL=()
rm -f "$ADMIN_FIFO"
sleep 1

# Restart admin listening (second session)
start_admin_listen "$PORT" "$SECRET"

# Wait for agent to auto-reconnect (reconnect interval + buffer)
if ! wait_for_log "new connection" "$WAIT_SECS"; then
  fail "$TEST_NAME" "Agent B did not reconnect after ${WAIT_SECS}s"
  exit 1
fi

pass "$TEST_NAME"
