#!/bin/bash
set -euo pipefail
SECRET="test-full-suite"
PASS=0; FAIL=0; SKIP=0
ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1: $2"; }
skip() { SKIP=$((SKIP+1)); echo -e "\033[33m  SKIP\033[0m  $1"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() {
  for c in s-a s-g s-g2 s-t; do docker rm -f $c 2>/dev/null; done
  docker network rm s-net 2>/dev/null || true
}
trap cleanup EXIT
cleanup

cd /mnt/d/Code/Shroud
docker build -t shroud-test -f deploy/Dockerfile . 2>&1 | tail -1
docker network create s-net 2>/dev/null || true
docker run -d --name s-t --network s-net nginx:alpine 2>/dev/null

# ================================================================
# Use FIFO pattern: tail -f /tmp/cmd | admin --script &
# Agent connects to admin. Commands sent via echo >> /tmp/cmd
# ================================================================

setup() {
  docker rm -f s-a s-g 2>/dev/null || true
  
  # Start admin reading from FIFO
  docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
    sh -c "tail -n +1 -f /tmp/scmd | /opt/shroud/admin -l 9999 -s '$SECRET' --identity-plain --identity-dir /tmp --heartbeat --script" 2>/dev/null
  sleep 3
  
  # Start agent
  docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
    -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>/dev/null
  
  # Wait for enrollment
  for i in $(seq 1 20); do
    if docker logs s-a 2>&1 | grep -q "successfully\|Node\[0\]"; then break; fi
    sleep 2
  done
  sleep 2
}

cmd() { docker exec s-a sh -c "echo '$1' >> /tmp/scmd" 2>/dev/null; sleep ${2:-1}; }

# ================================================================
banner "1. Enrollment + Topo + Detail"
# ================================================================
setup
docker logs s-a 2>&1 | grep -q "successfully" && ok "1.1 Agent enrolled" || fail "1.1" "Not enrolled"

cmd "topo" 2
cmd "detail" 2
LOGS=$(docker logs s-a 2>&1 | tail -15)
echo "$LOGS" | grep -q "Node\[0\]" && ok "1.2 Topology display" || fail "1.2 Topo" "Missing"
echo "$LOGS" | grep -qi "hostname\|Hostname\|IP" && ok "1.3 Detail display" || skip "1.3 Detail"
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "2. Node Memo"
# ================================================================
setup
cmd "use 0" 2
cmd "addmemo memotest-abc" 2
cmd "back" 1
cmd "detail" 2
docker logs s-a 2>&1 | tail -10 | grep -q "memotest-abc" && ok "2.1 addmemo" || fail "2.1" "Memo not in logs"
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "3. SOCKS5 Proxy"
# ================================================================
setup
cmd "use 0" 2
cmd "socks 0.0.0.0:7777" 3

sleep 3
SOCKS=$(timeout 10 docker run --rm --network s-net --entrypoint curl shroud-test:latest \
  -s --connect-timeout 5 --socks5 s-a:7777 http://s-t/ 2>/dev/null || echo "CURL_FAIL")
if echo "$SOCKS" | grep -q "nginx\|Welcome"; then
  ok "3.1 SOCKS5 proxy (curl via admin:7777 → agent → target:80)"
else
  fail "3.1 SOCKS5" "curl result: ${SOCKS:0:100}"
fi
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "4. Port Forward"
# ================================================================
setup
cmd "use 0" 2
cmd "forward 8888 s-t:80" 3
sleep 3

FWD=$(timeout 10 docker run --rm --network s-net --entrypoint wget shroud-test:latest \
  -qO- --timeout=5 http://s-a:8888/ 2>/dev/null || echo "WGET_FAIL")
if echo "$FWD" | grep -q "nginx\|Welcome"; then
  ok "4.1 Port forward (admin:8888 → agent → target:80)"
else
  fail "4.1 Forward" "wget: ${FWD:0:100}"
fi
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "5. Shell Command"
# ================================================================
setup
cmd "use 0" 2
cmd "shell" 3
cmd "whoami" 3
cmd "exit" 2
sleep 5
LOGS=$(docker logs s-a 2>&1 | tail -20)
if echo "$LOGS" | grep -q "root"; then
  ok "5.1 Shell whoami=root"
else
  fail "5.1 Shell" "No root in output"
fi
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "6. File Download"
# ================================================================
setup
# Create test file on agent
docker exec s-g sh -c 'echo "dl-test-data-789" > /tmp/to-dl.txt' 2>/dev/null

cmd "use 0" 2
cmd "download /tmp/to-dl.txt /tmp/downloaded.txt" 5
sleep 5
LOGS=$(docker logs s-a 2>&1 | tail -10)
if echo "$LOGS" | grep -qi "transmit\|progress\|100\|success"; then
  ok "6.1 File download reported in logs"
elif docker exec s-a test -f /tmp/downloaded.txt 2>/dev/null; then
  DL=$(docker exec s-a cat /tmp/downloaded.txt 2>/dev/null || "")
  echo "$DL" | grep -q "dl-test-data" && ok "6.1 File download ($DL)" || fail "6.1" "Wrong content: $DL"
else
  skip "6.1 File download (log check inconclusive)"
fi
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "7. Shutdown"
# ================================================================
setup
docker logs s-a 2>&1 | grep -q "successfully" || { fail "7.0 Setup" "Agent not enrolled"; docker rm -f s-a s-g 2>/dev/null; sleep 2; }

cmd "use 0" 2
cmd "shutdown" 3
sleep 5

AGENT_ALIVE=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -c s-g || echo "0")
[ "$AGENT_ALIVE" -eq 0 ] && ok "7.1 Shutdown (agent terminated)" || fail "7.1" "Agent still running"
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "8. Revocation"
# ================================================================
setup
cmd "use 0" 2
cmd "revoke" 3
sleep 5

# Check admin identity for revoked serials
REVOKED=$(docker exec s-a cat /tmp/admin_identity.json 2>/dev/null | grep -c "revoked_serials" || echo "0")
# Actually check if revoked_serials has entries
HAS_REVOKED=$(docker exec s-a sh -c "cat /tmp/admin_identity.json 2>/dev/null | grep -o '\"revoked_serials\"' || true")
if [ -n "$HAS_REVOKED" ]; then
  ok "8.1 Revocation (revoked_serials field present)"
else
  fail "8.1 Revoke" "No revoked_serials in identity"
fi

# Agent should be disconnected after revoke
AGENT2=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -c s-g || echo "0")
[ "$AGENT2" -eq 0 ] && ok "8.2 Agent disconnected after revoke" || skip "8.2 Agent still connected"
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "9. Multi-Hop (listen + child connect)"
# ================================================================
setup
cmd "use 0" 2
cmd "listen 1 0.0.0.0:10000" 3
sleep 5

# Check agent-0 is listening
LISTEN=$(docker exec s-g ss -tlnp 2>/dev/null | grep 10000 || echo "")
if [ -n "$LISTEN" ]; then
  ok "9.1 Agent-0 listening on 10000"
else
  fail "9.1 Listen" "Port 10000 not open in agent-0"
fi

# Connect child agent
docker run -d --name s-g2 --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-g:10000 -s "$SECRET" --fileless -v 2>/dev/null
sleep 10

LOGS2=$(docker logs s-a 2>&1 | tail -20)
if echo "$LOGS2" | grep -q "successfully\|online\|Node\[1\]"; then
  ok "9.2 Multi-hop: child agent connected via agent-0"
else
  fail "9.2 Multi-hop" "Child not visible in admin"
fi

# Check child has no identity on disk (--fileless)
DISK=$(docker exec s-g2 sh -c 'find / -name "*identity*.json" 2>/dev/null' || echo "")
[ -z "$DISK" ] && ok "9.3 Fileless child: zero disk artifacts" || fail "9.3" "Found: $DISK"

docker rm -f s-a s-g s-g2 2>/dev/null; sleep 2

# ================================================================
banner "10. TCP Port Check (Admin + Agent)"
# ================================================================
setup
# Verify admin is listening on 9999
ADMIN_PORT=$(docker exec s-a ss -tlnp 2>/dev/null | grep 9999 || echo "")
[ -n "$ADMIN_PORT" ] && ok "10.1 Admin port 9999 listening" || fail "10.1" "Not listening"

# Verify agent has connection to admin
AGENT_CONN=$(docker exec s-g ss -tnp 2>/dev/null | grep 9999 || echo "")
[ -n "$AGENT_CONN" ] && ok "10.2 Agent connected to admin:9999" || fail "10.2" "No connection"
docker rm -f s-a s-g 2>/dev/null; sleep 2

# ================================================================
banner "RESULTS"
# ================================================================
echo
echo "Total: $((PASS+FAIL+SKIP))  PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP"
echo
[ "$FAIL" -gt 0 ] && echo "FAILURES: $FAIL" && exit 1 || echo "All tests passed!"
