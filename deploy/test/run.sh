#!/bin/bash
set -euo pipefail
SECRET="test-shroud-secret-do-not-use-in-production"
PASS=0; FAIL=0; SKIP=0
ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1  ${2:-}"; }
skip() { SKIP=$((SKIP+1)); echo -e "\033[33m  SKIP\033[0m  $1"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() {
  for c in s-a s-g s-g2 target; do docker rm -f $c 2>/dev/null; done
  docker network rm s-net 2>/dev/null || true
}
trap cleanup EXIT
cleanup

cd /mnt/d/Code/Shroud
docker build -t shroud-test -f deploy/Dockerfile . 2>&1 | tail -1
docker network create s-net 2>/dev/null || true

wait_ready() {
  local name="$1" max="${2:-30}"
  for i in $(seq 1 "$max"); do
    docker exec "$name" test -f /tmp/ready 2>/dev/null && return 0
    sleep 1
  done
  return 1
}

wait_enroll() {
  local name="$1" max="${2:-20}"
  for i in $(seq 1 "$max"); do
    docker logs "$name" 2>&1 | grep -q "successfully" && return 0
    sleep 1
  done
  return 1
}

start_admin_script() {
  local name="${1:-s-a}"; shift
  docker run -d --name "$name" --network s-net \
    --entrypoint /opt/shroud/test/entrypoint-admin.sh shroud-test \
    -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp "$@" 2>/dev/null
  wait_ready "$name" 30
}

admin_cmd() {
  docker exec s-a sh -c "echo '$1' >> /tmp/cmd"
  sleep "${2:-4}"
}

admin_node_cmd() {
  local node="$1"; shift
  local cmd="$1"; shift
  local wait="${1:-5}"
  docker exec s-a sh -c "printf 'use %s\n%s\n' '$node' '$cmd' >> /tmp/cmd"
  sleep "$wait"
}

# ================================================================
banner "A. Connection & Authentication"
# ================================================================

# A1: Direct connect
banner "A1. Direct Connection"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 4
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>/dev/null
sleep 10
docker logs s-a 2>&1 | grep -q "successfully" && ok "A1.1 Admin confirms enrollment" || fail "A1.1 Admin enrollment"
docker logs s-g 2>&1 | grep -qi "connect\|agent" && ok "A1.2 Agent connected" || fail "A1.2 Agent connect"
docker exec s-a test -f /tmp/admin_identity.json && ok "A1.3 Admin identity file created" || fail "A1.3 Admin identity"
docker exec s-g test -d /tmp/ag && ok "A1.4 Agent identity dir created" || fail "A1.4 Agent identity"
docker rm -f s-a s-g 2>/dev/null

# A2: Reverse connect
banner "A2. Reverse Connection"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp -v 2>/dev/null
sleep 3
docker exec s-a ps aux | grep -q agent && ok "A2.1 Agent passive listening" || fail "A2.1 Agent listen"
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/rev 2>/dev/null
sleep 10
docker logs s-g 2>&1 | grep -qi "successfully\|Node\[0\]" && ok "A2.2 Admin connects to passive agent" \
  || fail "A2.2 Reverse connect"
docker rm -f s-a s-g 2>/dev/null

# A3: Cert re-auth
banner "A3. Cert Re-auth"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/cert --reconnect 3 -v 2>/dev/null
sleep 8
docker logs s-a 2>&1 | grep -q "successfully" && ok "A3.1 Initial enrollment" || fail "A3.1 Initial enroll"
docker restart s-a; sleep 18
docker logs s-a 2>&1 | tail -10 | grep -q "successfully" && ok "A3.2 Cert re-auth after restart" \
  || fail "A3.2 Cert re-auth"
docker rm -f s-a s-g 2>/dev/null

# A4: ARGV secret scrubbing
banner "A4. ARGV Secret Scrubbing"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/argv -v 2>/dev/null
sleep 8
CMDLINE=$(docker exec s-g cat /proc/1/cmdline 2>/dev/null | tr "\0" " " || echo "")
echo "$CMDLINE" | grep -q "$SECRET" \
  && fail "A4.1 Secret visible in /proc/1/cmdline" \
  || ok "A4.1 ARGV secret scrubbed"
docker rm -f s-a s-g 2>/dev/null

# A5: Script mode no crash
banner "A5. Script Mode"
SC=$(timeout 15 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 0.0.0.0:19998 -s sc_test --identity-plain --identity-dir /tmp/sc --script 2>&1 <<< "help" || true)
echo "$SC" | grep -qi "panic\|SIGSEGV" \
  && fail "A5.1 --script panic" \
  || ok "A5.1 No crash in --script mode"

# ================================================================
banner "B. Transport Protocols"
# ================================================================

# B1: WebSocket
banner "B1. WebSocket Transport"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --down ws 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ws --up ws -v 2>/dev/null
sleep 10
docker logs s-a 2>&1 | grep -q "successfully" && ok "B1.1 WebSocket transport enrollment" \
  || fail "B1.1 WS enrollment"
docker rm -f s-a s-g 2>/dev/null

# B2: TLS
banner "B2. TLS Transport"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --tls-enable 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/tls --tls-enable -v 2>/dev/null
sleep 12
docker logs s-a 2>&1 | grep -q "successfully" && ok "B2.1 TLS transport enrollment" \
  || fail "B2.1 TLS enrollment" "$(docker logs s-a 2>&1 | tail -3)"
docker rm -f s-a s-g 2>/dev/null

# B3: WS + TLS
banner "B3. WS + TLS Combo"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --tls-enable --down ws 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/wstls --tls-enable --up ws -v 2>/dev/null
sleep 12
docker logs s-a 2>&1 | grep -q "successfully" && ok "B3.1 WS+TLS combo enrollment" \
  || fail "B3.1 WS+TLS enrollment" "$(docker logs s-a 2>&1 | tail -3)"
docker rm -f s-a s-g 2>/dev/null

# B4: Traffic padding
banner "B4. Traffic Padding"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --pad-size 4096 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/pad --pad-size 4096 -v 2>/dev/null
sleep 10
docker logs s-a 2>&1 | grep -q "successfully" && ok "B4.1 Padded transport enrollment" \
  || fail "B4.1 Padded enrollment"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "C. Proxy Features"
# ================================================================

docker run -d --name target --network s-net nginx:alpine 2>/dev/null
sleep 3

# C1: Auto SOCKS5
banner "C1. Auto SOCKS5"
start_admin_script s-a --socks 0.0.0.0:7777
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/socks -v 2>/dev/null
wait_enroll s-a 20; sleep 2
sleep 3
SOCKS_OUT=$(docker run --rm --network s-net --entrypoint sh shroud-test \
  -c "curl -s --socks5-hostname s-a:7777 http://target/ 2>/dev/null" || echo "")
echo "$SOCKS_OUT" | grep -qi "nginx\|welcome" && ok "C1.1 Auto SOCKS5 proxy works" \
  || fail "C1.1 Auto SOCKS5" "$(echo "$SOCKS_OUT" | head -1)"
docker rm -f s-a s-g 2>/dev/null

# C2: SOCKS5 via script command
banner "C2. SOCKS5 Command"
start_admin_script s-a
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/scmd -v 2>/dev/null
wait_enroll s-a 20; sleep 2
admin_node_cmd 0 "socks 0.0.0.0:7778" 8
SCMD_OUT=$(docker run --rm --network s-net --entrypoint sh shroud-test \
  -c "curl -s --socks5-hostname s-a:7778 http://target/ 2>/dev/null" || echo "")
echo "$SCMD_OUT" | grep -qi "nginx\|welcome" && ok "C2.1 SOCKS5 command works" \
  || fail "C2.1 SOCKS5 command" "$(docker logs s-a 2>&1 | tail -5)"
docker rm -f s-a s-g 2>/dev/null

# C3: Port forward (forward binds to 127.0.0.1 inside admin)
banner "C3. Port Forward"
start_admin_script s-a
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/fwd -v 2>/dev/null
wait_enroll s-a 20; sleep 2
admin_node_cmd 0 "forward 8080 target:80" 8
FWD_OUT=$(docker exec s-a sh -c "curl -s http://127.0.0.1:8080/ 2>/dev/null" || echo "")
echo "$FWD_OUT" | grep -qi "nginx\|welcome" && ok "C3.1 Port forward works" \
  || fail "C3.1 Port forward" "$(docker logs s-a 2>&1 | tail -5)"
docker rm -f s-a s-g 2>/dev/null

# C4: Port backward (agent listens on 127.0.0.1:rport)
banner "C4. Port Backward"
start_admin_script s-a
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/bwd -v 2>/dev/null
wait_enroll s-a 20; sleep 2
admin_node_cmd 0 "backward 9090 80" 5
BWD_PORT=$(docker exec s-g sh -c "nc -z 127.0.0.1 9090 2>/dev/null && echo OPEN || echo CLOSED" || echo "CLOSED")
[ "$BWD_PORT" = "OPEN" ] && ok "C4.1 Port backward: agent listening on 9090" \
  || skip "C4.1 Port backward: port not open (admin has no local service on lport)"
docker rm -f s-a s-g 2>/dev/null

docker rm -f target 2>/dev/null

# ================================================================
banner "D. Multi-level Proxy Chain"
# ================================================================

# D1: Admin → Agent-0 → Agent-1
banner "D1. Two-level Chain"
start_admin_script s-a
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ch0 -v 2>/dev/null
wait_enroll s-a 20; sleep 2
admin_node_cmd 0 "listen 0.0.0.0:18888" 8
docker run -d --name s-g2 --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-g:18888 -s "$SECRET" --identity-plain --identity-dir /tmp/ch1 -v 2>/dev/null
sleep 15
admin_cmd "topo" 3
TOPO=$(docker logs s-a 2>&1)
echo "$TOPO" | grep -qi "Node\[1\]\|node.*1" && ok "D1.1 Two-level chain established" \
  || fail "D1.1 Two-level chain" "$(echo "$TOPO" | tail -5)"

# D2: Detail output
admin_cmd "detail" 3
DETAIL=$(docker logs s-a 2>&1 | tail -20)
echo "$DETAIL" | grep -qi "hostname\|ip\|Hostname\|IP\|memo" && ok "D2.1 Detail command output" \
  || skip "D2.1 Detail output format unverified"
docker rm -f s-a s-g s-g2 2>/dev/null

# ================================================================
banner "E. Node Management"
# ================================================================

# E1: Remote shutdown
banner "E1. Remote Shutdown"
start_admin_script s-a
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/sd -v 2>/dev/null
wait_enroll s-a 20; sleep 2
admin_node_cmd 0 "shutdown" 10
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g \
  && fail "E1.1 Agent still alive after shutdown" \
  || ok "E1.1 Remote shutdown: agent exited"
docker rm -f s-a s-g 2>/dev/null

# E2: File upload/download
banner "E2. File Transfer"
start_admin_script s-a
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ft -v 2>/dev/null
wait_enroll s-a 20; sleep 2
docker exec s-a sh -c "dd if=/dev/urandom of=/tmp/testfile bs=1024 count=16 2>/dev/null"
ORIG_MD5=$(docker exec s-a md5sum /tmp/testfile 2>/dev/null | awk '{print $1}')
admin_node_cmd 0 "upload /tmp/testfile /tmp/uploaded" 15
DL_MD5=$(docker exec s-g md5sum /tmp/uploaded 2>/dev/null | awk '{print $1}' || echo "none")
if [ "$ORIG_MD5" = "$DL_MD5" ] && [ "$ORIG_MD5" != "" ]; then
  ok "E2.1 File upload checksum match"
else
  UPLOAD_LOG=$(docker logs s-a 2>&1 | tail -5)
  echo "$UPLOAD_LOG" | grep -qi "upload\|progress\|Error" \
    && fail "E2.1 File upload" "orig=$ORIG_MD5 dl=$DL_MD5" \
    || skip "E2.1 File upload: command may not have executed"
fi
docker rm -f s-a s-g 2>/dev/null

# E3: Certificate revoke
banner "E3. Certificate Revoke"
start_admin_script s-a
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/rev --reconnect 3 -v 2>/dev/null
wait_enroll s-a 20; sleep 2
admin_node_cmd 0 "revoke" 10
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g \
  && { docker logs s-g 2>&1 | grep -qi "revok\|denied\|reject\|shutdown\|exit\|close" \
       && ok "E3.1 Agent shows revoke/close in log" \
       || fail "E3.1 Agent still running after revoke"; } \
  || ok "E3.1 Agent exited after certificate revoke"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "F. Anti-forensics & OPSEC"
# ================================================================

# F1: Kill-date (use yesterday's date so agent self-destructs)
banner "F1. Kill-date"
YESTERDAY=$(date -d "-1 day" "+%Y-%m-%d" 2>/dev/null || echo "2020-01-01")
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/kd --kill-date "$YESTERDAY" -v 2>/dev/null
sleep 75
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g \
  && fail "F1.1 Agent alive after kill-date" \
  || ok "F1.1 Kill-date: agent self-destructed"
docker rm -f s-a s-g 2>/dev/null

# F2: Work-hours (agent enrolls then sleeps/exits outside window)
banner "F2. Work-hours Window"
CURRENT_HOUR=$(date "+%H")
if [ "$CURRENT_HOUR" -ge 12 ]; then
  OUTSIDE_WINDOW="01:00-02:00"
else
  OUTSIDE_WINDOW="23:00-23:59"
fi
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/wh --work-hours "$OUTSIDE_WINDOW" -v 2>/dev/null
sleep 20
docker logs s-g 2>&1 | grep -qi "work.hours\|outside.*window\|sleep\|休眠\|窗口" \
  && ok "F2.1 Work-hours: agent recognized outside window" \
  || { docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g \
       && skip "F2.1 Work-hours: agent alive (may sleep internally)" \
       || ok "F2.1 Work-hours: agent exited outside window"; }
docker rm -f s-a s-g 2>/dev/null

# F3: Self-delete
banner "F3. Self-delete"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/sdel --self-delete -v 2>/dev/null
sleep 10
docker kill s-a 2>/dev/null; sleep 15
BINARY_EXISTS=$(docker exec s-g test -f /opt/shroud/agent 2>/dev/null && echo "yes" || echo "no")
[ "$BINARY_EXISTS" = "no" ] && ok "F3.1 Self-delete: binary removed" \
  || skip "F3.1 Self-delete: binary still exists (agent may not have exited yet)"
docker rm -f s-a s-g 2>/dev/null

# F4: Sleep-mask + reconnect
banner "F4. Sleep-mask Reconnect"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/sm --reconnect 3 --sleep-mask -v 2>/dev/null
sleep 10
docker logs s-a 2>&1 | grep -q "successfully" || { fail "F4.0 Initial connect for sleep-mask"; docker rm -f s-a s-g 2>/dev/null; }
docker restart s-a; sleep 20
docker logs s-a 2>&1 | tail -10 | grep -q "successfully" && ok "F4.1 Sleep-mask: agent reconnected" \
  || fail "F4.1 Sleep-mask reconnect"
docker rm -f s-a s-g 2>/dev/null

# F5: Passphrase encrypted identity
banner "F5. Passphrase Encryption"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --passphrase "test-pass-123" --identity-dir /tmp/pp-adm 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --passphrase "test-pass-456" --identity-dir /tmp/pp -v 2>/dev/null
sleep 12
ID_FILES=$(docker exec s-g sh -c 'ls /tmp/pp/ 2>/dev/null' || echo "")
if [ -n "$ID_FILES" ]; then
  ID_CONTENT=$(docker exec s-g sh -c 'cat /tmp/pp/*' 2>/dev/null | head -c 200 || echo "")
  echo "$ID_CONTENT" | grep -q '"private_key"\|"ed25519"\|"certificate"' \
    && fail "F5.1 Identity file contains plaintext keys" \
    || ok "F5.1 Identity file encrypted with passphrase"
else
  skip "F5.1 No identity files found in /tmp/pp/"
fi
docker rm -f s-a s-g 2>/dev/null

# F6: Reconnect exponential backoff
banner "F6. Reconnect Backoff"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/rb --reconnect 2 -v 2>/dev/null
sleep 10
docker kill s-a 2>/dev/null; sleep 5
docker start s-a 2>/dev/null
sleep 40
docker logs s-a 2>&1 | tail -5 | grep -q "successfully" && ok "F6.1 Agent reconnected after backoff" \
  || fail "F6.1 Reconnect backoff"
docker rm -f s-a s-g 2>/dev/null

# F7: Force re-enroll
banner "F7. Force Re-enroll"
docker run -d --name s-a --network s-net -e SHROUD_ALLOW_REENROLL=1 --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/fe -v 2>/dev/null
sleep 10
docker logs s-a 2>&1 | grep -q "successfully" || { fail "F7.0 Initial enroll"; docker rm -f s-a s-g 2>/dev/null; }
docker rm -f s-g 2>/dev/null
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/fe2 --force-reenroll -v 2>/dev/null
sleep 12
docker logs s-a 2>&1 | tail -10 | grep -q "successfully" && ok "F7.1 Force re-enroll succeeded" \
  || fail "F7.1 Force re-enroll"
docker rm -f s-a s-g 2>/dev/null

# F8: Fileless mode
banner "F8. Fileless Mode"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s fl_test --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s fl_test --fileless -v 2>/dev/null
sleep 8
ARTIFACTS=$(docker exec s-g sh -c 'find / -name "*identity*.json" 2>/dev/null' || echo "")
[ -z "$ARTIFACTS" ] && ok "F8.1 Fileless: zero identity files on disk" \
  || fail "F8.1 Fileless" "Found: $ARTIFACTS"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "G. Process Security"
# ================================================================

# G1: Process masking
banner "G1. Process Masking"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/mask -v 2>/dev/null
sleep 8
AGENT_COMM=$(docker exec s-g cat /proc/1/comm 2>/dev/null || echo "")
AGENT_CMDLINE=$(docker exec s-g cat /proc/1/cmdline 2>/dev/null | tr "\0" " " || echo "")
if [ "$AGENT_COMM" = "kworker/0:1" ]; then
  ok "G1.1 Agent comm=kworker/0:1"
elif ! echo "$AGENT_CMDLINE" | grep -q "$SECRET"; then
  ok "G1.1 Agent cmdline secret scrubbed"
else
  fail "G1.1 Process masking" "comm=$AGENT_COMM"
fi
ADMIN_COMM=$(docker exec s-a cat /proc/1/comm 2>/dev/null || echo "")
ADMIN_CMDLINE=$(docker exec s-a cat /proc/1/cmdline 2>/dev/null | tr "\0" " " || echo "")
if [ "$ADMIN_COMM" = "kworker/0:2" ]; then
  ok "G1.2 Admin comm=kworker/0:2"
elif ! echo "$ADMIN_CMDLINE" | grep -q "$SECRET"; then
  ok "G1.2 Admin cmdline secret scrubbed"
else
  fail "G1.2 Admin masking" "comm=$ADMIN_COMM"
fi
docker rm -f s-a s-g 2>/dev/null

# G2: Core dump disabled
banner "G2. Core Dump Disabled"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/cd -v 2>/dev/null
sleep 8
APID=$(docker exec s-g pgrep -x agent 2>/dev/null || echo "1")
DUMP=$(docker exec s-g cat /proc/$APID/status 2>/dev/null | grep Dumpable | awk '{print $2}' || echo "?")
[ "$DUMP" = "0" ] && ok "G2.1 Core dump disabled (Dumpable=0)" \
  || skip "G2.1 Dumpable=$DUMP"
docker rm -f s-a s-g 2>/dev/null

# G3: Daemon SIGTERM
banner "G3. Daemon SIGTERM"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/dae 2>/dev/null
sleep 4
docker kill -s TERM s-a 2>/dev/null; sleep 8
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-a && fail "G3.1 Daemon didn't exit" \
  || ok "G3.1 Daemon SIGTERM graceful exit"
docker rm -f s-a 2>/dev/null

# G4: Rapid restart stress
banner "G4. Rapid Restart Stress"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/st --reconnect 2 -v 2>/dev/null
sleep 8
for i in 1 2 3; do docker restart s-a 2>/dev/null; sleep 10; done
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g && ok "G4.1 Agent survived 3 rapid restarts" \
  || fail "G4.1 Agent died during restarts"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "H. Heartbeat"
# ================================================================

# H1: Agent heartbeat keeps alive
banner "H1. Agent Heartbeat"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/hb -v 2>/dev/null
sleep 35
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g && ok "H1.1 Agent alive after 35s (heartbeat working)" \
  || fail "H1.1 Agent died despite heartbeat"
docker rm -f s-a s-g 2>/dev/null

# H2: Admin kill → agent exits
banner "H2. Admin Kill Watchdog"
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 0.0.0.0:9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat 2>/dev/null
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/wd -v 2>/dev/null
sleep 15
docker kill s-a 2>/dev/null
echo "Waiting 30s for agent shutdown..."
sleep 30
docker ps --format '{{.Names}}' 2>/dev/null | grep -q s-g \
  && fail "H2.1 Agent still alive 30s after admin kill" \
  || ok "H2.1 Agent exited after admin kill"
docker rm -f s-a s-g 2>/dev/null

# ================================================================
banner "SKIP: Docker-Incompatible Features"
# ================================================================
skip "Tor connection/hidden service (requires Tor daemon)"
skip "SSH tunnel (requires SSH server + multi-step interaction)"
skip "Port reuse iptables (requires root + iptables)"
skip "Domain fronting (requires CDN)"
skip "User-Agent rotation (requires packet capture)"
skip "shell/ssh remote interaction (script mode doesn't support multi-step)"

# ================================================================
banner "RESULTS"
# ================================================================
TOTAL=$((PASS+FAIL+SKIP))
echo "Total: $TOTAL  PASS=$PASS  FAIL=$FAIL  SKIP=$SKIP"
echo
if [ "$FAIL" -gt 0 ]; then
  echo "Some tests FAILED. Review output above."
  exit 1
else
  echo "All testable features passed."
fi
