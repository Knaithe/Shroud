#!/bin/bash
set -euo pipefail
SECRET="debug123"
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo "PASS $1"; }
fail() { FAIL=$((FAIL+1)); echo "FAIL $1: $2"; }

cd /mnt/d/Code/Shroud
docker rm -f s-a s-g 2>/dev/null || true
docker network rm s-net 2>/dev/null || true
docker network create s-net 2>/dev/null || true

docker run -d --name s-a --network s-net --entrypoint /opt/shroud/admin shroud-test \
  --daemon -l 9999 -s "$SECRET" --identity-plain --identity-dir /tmp 2>/dev/null
sleep 4

docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>/dev/null
sleep 8

echo "=== Daemon admin logs ==="
docker logs s-a 2>&1 | tail -5

echo ""
echo "=== Test: simple detail command ==="
echo "detail" > /tmp/scmd1
timeout 15 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/dbg --script < /tmp/scmd1 2>&1 | head -30
echo "---exit code: $?"

echo ""
echo "=== Test: use 0 then detail ==="
printf "use 0\ndetail\n" > /tmp/scmd2
timeout 15 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/dbg2 --script < /tmp/scmd2 2>&1 | head -50
echo "---exit code: $?"

echo ""
echo "=== Test: use 0 then socks 7777 ==="
printf "use 0\nsocks 0.0.0.0:7777\n" > /tmp/scmd3
timeout 15 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/dbg3 --script < /tmp/scmd3 2>&1 | head -30
echo "---exit code: $?"

echo ""
echo "=== Check if socks port is open on admin ==="
sleep 3
docker exec s-a ss -tlnp 2>/dev/null | grep -E "7777|LISTEN" | head -5 || echo "no ports found"

echo ""
echo "=== Test: shell whoami ==="
printf "use 0\nshell\nwhoami\nexit\n" > /tmp/scmd4
timeout 15 docker run --rm --network s-net --entrypoint /opt/shroud/admin shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/dbg4 --script < /tmp/scmd4 2>&1 | head -50
echo "---exit code: $?"

docker rm -f s-a s-g 2>/dev/null || true
echo "Done: PASS=$PASS FAIL=$FAIL"
