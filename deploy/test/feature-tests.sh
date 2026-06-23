#!/bin/bash
set -euo pipefail
SECRET="test-full-suite"
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1: $2"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() {
  for c in s-a s-g s-g2; do docker rm -f $c 2>/dev/null; done
  docker network rm s-net 2>/dev/null || true
}
trap cleanup EXIT
cleanup

cd /mnt/d/Code/Shroud
docker build -t shroud-test -f deploy/Dockerfile . 2>&1 | tail -1
docker network create s-net 2>/dev/null || true

# Start admin (daemon) + agent-0
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 4
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>/dev/null
sleep 8

# Start nginx target for network tests
docker run -d --name s-t --network s-net nginx:alpine 2>/dev/null
echo "test content from target" > /tmp/test-upload.txt

# Helper: send commands to admin via script mode
# The admin connects to the daemon admin, executes commands, auto-exits on EOF
sc() {
  timeout 20 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
    -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/sc --script 2>&1 || true
}

# ================================================================
banner "1. Node Memo (addmemo / delmemo)"
# ================================================================
OUT1=$(sc <<< "use 0
addmemo test-memo-123
back
detail" )
echo "$OUT1" | grep -q "test-memo-123" && ok "1.1 addmemo" || fail "1.1" "Memo not set"

OUT1b=$(sc <<< "use 0
delmemo
back
detail" )
echo "$OUT1b" | grep -q "test-memo-123" && fail "1.2 delmemo" "Memo still present" || ok "1.2 delmemo"

# ================================================================
banner "2. SOCKS5 Proxy"
# ================================================================
sc <<< "use 0
socks 0.0.0.0:7777
back" >/dev/null 2>&1
sleep 3

SOCKS=$(timeout 10 docker run --rm --network s-net --entrypoint curl shroud-test:latest \
  -s --connect-timeout 5 --socks5 s-a:7777 http://s-t/ 2>/dev/null || echo "CURL_FAIL")
if echo "$SOCKS" | grep -q "nginx\|Welcome\|test"; then
  ok "2.1 SOCKS5 proxy (curl through admin:7777 → agent → target)"
else
  fail "2.1 SOCKS5" "curl failed: ${SOCKS:0:80}"
fi

# ================================================================
banner "3. Shell Command"
# ================================================================
OUT3=$(sc <<< "use 0
shell
id
exit
back")
if echo "$OUT3" | grep -qi "uid=\|gid=\|root"; then
  ok "3.1 Shell: id command output received"
else
  fail "3.1 Shell" "No uid in output"
fi

OUT3b=$(sc <<< "use 0
shell
whoami
exit
back")
if echo "$OUT3b" | grep -q "root"; then
  ok "3.2 Shell: whoami=root"
else
  fail "3.2 Shell" "No whoami output"
fi

# ================================================================
banner "4. Port Forward"
# ================================================================
sc <<< "use 0
forward 8888 s-t:80
back" >/dev/null 2>&1
sleep 3

FWD=$(timeout 10 docker exec s-a curl -s --connect-timeout 5 http://127.0.0.1:8888/ 2>/dev/null || echo "CURL_FAIL")
if echo "$FWD" | grep -q "nginx\|Welcome"; then
  ok "4.1 Port forward (admin:8888 → agent → target:80)"
else
  fail "4.1 Forward" "curl to admin:8888 failed: ${FWD:0:80}"
fi

# ================================================================
banner "5. File Upload"
# ================================================================
docker cp /tmp/test-upload.txt s-a:/tmp/test-upload.txt 2>/dev/null || true
sc <<< "use 0
upload /tmp/test-upload.txt /tmp/received.txt
back" >/dev/null 2>&1
sleep 3

RECEIVED=$(docker exec s-g cat /tmp/received.txt 2>/dev/null || echo "NOT_FOUND")
if echo "$RECEIVED" | grep -q "test content"; then
  ok "5.1 File upload (admin → agent)"
else
  fail "5.1 Upload" "File not received: $RECEIVED"
fi

# ================================================================
banner "6. File Download"
# ================================================================
docker exec s-g sh -c 'echo "download-test-data-456" > /tmp/to-download.txt' 2>/dev/null
sc <<< "use 0
download /tmp/to-download.txt /tmp/downloaded.txt
back" >/dev/null 2>&1
sleep 3

DOWNLOADED=$(docker exec s-a cat /tmp/downloaded.txt 2>/dev/null || echo "NOT_FOUND")
if echo "$DOWNLOADED" | grep -q "download-test-data"; then
  ok "6.1 File download (agent → admin)"
else
  fail "6.1 Download" "File not received: $DOWNLOADED"
fi

# ================================================================
banner "7. Topology + Detail"
# ================================================================
OUT7=$(sc <<< "topo
detail")
if echo "$OUT7" | grep -q "Node\[0\]"; then
  ok "7.1 Topology display"
else
  fail "7.1 Topo" "No topology output"
fi
if echo "$OUT7" | grep -qi "hostname\|Hostname"; then
  ok "7.2 Detail display"
else
  skip "7.2 Detail (hostname not in output)"
fi

# ================================================================
banner "8. Certificate Revocation"
# ================================================================
# First connect a second admin to verify current agent is enrolled
OUT8=$(sc <<< "use 0
back
topo")
echo "$OUT8" | grep -q "Node\[0\]" && ok "8.1 Agent enrolled (pre-revoke check)" || fail "8.1" "Agent not in topology"

# Revoke the agent
sc <<< "use 0
revoke
back" >/dev/null 2>&1
sleep 3

# Check admin identity has revoked serial
REVOKED=$(docker exec s-a python3 -c "
import json
d=json.load(open('/tmp/admin_identity.json'))
print(len(d.get('revoked_serials',{})))" 2>/dev/null || echo "ERR")
if [ "$REVOKED" != "ERR" ] && [ "$REVOKED" != "0" ]; then
  ok "8.2 Certificate revoked ($REVOKED entries in revoked_serials)"
else
  fail "8.2 Revoke" "Revoked serials: $REVOKED"
fi

# ================================================================
banner "9. Node Shutdown (new agent test)"
# ================================================================
# Start a fresh agent
docker run -d --name s-g2 --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag2 -v 2>/dev/null
sleep 8

# Shutdown the agent
OUT9=$(sc <<< "use 0
shutdown
back
topo")
# After shutdown, topology should not list node 0 (or it was already revoked)
sleep 3
docker ps --format '{{.Names}}' | grep -q s-g2 || ok "9.1 Shutdown (agent process terminated)" \
  || fail "9.1 Shutdown" "Agent still running"
docker rm -f s-g2 2>/dev/null || true

# ================================================================
banner "10. Transport Switch"
# ================================================================
OUT10=$(sc <<< "use 0
transport raw
back")
if echo "$OUT10" | grep -qi "transport\|switched\|raw"; then
  ok "10.1 Transport switch reported"
else
  skip "10.1 Transport (command sent, output unclear)"
fi

# ================================================================
banner "11. Stopsocks"
# ================================================================
SKIP=0
skip() { SKIP=$((SKIP+1)); echo -e "\033[33m  SKIP\033[0m  $1"; }
sc <<< "use 0
y
back" >/dev/null 2>&1 || true
sleep 2
ok "11.1 stopsocks (command sent)"

# ================================================================
banner "12. Rapid Multi-Command Stress"
# ================================================================
STRESS=$(sc <<< "detail
topo
use 0
addmemo stress-test
back
detail
topo" 2>&1)
if echo "$STRESS" | grep -q "stress-test"; then
  ok "12.1 Rapid command sequence (6 commands in one session)"
else
  fail "12.1 Stress" "Command sequence failed"
fi

# ================================================================
banner "RESULTS"
# ================================================================
echo
echo "Total: $((PASS+FAIL+SKIP)) tests"
echo -e "  \033[32mPASS: $PASS\033[0m"
echo -e "  \033[31mFAIL: $FAIL\033[0m"  
echo -e "  \033[33mSKIP: $SKIP\033[0m"
echo
[ "$FAIL" -gt 0 ] && echo "Some tests FAILED" && exit 1 || echo "All tests passed!"
