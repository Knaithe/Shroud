#!/bin/bash
set -euo pipefail
cd /mnt/d/Code/Shroud
docker rm -f s-a s-g 2>/dev/null || true
docker network rm s-net 2>/dev/null || true
docker network create s-net 2>/dev/null || true

SECRET="test123"

echo "========== BUG-1: ARGV Scrubbing =========="
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp --heartbeat >/dev/null 2>&1
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v >/dev/null 2>&1
sleep 8

CMDLINE=$(docker exec s-g cat /proc/1/cmdline 2>/dev/null | tr '\0' ' ' || echo "")
if echo "$CMDLINE" | grep -q "$SECRET"; then
  echo "FAIL: Secret visible in cmdline"
else
  echo "PASS: Secret hidden from /proc/1/cmdline"
fi
docker rm -f s-a s-g 2>/dev/null

echo ""
echo "========== BUG-2: --script crash =========="
OUT=$(timeout 10 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -l 19998 -s test123 --identity-plain --identity-dir /tmp/s --script 2>&1 <<< "help" || true)
if echo "$OUT" | grep -qi "panic\|SIGSEGV"; then
  echo "FAIL: --script mode crashed"
else
  echo "PASS: No --script crash"
fi

echo ""
echo "========== BUG-3: listen single-line =========="
docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp >/dev/null 2>&1
sleep 3
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag2 -v >/dev/null 2>&1
sleep 8

timeout 15 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/test2 --script >/tmp/listen-out.txt 2>&1 << 'CMDS'
use 0
listen 1 0.0.0.0:10000
back
topo
CMDS

sleep 3
LISTEN=$(docker exec s-g ss -tlnp 2>/dev/null | grep 10000 || echo "")
if [ -n "$LISTEN" ]; then
  echo "PASS: Agent listening on 10000 (listen 1 <port> works)"
else
  echo "FAIL: Port 10000 not open - single-line listen may not work"
  echo "Output:"
  cat /tmp/listen-out.txt | head -20
fi
docker rm -f s-a s-g 2>/dev/null

echo ""
echo "Done."
