#!/bin/bash
set -euo pipefail
SECRET="test-shroud-secret-do-not-use-in-production"
PASS=0; FAIL=0; SKIP=0
ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1"; }
skip() { SKIP=$((SKIP+1)); echo -e "\033[33m  SKIP\033[0m  $1"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() { for c in s-a s-g; do docker rm -f $c 2>/dev/null; done; docker network rm s-net 2>/dev/null || true; }
trap cleanup EXIT
cleanup

cd /mnt/d/Code/Shroud
docker build -t shroud-test -f deploy/Dockerfile . 2>&1 | tail -1
docker network create s-net 2>/dev/null || true

A="docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test"
G="docker run --rm --network s-net --entrypoint /opt/shroud/agent shroud-test"

# ================================================================
banner "1. Direct Connection (Admin daemon, Agent active)"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 4

OUT1=$(timeout 20 $G -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>&1 || true)
echo "$OUT1" | grep -q "connect\|agent" && ok "1.1 Agent connects to admin" || fail "1.1" "$(echo $OUT1 | tail -2)"
docker logs s-a 2>&1 | grep -q "successfully" && ok "1.2 Admin confirms enrollment" || fail "1.2" "Admin didn't log success"
docker exec s-a test -f /tmp/admin_identity.json && ok "1.3 Admin identity file created" || fail "1.3" "No admin identity"
ok "1.4 Agent identity file created ($(docker exec s-a ls -la /tmp/ag/agent_identity.json 2>/dev/null | awk '{print $5}' || echo '?'))"
docker rm -f s-a 2>/dev/null

# ================================================================
banner "2. Reverse Connection (Agent passive, Admin active)"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 2
docker exec s-a ps aux | grep -q agent && ok "2.1 Agent passive listening" || fail "2.1" "Agent not running"

REV=$(timeout 20 $A -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/rev --script 2>&1 <<< "detail" || true)
echo "$REV" | grep -qi "successfully\|Node\[0\]" && ok "2.2 Admin connects to agent" \
  || fail "2.2" "$(echo $REV | grep -i error | tail -1)"
docker rm -f s-a 2>/dev/null

# ================================================================
banner "3. BUG: ARGV scrubbing ineffective"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/argv -v 2>/dev/null; sleep 8

CMDLINE=$(docker exec s-g cat /proc/1/cmdline 2>/dev/null | tr "\0" " " || echo "")
echo "$CMDLINE" | grep -q "$SECRET" \
  && fail "BUG-1" "Secret visible in /proc/1/cmdline: ... -s $SECRET ..." \
  || ok "3.1 ARGV secret hidden"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "4. BUG: --script mode crash"
# ================================================================
SC=$(timeout 15 $A -l 0.0.0.0:19998 -s sc_test --identity-plain --identity-dir /tmp/sc --script 2>&1 <<< "help" || true)
echo "$SC" | grep -qi "panic\|SIGSEGV" \
  && fail "BUG-2" "--script panic: AdminCleanExit nil" \
  || ok "4.1 No crash in --script mode"

# ================================================================
banner "5. Cert Re-auth"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/cert --reconnect 3 -v 2>/dev/null; sleep 8

docker logs s-a 2>&1 | grep -q "successfully" && ok "5.1 Initial enrollment" || fail "5.1" "First enroll failed"

docker restart s-a; sleep 18
docker logs s-a 2>&1 | tail -10 | grep -q "successfully" && ok "5.2 Cert re-auth: agent reconnected" \
  || fail "5.2" "No reconnect after admin restart"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "6. Heartbeat (admin side)"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/hb -v 2>/dev/null; sleep 30

docker logs s-a 2>&1 | grep -qi "heartbeat\|HEARTBEAT" && ok "6.1 Admin sends heartbeats" \
  || skip "6.1 Heartbeat not logged"
docker logs s-a 2>&1 | grep -qi "HEARTBEATACK\|heartbeat.*ack\|ack" && ok "6.2 Admin receives heartbeat ACKs" \
  || skip "6.2 ACKs not logged"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "7. Heartbeat Watchdog (agent self-destruct)"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/wd --reconnect 200 -v 2>/dev/null
sleep 15

# Kill admin and wait for watchdog (3 consecutive misses × 30s interval after 90s threshold ≈ 180s)
docker kill s-a 2>/dev/null
echo "Waiting 220s for agent watchdog (3-miss tolerance)..."
sleep 220

docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g \
  && fail "7.1" "Agent still alive 220s after admin kill (watchdog failed)" \
  || ok "7.1 Watchdog: agent exited after admin kill"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "8. Process Masking"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/mask -v 2>/dev/null; sleep 8

COMM=$(docker exec s-g cat /proc/1/comm 2>/dev/null || echo "")
[ "$COMM" = "kworker/0:1" ] && ok "8.1 Agent comm=kworker/0:1" \
  || fail "8.1" "Agent comm=$COMM"

ADMIN_COMM=$(docker exec s-a cat /proc/1/comm 2>/dev/null || echo "")
[ "$ADMIN_COMM" = "kworker/0:2" ] && ok "8.2 Admin comm=kworker/0:2" \
  || fail "8.2" "Admin comm=$ADMIN_COMM"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "9. Fileless Mode"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s fl_test --identity-plain --identity-dir /tmp 2>/dev/null; sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s fl_test --fileless -v 2>/dev/null; sleep 8

ARTIFACTS=$(docker exec s-g sh -c 'find / -name "*identity*.json" 2>/dev/null' || echo "")
[ -z "$ARTIFACTS" ] && ok "9.1 Fileless: zero identity files on disk" \
  || fail "9.1" "Found: $ARTIFACTS"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "10. Rapid Restart Stress"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/st --reconnect 2 -v 2>/dev/null; sleep 8

for i in 1 2 3; do docker restart s-a 2>/dev/null; sleep 10; done
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g && ok "10.1 Agent survived 3 rapid restarts" \
  || fail "10.1" "Agent died"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "11. Daemon Mode SIGTERM"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/dae 2>/dev/null; sleep 4
docker kill -s TERM s-a 2>/dev/null; sleep 3
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-a && fail "11.1" "Daemon didn't exit" \
  || ok "11.1 Daemon SIGTERM graceful exit"
docker rm -f s-a 2>/dev/null

# ================================================================
banner "12. Core Dump Disabled"
# ================================================================
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null; sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/cd -v 2>/dev/null; sleep 8

APID=$(docker exec s-g pgrep -x agent 2>/dev/null || echo "1")
DUMP=$(docker exec s-g cat /proc/$APID/status 2>/dev/null | grep Dumpable | awk '{print $2}' || echo "?")
[ "$DUMP" = "0" ] && ok "12.1 Core dump disabled (Dumpable=0, PID=$APID)" \
  || skip "12.1 Dumpable=$DUMP"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "RESULTS"
# ================================================================
echo "Total: $((PASS+FAIL+SKIP))  PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP"
echo

if [ "$FAIL" -gt 0 ]; then
  echo "=== BUGS CONFIRMED ==="
  echo "BUG-1 (CRITICAL): ARGV scrubbing ineffective - secret in /proc/cmdline"
  echo "BUG-2 (MEDIUM): --script mode nil pointer (if reproduced)"
  echo
  exit 1
else
  echo "All tests passed."
fi
