#!/bin/bash
# Shroud Integration Test Environment
# Edit these variables before running tests
#
# TIP: To avoid committing real IPs/passwords, create:
#   script/test_env.local.sh  (gitignored)
# with overridden values. It will be sourced automatically if present.

# Machine IPs (placeholders — replace with real values or use .local.sh override)
export IP_A="10.0.0.1"
export IP_B="10.0.0.2"
export IP_C="10.0.0.3"

# SSH user for remote execution (must have passwordless SSH to B and C)
export SSH_USER="root"

# Shared secret for all tests (override per-case as needed)
export SECRET="integration-test-secret"

# Binary paths (on each machine after deployment)
export ADMIN_BIN="/opt/shroud/shroud_admin"
export AGENT_BIN="/opt/shroud/shroud_agent"

# Build output directory (on machine A)
export BUILD_DIR="$(cd "$(dirname "$0")/.." && pwd)/release"

# Base port (tests use PORT_BASE+0 through PORT_BASE+20)
export PORT_BASE=13000

# Test log directory
export LOG_DIR="/tmp/shroud_test_logs"

# SSH test credentials (for ssh/sshtunnel tests, user on machine B)
export SSH_TEST_USER="testuser"
export SSH_TEST_PASS="testpass"

# Timeouts
export CONNECT_TIMEOUT=10
export RECONNECT_INTERVAL=3

# Optional: Tor proxy address (on machine A, for tor tests)
export TOR_PROXY="127.0.0.1:9050"

# Optional: nginx WS proxy port (on machine A)
export WS_PROXY_PORT=8080
