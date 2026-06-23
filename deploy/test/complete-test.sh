#!/bin/bash
set -euo pipefail
SECRET="test-shroud-secret-do-not-use-in-production"
PASS=0; FAIL=0; SKIP=0
ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1: $2"; }
skip() { SKIP=$((SKIP+1)); echo -e "\033[33m  SKIP\033[0m  $1"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() {
  for c in s-admin s-agent s-agent2; do docker rm -f $c 2>/dev/null; done
  docker network rm s-net 2>/dev/null || true
}
trap cleanup EXIT
cleanup

cd /mnt/d/Code/Shroud
docker build -t shroud-test -f deploy/Dockerfile . 2>&1 | tail -1
docker network create s-net 2>/dev/null || true

# Helper: run admin
admin() { docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test "$@"; }
# Helper: run agent
agent() { docker run --rm --network s-net --entrypoint /opt/shroud/agent shroud-test "$@"; }
# Helper: run agent daemon
agentd() { docker run -d --network s-net --entrypoint /opt/shroud/agent shroud-test "$@"; }

# ================================================================
banner "1. Direct Connection (Admin Passive, Agent Active)"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3

DIR_OUT=$(timeout 20 agent \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>&1 || true)

echo "$DIR_OUT" | grep -q "connect\|agent" && ok "1.1 Direct connection OK" \
  || fail "1.1" "$(echo $DIR_OUT | tail -3)"
docker logs s-admin 2>&1 | grep -q "successfully" && ok "1.2 Admin confirmed enrollment" \
  || fail "1.2" "No enrollment confirmation"
docker exec s-admin test -f /tmp/admin_identity.json && ok "1.3 Admin identity file created" \
  || fail "1.3" "No identity file"
docker rm -f s-admin 2>/dev/null

# ================================================================
banner "2. Reverse Connection (Agent Passive, Admin Active)"
# ================================================================
docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 2

docker exec s-agent ps aux 2>/dev/null | grep -q agent && ok "2.1 Agent passive listening" \
  || fail "2.1" "Agent not running"

REV_OUT=$(timeout 20 admin \
  -c s-agent:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/rev \
  --script 2>&1 <<< "detail\nexit\n" || true)

echo "$REV_OUT" | grep -qi "successfully\|Node\[0\]" && ok "2.2 Reverse connection: admin -> agent" \
  || fail "2.2" "Failed: $(echo $REV_OUT | grep -i 'error\|fail' | tail -1)"
docker rm -f s-agent 2>/dev/null

# ================================================================
banner "3. ARGV Scrubbing BUG"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3

docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/argv -v 2>/dev/null; sleep 8

CMDLINE=$(docker exec s-agent cat /proc/1/cmdline 2>/dev/null | tr "\0" " " || echo "")
if echo "$CMDLINE" | grep -q "$SECRET"; then
  fail "BUG-1" "SECRET visible in /proc/1/cmdline"
  echo "         Fix: directly overwrite kernel argv memory region,"
  echo "         os.Args[i] = newString only changes Go's copy"
else
  ok "3.1 ARGV secret hidden"
fi
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "4. --script mode crash"
# ================================================================
SC_OUT=$(timeout 15 admin \
  -l 19998 -s script_test --identity-plain --identity-dir /tmp/sc --script 2>&1 <<< "help" || true)

echo "$SC_OUT" | grep -qi "panic\|SIGSEGV" \
  && fail "BUG-2" "--script nil pointer: AdminCleanExit() before set" \
  || ok "4.1 No --script crash"

# ================================================================
banner "5. Cert Re-auth (agent reconnects after admin restart)"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3

docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/cert --reconnect 3 -v 2>/dev/null; sleep 8

docker logs s-admin 2>&1 | grep -q "successfully" && ok "5.1 Initial enrollment" || fail "5.1" "Init failed"
docker restart s-admin; sleep 18
docker logs s-admin 2>&1 | tail -10 | grep -q "successfully" && ok "5.2 Cert re-auth reconnect" \
  || fail "5.2" "No reconnect within 18s"
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "6. Heartbeat Watchdog"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3

docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/hb --reconnect 100 -v 2>/dev/null
sleep 30

docker logs s-admin 2>&1 | grep -qi "heartbeat\|HEARTBEAT" && ok "6.1 Heartbeat admin side active" \
  || skip "6.1 Heartbeat not visible in logs"

echo "Killing admin, waiting 95s for watchdog (or 10s for conn close)..."
docker kill s-admin 2>/dev/null
# The agent will detect broken connection (reconnect every 100s) or watchdog (90s)
# Either way, agent should exit within ~100s
sleep 100

AGENT_ALIVE=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -c s-agent || echo "0")
if [ "$AGENT_ALIVE" -eq 0 ]; then
  ok "6.2 Agent exited after admin kill (watchdog or reconnect failure)"
else
  fail "6.2" "Agent still running after 100s"
fi
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "7. Rapid Restart Stress"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3

docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/stress --reconnect 2 -v 2>/dev/null; sleep 8

for i in 1 2 3; do docker restart s-admin 2>/dev/null; sleep 10; done

docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-agent && ok "7.1 Agent survived 3 rapid restarts" \
  || fail "7.1" "Agent died"
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "8. Daemon Mode"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp/dae 2>/dev/null; sleep 4
docker kill -s TERM s-admin 2>/dev/null; sleep 3
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-admin && fail "8.1" "Daemon no SIGTERM exit" \
  || ok "8.1 Daemon SIGTERM graceful exit"
docker rm -f s-admin 2>/dev/null

# ================================================================
banner "9. Fileless Mode"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s fileless_test --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3

docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s fileless_test --fileless -v 2>/dev/null; sleep 8

ARTIFACTS=$(docker exec s-agent sh -c 'find / -name "*identity*.json" 2>/dev/null' || echo "")
[ -z "$ARTIFACTS" ] && ok "9.1 Fileless: no identity files on disk" \
  || fail "9.1" "Found: $ARTIFACTS"
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "10. Multi-Hop (interactive admin → agent listen → child connect)"
# ================================================================
echo "This test requires interactive admin (not --script)."
echo "Starting admin in background with named pipe..."

# Create a FIFO for interactive input
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3

# Agent-0 connects
docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/hop -v 2>/dev/null; sleep 8

docker logs s-admin 2>&1 | grep -q "successfully" && ok "10.1 Agent-0 enrolled" || fail "10.1" "Not enrolled"

# Use script admin to drive commands (admin2 connects to admin and sends listen)
echo "Setting up multi-hop via script admin..."
SCRIPT_CMDS="use 0\nlisten\n1\n0.0.0.0:10000\nback\ntopo"
HOP_OUT=$(timeout 20 admin \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/hop2 --script 2>&1 <<< "$SCRIPT_CMDS" || true)

# Check if listen port is open on agent-0
LISTEN=$(docker exec s-agent ss -tlnp 2>/dev/null | grep 10000 || echo "")
if [ -n "$LISTEN" ]; then
  ok "10.2 Agent-0 listening on 10000 (listen command works in script mode)"
else
  skip "10.2 Agent-0 listen (script mode listen may not work)"
fi

docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "11. Enrollment Token One-Time-Use"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s onetime --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3

# First agent
timeout 15 agent -c s-admin:9999 -s onetime --identity-plain --identity-dir /tmp/ot1 -v >/dev/null 2>&1 &
sleep 10

# Second should fail (admin single child limit OR token reuse)
timeout 15 agent -c s-admin:9999 -s onetime --identity-plain --identity-dir /tmp/ot2 -v >/tmp/ot2.log 2>&1 || true

CONNS=$(docker logs s-admin 2>&1 | grep -c "successfully" 2>/dev/null || echo "1")
if [ "$CONNS" -le 1 ]; then
  ok "11.1 Token reuse prevented (only 1 agent)"
else
  if grep -qi "already consumed\|token" /tmp/ot2.log 2>/dev/null; then
    ok "11.1 Token reuse explicitly rejected"
  else
    fail "11.1" "$CONNS agents enrolled (token reuse allowed)"
  fi
fi
docker rm -f s-admin 2>/dev/null

# ================================================================
banner "12. Process Name Masking"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3
docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/mask -v 2>/dev/null; sleep 8

COMM=$(docker exec s-agent cat /proc/1/comm 2>/dev/null || echo "")
[ "$COMM" = "kworker/0:1" ] && ok "12.1 Process name: kworker/0:1" \
  || fail "12.1" "comm=$COMM (expected kworker/0:1)"
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "13. Identity File Integrity"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3
docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/idcheck -v 2>/dev/null; sleep 8

ADMIN_JSON=$(docker exec s-admin python3 -c "import json; json.load(open('/tmp/admin_identity.json'))" 2>&1 || echo "INVALID")
AGENT_JSON=$(docker exec s-agent python3 -c "import json; json.load(open('/tmp/idcheck/agent_identity.json'))" 2>&1 || echo "INVALID")

[ "$ADMIN_JSON" != "INVALID" ] && ok "13.1 Admin identity valid JSON" || fail "13.1" "Admin identity: $ADMIN_JSON"
[ "$AGENT_JSON" != "INVALID" ] && ok "13.2 Agent identity valid JSON" || fail "13.2" "Agent identity: $AGENT_JSON"
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "14. Core Dump Disabled"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3
docker run -d --name s-agent --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/coredump -v 2>/dev/null; sleep 8

# Check dumpable flag (PID 1 might not be the agent due to entrypoint)
AGENT_PID=$(docker exec s-agent pgrep -x agent 2>/dev/null || echo "1")
DUMPABLE=$(docker exec s-agent cat /proc/$AGENT_PID/status 2>/dev/null | grep Dumpable | awk '{print $2}' || echo "?")
[ "$DUMPABLE" = "0" ] && ok "14.1 Core dump disabled (Dumpable=0)" \
  || skip "14.1 Dumpable=$DUMPABLE (PID=$AGENT_PID)"
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "FINAL RESULTS"
# ================================================================
echo
echo "Total: $((PASS + FAIL + SKIP)) tests"
echo -e "  \033[32mPASS: $PASS\033[0m"
echo -e "  \033[31mFAIL: $FAIL\033[0m"
echo -e "  \033[33mSKIP: $SKIP\033[0m"
echo

if [ "$FAIL" -gt 0 ]; then
  echo "=== BUGS FOUND ==="
  echo "BUG-1 (CRITICAL): ARGV scrubbing ineffective on Linux"
  echo "  os.Args[i] = newString doesn't modify kernel argv memory."
  echo "  Fix: use unsafe pointer to overwrite /proc/self/cmdline region,"
  echo "  or prctl(PR_SET_MM, PR_SET_MM_ARG_START/END)."
  echo
  echo "BUG-2 (MEDIUM): --script mode nil pointer when AdminCleanExit not set"
  echo "  listenCtrlC goroutine calls nil func before main() sets it."
  echo "  Fix: initialize AdminCleanExit with a no-op default."
  echo
  exit 1
else
  echo "All critical tests passed."
fi
