#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="02_basic_tls_fp"
trap cleanup EXIT

# Start admin with TLS enabled (insecure on server side to auto-generate cert)
start_admin_listen "$PORT_BASE" "$SECRET" "--tls-enable --tls-insecure"

# Extract the fingerprint from admin log (stderr is merged into the log)
# Format: [*] TLS certificate fingerprint (SHA256): <hex>
if ! wait_for_log "TLS certificate fingerprint" 10; then
  fail "$TEST_NAME" "admin did not print TLS fingerprint"
  exit 1
fi

CAPTURED_FP=$(grep -o 'fingerprint (SHA256): [0-9a-f]*' "$ADMIN_LOG" | head -1 | awk '{print $NF}')
if [ -z "$CAPTURED_FP" ]; then
  fail "$TEST_NAME" "could not extract fingerprint from admin log"
  exit 1
fi

# Start agent on B with the captured fingerprint for pinning
start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET --tls-enable --tls-fingerprint $CAPTURED_FP"

# Wait for successful connection
if ! wait_for_log "new connection" 15; then
  fail "$TEST_NAME" "agent with TLS fingerprint pinning did not connect"
  exit 1
fi

pass "$TEST_NAME"
