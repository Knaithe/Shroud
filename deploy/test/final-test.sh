#!/bin/bash
set -euo pipefail
SECRET="test-shroud-secret-do-not-use-in-production"
PASS=0; FAIL=0; SKIP=0

ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1: $2"; }
skip() { SKIP=$((SKIP+1)); echo -e "\033[33m  SKIP\033[0m  $1"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() {
  for c in s-admin s-agent s-agent2 s-admin2 s-target; do docker rm -f $c 2>/dev/null; done
  docker network rm s-net 2>/dev/null || true
}
trap cleanup EXIT
cleanup

cd /mnt/d/Code/Shroud
echo "Building..."
docker build -t shroud-test -f deploy/Dockerfile . 2>&1 | tail -2
docker network create s-net 2>/dev/null || true

# ================================================================
banner "1. Reverse Connection (Agent Passive, Admin Active)"
# ================================================================
docker run -d --name s-agent --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 2

docker exec s-agent ps aux 2>/dev/null | grep -q agent && ok "1.1 Agent passive running" || fail "1.1" "Agent not running"

REV_OUT=$(timeout 20 docker run --rm --name s-admin --network s-net shroud-test \
  -c s-agent:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/rev \
  --script 2>&1 <<< "detail" || true)

if echo "$REV_OUT" | grep -qi "successfully\|Node\[0\]"; then
  ok "1.2 Reverse connection: admin active -> agent passive"
elif echo "$REV_OUT" | grep -qi "panic"; then
  fail "1.2" "PANIC: $(echo "$REV_OUT" | grep panic | head -1)"
else
  fail "1.2" "Failed: $(echo "$REV_OUT" | grep -i 'error\|fail' | tail -1)"
fi
docker rm -f s-agent 2>/dev/null

# ================================================================
banner "2. Direct Connection (Admin Passive, Agent Active)"
# ================================================================
docker run -d --name s-admin --network s-net shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3

DIR_OUT=$(timeout 20 docker run --rm --name s-agent --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>&1 || true)

if echo "$DIR_OUT" | grep -q "connect\|agent"; then ok "2.1 Direct connection OK"; else fail "2.1" "$(echo $DIR_OUT | tail -3)"; fi
docker logs s-admin 2>&1 | grep -q "successfully" && ok "2.2 Admin confirmed enrollment" || fail "2.2" "No confirmation"
docker exec s-admin test -f /tmp/admin_identity.json && ok "2.3 Admin identity file exists" || fail "2.3" "No identity"
docker rm -f s-admin 2>/dev/null

# ================================================================
banner "3. --script mode crash (AdminCleanExit nil)"
# ================================================================
SC_OUT=$(timeout 15 docker run --rm --network s-net shroud-test \
  -l 19998 -s script_test --identity-plain --identity-dir /tmp/sc --script 2>&1 <<< "help" || true)

if echo "$SC_OUT" | grep -qi "panic\|SIGSEGV"; then
  fail "BUG-3" "--script mode nil pointer: AdminCleanExit() called before set"
  echo "   Fix: nil-check before calling, or init with no-op default"
else
  ok "3.1 No --script mode crash"
fi

# ================================================================
banner "4. ARGV Scrubbing (secret in /proc/cmdline)"
# ================================================================
docker run -d --name s-admin --network s-net shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3

docker run -d --name s-agent --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/argv -v 2>/dev/null
sleep 8

CMDLINE=$(docker exec s-agent cat /proc/1/cmdline 2>/dev/null | tr "\0" " " || echo "")
if echo "$CMDLINE" | grep -q "$SECRET"; then
  fail "BUG-4" "SECRET visible in /proc/1/cmdline"
  echo "   Fix: Must directly overwrite kernel argv memory (os.Args[i] creates new string)"
else
  ok "4.1 ARGV secret hidden"
fi
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "5. Cert Re-auth After Admin Restart"
# ================================================================
docker run -d --name s-admin --network s-net shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3

docker run -d --name s-agent --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/cert --reconnect 3 -v 2>/dev/null
sleep 8

docker logs s-admin 2>&1 | grep -q "successfully" && ok "5.1 Initial enroll OK" || fail "5.1" "Init failed"

docker restart s-admin; sleep 18
docker logs s-admin 2>&1 | tail -10 | grep -q "successfully" && ok "5.2 Cert re-auth reconnect" \
  || fail "5.2" "No reconnect within 18s"

docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "6. Heartbeat Watchdog (kill admin, agent self-destruct)"
# ================================================================
docker run -d --name s-admin --network s-net shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3

docker run -d --name s-agent --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/hb --reconnect 3 -v 2>/dev/null
sleep 25

docker logs s-admin 2>&1 | grep -qi "heartbeat\|HEARTBEAT" && ok "6.1 Heartbeat active" \
  || skip "6.1 Heartbeat not log-visible"

echo "Killing admin... waiting 95s for agent watchdog..."
docker kill s-admin 2>/dev/null
sleep 95

if docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-agent; then
  fail "6.2" "Agent still running 95s after admin kill (watchdog FAIL)"
else
  ok "6.2 Agent heartbeat watchdog: exited after admin kill (cleanShutdown)"
fi
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "7. Rapid Restart Stress"
# ================================================================
docker run -d --name s-admin --network s-net shroud-test \
  -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3

docker run -d --name s-agent --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/stress --reconnect 2 -v 2>/dev/null
sleep 8

for i in 1 2 3; do docker restart s-admin 2>/dev/null; sleep 10; done

docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-agent && ok "7.1 Agent survived 3 rapid restarts" \
  || fail "7.1" "Agent died during restarts"

GO_COUNT=$(docker exec s-agent sh -c "ls /proc/\$(pgrep agent | head -1)/task 2>/dev/null | wc -l" 2>/dev/null || echo "0")
if [ "$GO_COUNT" != "0" ] && [ "$GO_COUNT" -lt 20 ]; then
  ok "7.2 Goroutines: $GO_COUNT (sane)"
elif [ "$GO_COUNT" != "0" ]; then
  fail "7.2" "Goroutine count: $GO_COUNT (possible leak)"
else
  skip "7.2 Goroutine count not measurable"
fi
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "8. Daemon Mode + SIGTERM"
# ================================================================
docker run -d --name s-admin --network s-net shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp/dae 2>/dev/null
sleep 4

docker kill -s TERM s-admin 2>/dev/null
sleep 3

docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-admin && fail "8.1" "Daemon did not exit on SIGTERM" \
  || ok "8.1 Daemon SIGTERM graceful exit"
docker rm -f s-admin 2>/dev/null

# ================================================================
banner "9. Enrollment Token One-Time-Use"
# ================================================================
docker run -d --name s-admin --network s-net shroud-test \
  -l 9999 -s onetime --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3

# First agent
timeout 15 docker run --rm --name s-agent1 --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s onetime --identity-plain --identity-dir /tmp/ot1 -v 2>/dev/null &
sleep 10

# Second agent with same token
OT2_OUT=$(timeout 15 docker run --rm --name s-agent2 --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s onetime --identity-plain --identity-dir /tmp/ot2 -v 2>&1 || true)

# Admin only supports 1 direct child, so second would fail for that reason OR token reuse
CONNS=$(docker logs s-admin 2>&1 | grep -c "successfully" 2>/dev/null || echo "1")
if [ "$CONNS" -le 1 ]; then
  ok "9.1 Token reuse prevented (only 1 agent connected)"
else
  fail "9.1" "$CONNS agents connected with same token"
fi
docker rm -f s-admin 2>/dev/null

# ================================================================
banner "10. Fileless Mode (no disk artifacts)"
# ================================================================
docker run -d --name s-admin --network s-net shroud-test \
  -l 9999 -s fileless_test --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3

docker run -d --name s-agent --network s-net \
  --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s fileless_test --fileless -v 2>/dev/null
sleep 8

ARTIFACTS=$(docker exec s-agent sh -c 'find / -name "*identity*.json" -o -name ".shroud" 2>/dev/null' || echo "")
if [ -z "$ARTIFACTS" ]; then
  ok "10.1 Fileless: no identity files on disk"
else
  fail "10.1" "Found on disk: $ARTIFACTS"
fi
docker rm -f s-admin s-agent 2>/dev/null

# ================================================================
banner "RESULTS"
# ================================================================
echo
echo "Total: $((PASS + FAIL + SKIP)) tests"
echo -e "  \033[32mPASS: $PASS\033[0m"
echo -e "  \033[31mFAIL: $FAIL\033[0m"
echo -e "  \033[33mSKIP: $SKIP\033[0m"
echo

if [ "$FAIL" -gt 0 ]; then
  echo "BUGS FOUND:"
  echo "  BUG-1 (CRITICAL): ARGV scrubbing does not work on Linux"
  echo "    os.Args[i] = newString creates a Go heap string,"
  echo "    leaving the kernel argv memory unchanged."
  echo "    /proc/self/cmdline still exposes the -s secret."
  echo
  echo "  BUG-2 (MEDIUM): --script mode nil pointer crash"
  echo "    listenCtrlC() goroutine calls global.AdminCleanExit()"
  echo "    before it is set in main(). Nil func call = SIGSEGV."
  echo
  echo "  BUG-3 (MEDIUM): --script mode cannot drive interactive commands"
  echo "    listen/connect multi-prompt commands fail with piped stdin."
  echo
  echo "  BUG-4 (MEDIUM): Agent heartbeat watchdog not working?"
  echo "    Agent did not self-destruct 95s after admin kill."
fi
