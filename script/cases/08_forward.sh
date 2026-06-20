#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="08_forward"
trap cleanup EXIT

HTTP_PORT=$((PORT_BASE + 11))
FWD_PORT=$((PORT_BASE + 12))

# Admin listens, agent on B connects
start_admin_listen "$PORT_BASE" "$SECRET"
start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET"

if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "agent did not connect"
  exit 1
fi

# Start a simple HTTP server on B
ssh_bg B "python3 -m http.server $HTTP_PORT --bind 0.0.0.0"
sleep 1

# Select node 0, set up forward: local FWD_PORT -> B's HTTP_PORT
admin_cmd "use 0"
admin_cmd "forward $FWD_PORT ${IP_B}:${HTTP_PORT}"

# Wait for forward to start
if ! wait_for_log "Forward start successfully" 10; then
  fail "$TEST_NAME" "forward did not start"
  exit 1
fi

# curl the forwarded port on localhost
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "http://127.0.0.1:$FWD_PORT/" 2>/dev/null || echo "000")

if [ "$HTTP_CODE" = "200" ]; then
  pass "$TEST_NAME"
else
  fail "$TEST_NAME" "expected HTTP 200, got $HTTP_CODE"
  exit 1
fi
