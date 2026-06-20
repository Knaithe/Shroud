#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="20_tor_proxy"

trap cleanup EXIT

PORT=$((PORT_BASE + 20))

# Check if Tor SOCKS proxy is available
TOR_HOST=$(echo "$TOR_PROXY" | cut -d: -f1)
TOR_PORT=$(echo "$TOR_PROXY" | cut -d: -f2)

if ! nc -z -w2 "$TOR_HOST" "$TOR_PORT" 2>/dev/null; then
  skip "$TEST_NAME" "Tor proxy not running at ${TOR_PROXY}"
  exit 0
fi

# Start admin listening
start_admin_listen "$PORT" "$SECRET"

# Start agent B connecting via Tor proxy
start_agent_remote B "-c ${IP_A}:${PORT} -s ${SECRET} --tor-proxy ${TOR_PROXY}"

# Wait for connection (Tor can be slow, use extended timeout)
TOR_TIMEOUT=$((CONNECT_TIMEOUT * 3))

if ! wait_for_log "new connection" "$TOR_TIMEOUT"; then
  fail "$TEST_NAME" "Agent B did not connect via Tor within ${TOR_TIMEOUT}s"
  exit 1
fi

# Verify node is present
admin_cmd "topo"
sleep 2

if ! assert_output_contains "node"; then
  fail "$TEST_NAME" "No nodes visible after Tor connection"
  exit 1
fi

pass "$TEST_NAME"
