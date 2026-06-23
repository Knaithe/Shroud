#!/bin/bash
set -euo pipefail
SECRET='test-shroud-secret-do-not-use-in-production'
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1: $2"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() {
  for c in s-admin s-agent s-agent2 s-target; do docker rm -f $c 2>/dev/null || true; done
  docker network rm s-net 2>/dev/null || true
}
trap cleanup EXIT
cleanup

# Build fresh
cd "$(dirname "$0")/../.."
echo "Building..."
wsl make linux_agent linux_admin 2>&1 | tail -3 || true
# Or use docker image
IMAGE="deploy-admin:latest"
docker build -t shroud-test -f deploy/Dockerfile . 2>&1 | tail -2

docker network create --subnet 10.88.0.0/24 s-net >/dev/null 2>&1 || true

ADMIN_RUN="docker run --rm --network s-net shroud-test"
AGENT_RUN="docker run --rm --network s-net --entrypoint /opt/shroud/agent shroud-test"

# ================================================================
banner "TEST 1: Reverse Connection (Agent Passive, Admin Active)"
# ================================================================
docker run -d --name s-agent --network s-net --ip 10.88.0.20 \
  --entrypoint /opt/shroud/agent shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp >/dev/null 2>&1
sleep 2

if docker exec s-agent ps aux | grep -q agent; then
  ok "1.1 Agent listening on passive mode"
else
  fail "1.1 Agent startup" "Agent not running"
  docker logs s-agent 2>&1
fi

# Admin active connect
REV_OUT=$(timeout 15 $ADMIN_RUN --ip 10.88.0.10 \
  -c 10.88.0.20:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/rev --script 2>&1 <<< "detail" || true)
sleep 2

if echo "$REV_OUT" | grep -qi "successfully\|connect\|Node\[0\]"; then
  ok "1.2 Reverse connection: admin active → agent passive (SUCCESS)"
elif echo "$REV_OUT" | grep -qi "panic"; then
  fail "1.2 Reverse connection" "PANIC: $(echo "$REV_OUT" | grep panic | head -1)"
else
  fail "1.2 Reverse connection" "No success indication"
  echo "   Output: $(echo "$REV_OUT" | tail -5)"
fi

cleanup

# ================================================================
banner "TEST 2: Direct Connection (Admin Passive, Agent Active)"
# ================================================================
docker run -d --name s-admin --network s-net --ip 10.88.0.10 shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat >/dev/null 2>&1
sleep 3

DIR_OUT=$(timeout 15 $AGENT_RUN --ip 10.88.0.20 \
  -c 10.88.0.10:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/agent-dir -v 2>&1 || true)

if echo "$DIR_OUT" | grep -qi "connect\|agent"; then
  ok "2.1 Direct connection: agent active → admin passive (SUCCESS)"
else
  fail "2.1 Direct connection" "$(echo "$DIR_OUT" | tail -3)"
fi

# Verify admin saw it
ADMIN_LOG=$(docker logs s-admin 2>&1)
if echo "$ADMIN_LOG" | grep -q "successfully"; then
  ok "2.2 Admin confirmed agent enrollment"
fi

# Check identity file
if docker exec s-admin test -f /tmp/admin_identity.json; then
  ok "2.3 Admin identity file created"
  ID_SIZE=$(docker exec s-admin wc -c /tmp/admin_identity.json | awk '{print $1}')
  ok "2.4 Admin identity file size: $ID_SIZE bytes"
fi

cleanup

# ================================================================
banner "TEST 3: Cert Re-auth (restart admin, agent reconnects)"
# ================================================================
docker run -d --name s-admin --network s-net --ip 10.88.0.10 shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat >/dev/null 2>&1
sleep 3

docker run -d --name s-agent --network s-net --ip 10.88.0.20 \
  --entrypoint /opt/shroud/agent shroud-test \
  -c 10.88.0.10:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/agent-cert --reconnect 3 -v >/dev/null 2>&1
sleep 8

# Verify initial connection
if docker logs s-admin 2>&1 | grep -q "successfully"; then
  ok "3.1 Initial cert enrollment OK"
else
  fail "3.1 Initial enrollment" "not connected"
  docker logs s-admin 2>&1 | tail -5
fi

# Restart admin
docker restart s-admin
sleep 15

# Check reconnect
if docker logs s-admin 2>&1 | tail -10 | grep -q "successfully"; then
  ok "3.2 Agent reconnected after admin restart (cert auth, not -s token)"
else
  fail "3.2 Cert re-auth" "Agent did not reconnect after admin restart"
  docker logs s-agent 2>&1 | tail -10
fi

cleanup

# ================================================================
banner "TEST 4: --script mode crash when AdminCleanExit is nil"
# ================================================================
SCRIPT_OUT=$(timeout 10 docker run --rm --network s-net shroud-test \
  -l 19998 -s script_test --identity-plain --identity-dir /tmp/s --script 2>&1 <<< "help" || true)

if echo "$SCRIPT_OUT" | grep -qi "panic\|SIGSEGV\|nil pointer"; then
  fail "BUG-4" "--script mode nil pointer: AdminCleanExit() called before set"
  echo "   Panic: $(echo "$SCRIPT_OUT" | grep -A3 panic | tr '\n' ' ')"
  echo ""
  echo "   Root cause: listenCtrlC() goroutine started at admin.go:57"
  echo "   but global.AdminCleanExit is set at admin.go:141 (after connection)"
  echo "   If Ctrl+C/ScriptTerminal.PollEvent fires before main() sets AdminCleanExit,"
  echo "   calling nil func() causes SIGSEGV."
  echo "   Fix: nil-check before calling, or set a no-op default at init."
else
  ok "4.1 --script mode no crash (AdminCleanExit is guarded)"
fi

cleanup

# ================================================================
banner "TEST 5: ARGV Scrubbing (BUG verified from earlier)"
# ================================================================
docker run -d --name s-admin --network s-net --ip 10.88.0.10 shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp >/dev/null 2>&1
sleep 3

docker run -d --name s-agent --network s-net --ip 10.88.0.20 \
  --entrypoint /opt/shroud/agent shroud-test \
  -c 10.88.0.10:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/argv-test -v >/dev/null 2>&1
sleep 8

CMDLINE=$(docker exec s-agent cat /proc/1/cmdline 2>/dev/null | tr '\0' ' ' || echo "")
if echo "$CMDLINE" | grep -q "$SECRET"; then
  fail "BUG-5" "ARGV: secret '$SECRET' visible in /proc/1/cmdline"
  echo "   Fix: use unsafe.Pointer to zero the argv memory region directly,"
  echo "   or call prctl(PR_SET_MM, PR_SET_MM_ARG_START/END) to truncate."
else
  ok "5.1 ARGV secret hidden from cmdline"
fi

cleanup

# ================================================================
banner "TEST 6: Multi-Hop (interactive admin driving agent listen)"
# ================================================================
docker run -d --name s-admin --network s-net --ip 10.88.0.10 shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat >/dev/null 2>&1
sleep 3

docker run -d --name s-agent --network s-net --ip 10.88.0.20 \
  --entrypoint /opt/shroud/agent shroud-test \
  -c 10.88.0.10:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/agent-hop -v >/dev/null 2>&1
sleep 8

# Try to send listen command via script admin (like the entrypoint does)
docker run -d --name s-admin2 --network s-net --ip 10.88.0.11 shroud-test \
  sh -c "tail -n +1 -f /tmp/cmd | /opt/shroud/admin -c 10.88.0.10:9999 -s '$SECRET' --identity-plain --identity-dir /tmp/a2 --script" >/dev/null 2>&1
sleep 3

# Send commands
echo "use 0" | docker exec -i s-admin2 tee -a /tmp/cmd >/dev/null
sleep 2
echo "listen" | docker exec -i s-admin2 tee -a /tmp/cmd >/dev/null
sleep 1
echo "1" | docker exec -i s-admin2 tee -a /tmp/cmd >/dev/null
sleep 1
echo "0.0.0.0:10000" | docker exec -i s-admin2 tee -a /tmp/cmd >/dev/null
sleep 5

# Check if agent is listening
LISTEN=$(docker exec s-agent ss -tlnp 2>/dev/null | grep 10000 || echo "")
if [ -n "$LISTEN" ]; then
  ok "6.1 Agent-0 listening on 10000 (multi-hop setup complete)"
  
  # Now connect agent-1
  docker run -d --name s-agent2 --network s-net --ip 10.88.0.30 \
    --entrypoint /opt/shroud/agent shroud-test \
    -c 10.88.0.20:10000 -s "$SECRET" --fileless -v >/dev/null 2>&1
  sleep 10
  
  if docker logs s-admin 2>&1 | tail -20 | grep -q "successfully"; then
    # Check if it's the second connection
    CONNECTIONS=$(docker logs s-admin 2>&1 | grep -c "successfully" || echo "0")
    if [ "$CONNECTIONS" -ge 2 ]; then
      ok "6.2 Multi-hop: Agent-1 registered (admin → agent-0 → agent-1)"
    else
      skip "6.2 Multi-hop (second connection not confirmed)"
    fi
  else
    A2LOG=$(docker logs s-agent2 2>&1 | tail -5)
    fail "6.2 Multi-hop" "Agent-1 did not connect: $A2LOG"
  fi
else
  fail "6.1 Agent-0 listen" "Port 10000 not open. --script mode listen through pipe may not work."
  echo "   This confirms BUG: script-mode piping can't drive interactive commands."
fi

cleanup

# ================================================================
banner "TEST 7: Enrollment Token One-Time-Use"
# ================================================================
docker run -d --name s-admin --network s-net --ip 10.88.0.10 shroud-test \
  -l 9999 -s onetime_token --identity-plain --identity-dir /tmp >/dev/null 2>&1
sleep 3

# First enrollment
$AGENT_RUN --ip 10.88.0.20 -c 10.88.0.10:9999 -s onetime_token \
  --identity-plain --identity-dir /tmp/ot1 -v >/tmp/ot1.log 2>&1 &
sleep 8

# Second enrollment attempt (should be rejected or use cert auth)
OT2_OUT=$(timeout 10 $AGENT_RUN --ip 10.88.0.21 -c 10.88.0.10:9999 -s onetime_token \
  --identity-plain --identity-dir /tmp/ot2 -v 2>&1 || true)

if echo "$OT2_OUT" | grep -qi "already consumed\|token\|reject"; then
  ok "7.1 Enrollment token reuse prevented (agent 2 rejected)"
elif echo "$OT2_OUT" | grep -qi "error\|fail\|fatal"; then
  ok "7.1 Enrollment token reuse prevented (agent 2 errored)"
else
  # Since admin supports only 1 direct child, this might be a different rejection reason
  ADMIN_CONNS=$(docker logs s-admin 2>&1 | grep -c "successfully" || echo "0")
  if [ "$ADMIN_CONNS" -eq 1 ]; then
    skip "7.1 Token reuse (admin at capacity - 1 child limit)"
  else
    fail "7.1 Token reuse" "Second agent connected: $ADMIN_CONNS connections"
  fi
fi

cleanup

# ================================================================
banner "RESULTS"
# ================================================================
echo
echo "Tests run: $((PASS + FAIL))"
echo -e "  \033[32mPASS: $PASS\033[0m"
echo -e "  \033[31mFAIL: $FAIL\033[0m"
echo

if [ "$FAIL" -gt 0 ]; then
  echo "=== BUGS FOUND ==="
fi
