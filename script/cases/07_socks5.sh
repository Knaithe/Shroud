#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="07_socks5"
trap cleanup EXIT

SOCKS_PORT=$((PORT_BASE + 10))

# Admin listens, agent on B connects
start_admin_listen "$PORT_BASE" "$SECRET"
start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET"

if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "agent did not connect"
  exit 1
fi

# Select node 0 and start socks5 proxy
admin_cmd "use 0"
admin_cmd "socks $SOCKS_PORT"

# Wait for socks to start
if ! wait_for_log "Socks start successfully" 10; then
  fail "$TEST_NAME" "socks5 proxy did not start"
  exit 1
fi

# Use curl through the socks5 proxy
if curl -s --max-time 10 --socks5 "127.0.0.1:$SOCKS_PORT" http://httpbin.org/ip > /dev/null 2>&1; then
  pass "$TEST_NAME"
else
  fail "$TEST_NAME" "curl through socks5 proxy failed"
  exit 1
fi
