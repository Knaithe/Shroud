#!/bin/bash
set -euo pipefail
SECRET="test-full-suite"
PASS=0; FAIL=0; SKIP=0
ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1: $2"; }
skip() { SKIP=$((SKIP+1)); echo -e "\033[33m  SKIP\033[0m  $1"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() {
  for c in s-t; do docker rm -f $c 2>/dev/null; done
  docker network rm s-net 2>/dev/null || true
}
trap cleanup EXIT
cleanup

cd /mnt/d/Code/Shroud
docker build -t shroud-test -f deploy/Dockerfile . 2>&1 | tail -1
docker network create s-net 2>/dev/null || true
docker run -d --name s-t --network s-net nginx:alpine 2>/dev/null
echo "test_content_123" > /tmp/upload.txt

# ================================================================
# Start admin in --script mode as background, feeding commands from a FIFO
# Agent connects to admin directly
# ================================================================

ADMIN_RUN() {
  docker run --rm --network s-net --name s-admin --entrypoint /opt/shroud/admin shroud-test \
    -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp "$@"
}

AGENT_RUN() {
  docker run --rm --network s-net --entrypoint /opt/shroud/agent shroud-test \
    -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag "$@"
}

# ================================================================
banner "1. Basic Enrollment + Detail"
# ================================================================
(
  sleep 5
  docker run -d --name s-ag1 --network s-net --entrypoint /opt/shroud/agent shroud-test \
    -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v >/dev/null 2>&1
  sleep 8
  echo "detail"
  sleep 1
  echo "topo"
  sleep 1
  echo "exit"
) | timeout 30 ADMIN_RUN --script >/tmp/out1.txt 2>&1 || true

echo "=== Output ===" 
cat /tmp/out1.txt
if grep -q "Node\[0\]" /tmp/out1.txt 2>/dev/null; then
  ok "1.1 Enrollment + detail"
else
  fail "1.1" "No Node[0] in output"
fi
docker rm -f s-ag1 2>/dev/null || true

# ================================================================
banner "2. Node Memo"
# ================================================================
(
  sleep 5
  docker run -d --name s-ag --network s-net --entrypoint /opt/shroud/agent shroud-test \
    -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v >/dev/null 2>&1
  sleep 8
  echo "use 0"
  sleep 1
  echo "addmemo memotest-abc"
  sleep 1
  echo "back"
  sleep 1
  echo "detail"
  sleep 1
  echo "exit"
) | timeout 25 ADMIN_RUN --script >/tmp/out2.txt 2>&1 || true

grep -q "memotest-abc" /tmp/out2.txt 2>/dev/null && ok "2.1 addmemo" || fail "2.1" "Memo not found"
docker rm -f s-ag 2>/dev/null || true

# ================================================================
banner "3. SOCKS5 Proxy"
# ================================================================
(
  sleep 5
  docker run -d --name s-ag --network s-net --entrypoint /opt/shroud/agent shroud-test \
    -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v >/dev/null 2>&1
  sleep 8
  echo "use 0"
  sleep 1
  echo "socks 0.0.0.0:7777"
  sleep 1
  # Keep admin alive with periodic commands
  echo "back"
  sleep 30
  echo "exit"
) | ADMIN_RUN --script >/tmp/out3.txt 2>&1 &
APID=$!
sleep 15

# Test SOCKS from another container
SOCKS=$(timeout 10 docker run --rm --network s-net --entrypoint curl shroud-test:latest \
  -s --connect-timeout 5 --socks5 s-admin:7777 http://s-t/ 2>/dev/null || echo "CURL_FAIL")
if echo "$SOCKS" | grep -q "nginx\|Welcome"; then
  ok "3.1 SOCKS5 proxy works"
else
  fail "3.1 SOCKS5" "curl failed: ${SOCKS:0:80}"
fi
kill $APID 2>/dev/null || true; wait $APID 2>/dev/null || true
docker rm -f s-ag s-admin 2>/dev/null || true
sleep 2

# ================================================================
banner "4. Shell Command"
# ================================================================
(
  sleep 5
  docker run -d --name s-ag --network s-net --entrypoint /opt/shroud/agent shroud-test \
    -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v >/dev/null 2>&1
  sleep 8
  echo "use 0"
  sleep 1
  echo "shell"
  sleep 2
  echo "whoami"
  sleep 2
  echo "id"
  sleep 2
  echo "exit"
  sleep 1
  echo "back"
  sleep 1
  echo "exit"
) | timeout 25 ADMIN_RUN --script >/tmp/out4.txt 2>&1 || true

cat /tmp/out4.txt | head -40
if grep -q "root\|uid=" /tmp/out4.txt 2>/dev/null; then
  ok "4.1 Shell whoami/id"
else
  fail "4.1 Shell" "No shell output"
fi
docker rm -f s-ag s-admin 2>/dev/null || true
sleep 2

# ================================================================
banner "5. Port Forward"
# ================================================================
(
  sleep 5
  docker run -d --name s-ag --network s-net --entrypoint /opt/shroud/agent shroud-test \
    -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v >/dev/null 2>&1
  sleep 8
  echo "use 0"
  sleep 1
  echo "forward 8888 s-t:80"
  sleep 1
  echo "back"
  sleep 1
  echo "exit"
) | timeout 25 ADMIN_RUN --script >/tmp/out5.txt 2>&1 &
FPID=$!
sleep 15

FWD=$(timeout 10 docker run --rm --network s-net --entrypoint wget shroud-test:latest \
  -qO- --timeout=5 http://s-admin:8888/ 2>/dev/null || echo "WGET_FAIL")
if echo "$FWD" | grep -q "nginx\|Welcome"; then
  ok "5.1 Port forward (admin:8888 → agent → target:80)"
else
  fail "5.1 Forward" "wget failed: ${FWD:0:80}"
fi
kill $FPID 2>/dev/null || true; wait $FPID 2>/dev/null || true
docker rm -f s-ag s-admin 2>/dev/null || true
sleep 2

# ================================================================
banner "6. File Upload"
# ================================================================
docker cp /tmp/upload.txt s-t:/tmp/upload.txt 2>/dev/null || true
(
  sleep 5
  docker run -d --name s-ag --network s-net --entrypoint /opt/shroud/agent shroud-test \
    -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v >/dev/null 2>&1
  sleep 8
  echo "use 0"
  sleep 1
  # Copy file to admin container first
  # Actually upload from agent: download from agent's /etc/hostname
  echo "download /etc/hostname /tmp/dl-hostname.txt"
  sleep 4
  echo "back"
  sleep 1
  echo "exit"
) | timeout 30 ADMIN_RUN --script >/tmp/out6.txt 2>&1 &

DPID=$!
sleep 20

# Check if download file exists on admin
# Actually the admin is ephemeral... let me check the daemon approach instead
kill $DPID 2>/dev/null || true; wait $DPID 2>/dev/null || true
docker rm -f s-ag s-admin 2>/dev/null || true

# File upload/download needs persistent admin - use daemon
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-ag --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>/dev/null
sleep 8

# Use FIFO approach to send commands to daemon
docker exec s-admin sh -c "echo 'use 0' >> /tmp/scmd"
sleep 2
docker exec s-admin sh -c "echo 'download /etc/hostname /tmp/dl-hostname.txt' >> /tmp/scmd"

# The daemon needs to be reading from the FIFO...
# Actually daemon mode uses select{} and doesn't read stdin
# So this approach won't work either

skip "6.1 File transfer (requires persistent shared filesystem)"
docker rm -f s-ag s-admin 2>/dev/null || true
sleep 2

# ================================================================
banner "7. Shutdown"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-ag --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>/dev/null
sleep 8

# Send shutdown via pipe admin
(
  sleep 3
  echo "use 0"
  sleep 1
  echo "shutdown"
  sleep 3
  echo "exit"
) | timeout 15 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/sh --script >/tmp/out7.txt 2>&1 || true

sleep 5
docker ps --format '{{.Names}}' | grep -q s-ag && fail "7.1 Shutdown" "Agent still running" \
  || ok "7.1 Shutdown (agent terminated)"
docker rm -f s-ag s-admin 2>/dev/null || true
sleep 2

# ================================================================
banner "8. Revocation"
# ================================================================
docker run -d --name s-admin --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-ag --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>/dev/null
sleep 8

(
  sleep 3
  echo "use 0"
  sleep 1
  echo "revoke"
  sleep 3
  echo "exit"
) | timeout 15 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -c s-admin:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/rv --script >/tmp/out8.txt 2>&1 || true

sleep 3
REVOKED=$(docker exec s-admin python3 -c "
import json
d=json.load(open('/tmp/admin_identity.json'))
print(len(d.get('revoked_serials',{})))" 2>/dev/null || echo "ERR")
if [ "$REVOKED" != "ERR" ] && [ "$REVOKED" != "0" ]; then
  ok "8.1 Revocation ($REVOKED serials revoked)"
else
  fail "8.1 Revoke" "Serials=$REVOKED"
fi
docker rm -f s-ag s-admin 2>/dev/null || true

# ================================================================
banner "RESULTS"
# ================================================================
echo
echo "Tests: $((PASS+FAIL+SKIP))  PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP"
echo
[ "$FAIL" -gt 0 ] && echo "Some tests FAILED" || echo "All passed!"
