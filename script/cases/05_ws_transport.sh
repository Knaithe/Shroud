#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="05_ws_transport"
trap cleanup EXIT

# Check if nginx WS proxy is available on A
if ! nc -z -w2 127.0.0.1 "$WS_PROXY_PORT" 2>/dev/null; then
  skip "$TEST_NAME" "nginx WS proxy not available on 127.0.0.1:${WS_PROXY_PORT}"
  exit 0
fi

# Admin listens with websocket downstream
start_admin_listen "$PORT_BASE" "$SECRET" "--down ws --domain localhost"

# Agent on B connects via WS through the nginx proxy on A
start_agent_remote B "-c ${IP_A}:${WS_PROXY_PORT} -s $SECRET --down ws --domain localhost"

# Wait for connection
if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "WS transport connection did not succeed"
  exit 1
fi

pass "$TEST_NAME"
