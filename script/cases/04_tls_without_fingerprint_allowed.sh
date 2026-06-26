#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="04_tls_without_fingerprint_allowed"
trap cleanup EXIT

start_admin A "-l ${PORT_BASE} -s ${SECRET} --tls-enable"
start_agent_remote B "-c ${IP_A}:${PORT_BASE} -s $SECRET --tls-enable"

if wait_for_log "new connection" "$CONNECT_TIMEOUT"; then
  pass "$TEST_NAME"
else
  fail "$TEST_NAME" "agent did not connect with --tls-enable and no fingerprint"
  exit 1
fi
