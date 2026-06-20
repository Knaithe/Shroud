#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="03_basic_tls_insecure"
trap cleanup EXIT

# Both sides use --tls-enable --tls-insecure
start_admin_listen "$PORT_BASE" "$SECRET" "--tls-enable --tls-insecure"

start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET --tls-enable --tls-insecure"

# Wait for successful connection
if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "TLS insecure connection did not succeed"
  exit 1
fi

# Agent log should contain WARNING about insecure mode
if ! assert_remote_log_contains B "WARNING"; then
  fail "$TEST_NAME" "agent log does not contain WARNING for insecure TLS"
  exit 1
fi

pass "$TEST_NAME"
