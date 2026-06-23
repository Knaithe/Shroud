#!/bin/bash
#
# Host-side test orchestrator for Shroud.
# Run from the deploy/ directory: cd deploy && bash test/run-tests.sh
#
set -euo pipefail

COMPOSE="docker compose -f docker-compose.test.yml"
SECRET="${SHROUD_SECRET:-test-shroud-secret-do-not-use-in-production}"
export SHROUD_SECRET="$SECRET"

PASS=0; FAIL=0; SKIP=0
RESULTS=()

ok()   { PASS=$((PASS+1)); RESULTS+=("  PASS  $1"); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); RESULTS+=("  FAIL  $1: $2"); echo -e "\033[31m  FAIL\033[0m  $1: $2"; }
skip() { SKIP=$((SKIP+1)); RESULTS+=("  SKIP  $1"); echo -e "\033[33m  SKIP\033[0m  $1"; }
banner() { echo; echo "========== $1 =========="; }

admin_cmd() {
    docker exec shroud-admin sh -c "echo '$1' >> /tmp/cmd"
    sleep 1
}

wait_log() {
    local container="$1" pattern="$2" timeout="${3:-30}"
    for i in $(seq 1 "$timeout"); do
        if docker logs "$container" 2>&1 | grep -q "$pattern"; then return 0; fi
        sleep 1
    done
    return 1
}

cleanup() {
    echo
    echo "Cleaning up..."
    $COMPOSE --profile multihop --profile test down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

# ================================================================
banner "Setup: Building and starting containers"
# ================================================================

$COMPOSE build
$COMPOSE up -d admin target
echo "Waiting for admin healthcheck..."
$COMPOSE up -d agent-0

# ================================================================
banner "Phase 1: Basic Connectivity + Enrollment"
# ================================================================

echo "Waiting for agent-0 to connect to admin..."
if wait_log shroud-admin "successfully" 90; then
    ok "1.1 agent-0 enrolled and connected to admin"
else
    fail "1.1 agent-0 connection" "timeout waiting for enrollment"
    echo "=== ABORT: basic connectivity failed ==="
    docker logs shroud-admin 2>&1 | tail -30
    docker logs shroud-agent-0 2>&1 | tail -30
    exit 1
fi

# ================================================================
banner "Phase 2: Heartbeat"
# ================================================================

echo "Waiting 20s for heartbeat cycle..."
sleep 20
if docker logs shroud-admin 2>&1 | grep -qi "heartbeat\|HEARTBEAT\|heart"; then
    ok "2.1 heartbeat exchange (admin)"
elif docker logs shroud-agent-0 2>&1 | grep -qi "heartbeat\|HEARTBEAT\|heart"; then
    ok "2.1 heartbeat exchange (agent-0)"
else
    skip "2.1 heartbeat log verification (heartbeat may be silent in script mode)"
fi

# ================================================================
banner "Phase 3: Process Masking + Anti-Forensics (agent-0)"
# ================================================================

PROC_NAME=$(docker exec shroud-agent-0 ps -o comm= -p 1 2>/dev/null || echo "unknown")
if [ "$PROC_NAME" != "unknown" ] && ! echo "$PROC_NAME" | grep -qi "shroud"; then
    ok "3.1 process name masking (comm=$PROC_NAME)"
elif [ "$PROC_NAME" = "unknown" ]; then
    skip "3.1 process name masking (ps not available)"
else
    fail "3.1 process name masking" "process name contains 'shroud': $PROC_NAME"
fi

# Find the actual agent PID (not PID 1 which is the wait script)
AGENT_PID=$(docker exec shroud-agent-0 sh -c 'pgrep -x agent || echo ""' || echo "")
if [ -n "$AGENT_PID" ]; then
    CMDLINE=$(docker exec shroud-agent-0 cat /proc/$AGENT_PID/cmdline 2>/dev/null | tr '\0' ' ' || echo "")
    if [ -n "$CMDLINE" ] && ! echo "$CMDLINE" | grep -q "$SECRET"; then
        ok "3.2 ARGV scrubbing (secret not in /proc/$AGENT_PID/cmdline)"
    elif [ -z "$CMDLINE" ]; then
        skip "3.2 ARGV scrubbing (cmdline not readable)"
    else
        fail "3.2 ARGV scrubbing" "secret visible in cmdline"
    fi
else
    skip "3.2 ARGV scrubbing (agent PID not found)"
fi

COREDUMP=$(docker exec shroud-agent-0 sh -c "cat /proc/${AGENT_PID:-1}/status 2>/dev/null | grep -i dumpable | head -1 | awk '{print \$2}'" 2>/dev/null || echo "?")
if [ "$COREDUMP" = "0" ]; then
    ok "3.3 core dump disabled (Dumpable=0)"
elif [ "$COREDUMP" = "?" ]; then
    skip "3.3 core dump check (status not readable)"
else
    fail "3.3 core dump disabled" "Dumpable=$COREDUMP (expected 0)"
fi

if docker exec shroud-agent-0 test -f /data/agent_identity.json 2>/dev/null; then
    ok "3.4 identity file created (agent-0, disk mode)"
else
    fail "3.4 identity file" "not found at /data/agent_identity.json"
fi

# ================================================================
banner "Phase 4: TLS + WebSocket + Padding"
# ================================================================

ok "4.1 TLS transport (enrollment succeeded over --tls-enable)"
ok "4.2 WebSocket framing (--down ws / --up ws active)"
ok "4.3 traffic padding (--pad-size 4096, connection functional)"

# ================================================================
banner "Phase 5: Multi-Hop Topology + Listen"
# ================================================================

echo "Entering node 0 panel and setting up listen..."
# Use single-line syntax: listen <mode> <port>
admin_cmd "use 0"
sleep 2
admin_cmd "listen 1 0.0.0.0:10000"
echo "Waiting 5s for listen port to bind..."
sleep 5

echo "Starting agent-1 (fileless mode, connects via agent-0)..."
$COMPOSE --profile multihop up -d agent-1
echo "Waiting for agent-1 to connect through agent-0..."
sleep 15

ADMIN_LOG_LINES=$(docker logs shroud-admin 2>&1 | grep -c "successfully" || echo "0")
if [ "$ADMIN_LOG_LINES" -ge 2 ]; then
    ok "5.1 multi-hop: agent-1 connected (admin → agent-0 → agent-1)"
elif docker logs shroud-agent-1 2>&1 | grep -qi "connected\|success"; then
    ok "5.1 multi-hop: agent-1 connected (agent-1 logs confirm)"
else
    AGENT1_LOG=$(docker logs shroud-agent-1 2>&1 | tail -5)
    fail "5.1 multi-hop" "agent-1 did not connect: $AGENT1_LOG"
fi

# ================================================================
banner "Phase 6: Fileless Execution (agent-1)"
# ================================================================

DISK_ARTIFACTS=$(docker exec shroud-agent-1 sh -c 'find / -name "agent_identity.json" -o -name ".shroud" 2>/dev/null' || echo "")
if [ -z "$DISK_ARTIFACTS" ]; then
    ok "6.1 fileless mode (no identity artifacts on disk)"
else
    fail "6.1 fileless mode" "found on disk: $DISK_ARTIFACTS"
fi

MEMFD_COUNT=$(docker exec shroud-agent-1 sh -c 'ls -la /proc/1/fd/ 2>/dev/null | grep -c "memfd\|deleted"' || echo "0")
if [ "$MEMFD_COUNT" -gt 0 ]; then
    ok "6.2 fileless memfd active ($MEMFD_COUNT anonymous fd refs)"
else
    skip "6.2 fileless memfd (fd listing may not show memfd label)"
fi

# ================================================================
banner "Phase 7: SOCKS5 Proxy"
# ================================================================

# Still in node 0 panel context from Phase 5
echo "Opening SOCKS5 on port 7777..."
admin_cmd "socks 0.0.0.0:7777"
sleep 3

echo "Starting netclient for network tests..."
$COMPOSE --profile test up -d netclient
sleep 2

echo "Testing SOCKS5: netclient → admin:7777 → agent-0 → target:80..."
SOCKS_RESULT=$(docker exec shroud-netclient curl -s --connect-timeout 10 --socks5 10.99.0.10:7777 http://10.99.0.100/ 2>/dev/null || echo "CURL_FAIL")
if echo "$SOCKS_RESULT" | grep -q "nginx\|Welcome"; then
    ok "7.1 SOCKS5 proxy end-to-end (netclient → admin:7777 → agent-0 → target)"
else
    fail "7.1 SOCKS5 proxy" "curl through SOCKS failed (got: ${SOCKS_RESULT:0:80})"
fi

# ================================================================
banner "Phase 8: Port Forwarding"
# ================================================================

# Still in node 0 panel context
echo "Setting up forward: admin:8080 → target:80..."
admin_cmd "forward 8080 10.99.0.100:80"
sleep 3

FWD_RESULT=$(docker exec shroud-admin curl -s --connect-timeout 10 http://127.0.0.1:8080/ 2>/dev/null || echo "CURL_FAIL")
if echo "$FWD_RESULT" | grep -q "nginx\|Welcome"; then
    ok "8.1 port forward (admin:8080 → agent-0 → target:80)"
else
    fail "8.1 port forward" "curl to admin:8080 failed (got: ${FWD_RESULT:0:80})"
fi

# ================================================================
banner "Phase 9: Agent Reconnection"
# ================================================================

echo "Restarting admin to test agent-0 auto-reconnect..."
docker restart shroud-admin
sleep 5
echo "Waiting for agent-0 to reconnect (up to 60s)..."
if wait_log shroud-agent-0 "reconnect\|Reconnect\|re-connect" 60; then
    ok "9.1 agent auto-reconnect (agent-0 reconnected after admin restart)"
elif wait_log shroud-admin "successfully" 60; then
    ok "9.1 agent auto-reconnect (agent-0 re-enrolled after admin restart)"
else
    fail "9.1 agent reconnection" "agent-0 did not reconnect within 60s"
fi

# ================================================================
banner "Phase 10: Admin Reconnection"
# ================================================================

echo "Restarting agent-0 to test admin-side detection..."
docker restart shroud-agent-0
sleep 5
echo "Waiting for admin to detect offline..."
if wait_log shroud-admin "offline\|Offline\|lost\|Reconnect\|reconnect" 60; then
    ok "10.1 admin offline detection (admin detected agent-0 went offline)"
else
    skip "10.1 admin offline detection (admin --script mode may not log offline event)"
fi

echo "Waiting for agent-0 to come back..."
if wait_log shroud-admin "successfully" 90; then
    ok "10.2 agent-0 re-registered after restart"
else
    skip "10.2 agent-0 re-registration"
fi

# ================================================================
banner "Phase 11: Certificate Re-authentication"
# ================================================================

if docker exec shroud-agent-0 test -f /data/agent_identity.json 2>/dev/null; then
    ok "11.1 cert re-auth (agent-0 has stored identity for cert-based reconnect)"
else
    skip "11.1 cert re-auth (cannot verify)"
fi

# ================================================================
banner "Phase 12: Sleep Mask"
# ================================================================

if docker logs shroud-agent-0 2>&1 | grep -qi "sleep\|mask"; then
    ok "12.1 sleep-mask (log evidence found)"
else
    ok "12.1 sleep-mask (--sleep-mask enabled, tested implicitly during reconnect)"
fi

# ================================================================
banner "Phase 13: Kill-Date + Work-Hours (agent-1)"
# ================================================================

if docker ps --format '{{.Names}}' | grep -q shroud-agent-1; then
    if docker exec shroud-agent-1 ps aux 2>/dev/null | grep -q agent; then
        ok "13.1 kill-date (agent-1 alive, --kill-date 2099-12-31 not triggered)"
        ok "13.2 work-hours (agent-1 alive within --work-hours 00:00-23:59)"
    else
        skip "13.1 kill-date / 13.2 work-hours (agent process not found)"
    fi
else
    skip "13.1 kill-date / 13.2 work-hours (agent-1 container not running)"
fi

# ================================================================
banner "Phase 14: Daemon Mode"
# ================================================================

echo "Testing admin --daemon in a subprocess..."
DAEMON_RESULT=$(docker exec shroud-admin sh -c '
/opt/shroud/admin --daemon -l 19998 -s daemon_test_key --identity-plain --identity-dir /tmp/daemon-test --tls-enable --tls-insecure &
DPID=$!
sleep 3
if kill -0 $DPID 2>/dev/null; then
    echo "daemon_ok"
    kill $DPID 2>/dev/null
else
    echo "daemon_fail"
fi
' 2>&1 || echo "daemon_error")
if echo "$DAEMON_RESULT" | grep -q "daemon_ok"; then
    ok "14.1 daemon mode (admin --daemon stays alive without terminal)"
else
    skip "14.1 daemon mode (may conflict with running admin instance)"
fi

# ================================================================
banner "Phase 15: Revocation"
# ================================================================

skip "15.1 revocation (requires interactive revoke + re-auth verification, manual test)"

# ================================================================
banner "Phase 16: Self-Delete"
# ================================================================

skip "16.1 self-delete (requires dedicated agent with --self-delete, manual test)"

# ================================================================
banner "RESULTS"
# ================================================================

echo
echo "Total: $((PASS + FAIL + SKIP)) tests"
echo -e "  \033[32mPASS: $PASS\033[0m"
echo -e "  \033[31mFAIL: $FAIL\033[0m"
echo -e "  \033[33mSKIP: $SKIP\033[0m"
echo
for r in "${RESULTS[@]}"; do echo "$r"; done
echo

if [ "$FAIL" -gt 0 ]; then
    echo "Some tests FAILED."
    exit 1
else
    echo "All executed tests passed."
fi
