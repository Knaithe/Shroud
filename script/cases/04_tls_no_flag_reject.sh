#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

TEST_NAME="04_tls_no_flag_reject"
trap cleanup EXIT

# Agent on B tries --tls-enable without --tls-fingerprint or --tls-insecure
# This should fail immediately at option parsing
AGENT_STDERR=$(ssh_cmd B "$AGENT_BIN -c ${IP_A}:${PORT_BASE} -s $SECRET --tls-enable" 2>&1 || true)
AGENT_EXIT=$?

# The agent should have exited nonzero (or printed the error and exited)
# Check stderr contains "requires"
if echo "$AGENT_STDERR" | grep -qi "requires"; then
  pass "$TEST_NAME"
else
  fail "$TEST_NAME" "agent stderr does not contain 'requires'; got: $AGENT_STDERR"
  exit 1
fi
