#!/bin/bash
set -e
cd "$(dirname "$0")/.."
export SHROUD_SECRET='test-shroud-secret-do-not-use-in-production'
COMPOSE="docker compose -f docker-compose.test.yml"

echo "=== Starting base ==="
$COMPOSE up -d admin target
echo "Waiting for admin healthy..."
for i in $(seq 1 30); do
  if $COMPOSE ps admin | grep -q healthy; then echo "Admin healthy!"; break; fi
  sleep 2
done

echo "=== Starting agent-0 ==="
$COMPOSE up -d agent-0
sleep 10

echo "=== Admin logs (last 30) ==="
docker logs shroud-admin 2>&1 | tail -30
echo
echo "=== Agent-0 logs (last 30) ==="
docker logs shroud-agent-0 2>&1 | tail -30

echo
echo "=== Diagnosing listen/multi-hop ==="
echo "Sending commands: use 0 → listen → 1 → 0.0.0.0:10000"
docker exec shroud-admin sh -c "echo 'use 0' >> /tmp/cmd"
sleep 2
docker exec shroud-admin sh -c "echo 'listen' >> /tmp/cmd"
sleep 1
docker exec shroud-admin sh -c "echo '1' >> /tmp/cmd"
sleep 1
docker exec shroud-admin sh -c "echo '0.0.0.0:10000' >> /tmp/cmd"
sleep 5

echo "=== Admin logs after listen ==="
docker logs shroud-admin 2>&1 | tail -15
echo
echo "=== Agent-0 check if listening ==="
docker exec shroud-agent-0 netstat -tlnp 2>/dev/null || docker exec shroud-agent-0 ss -tlnp 2>/dev/null || echo "netstat/ss not available"

echo "=== Agent-0 check process ==="
docker exec shroud-agent-0 ps aux 2>/dev/null || echo "ps not available"
echo "=== Agent-0 /proc/cmdline ==="
docker exec shroud-agent-0 sh -c 'for p in /proc/*/cmdline; do echo -n "$p: "; cat "$p" 2>/dev/null | tr \\0 " "; echo; done 2>/dev/null' | grep -v "^$" | head -20
