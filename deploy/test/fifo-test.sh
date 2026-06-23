#!/bin/bash
set -e
SECRET="t3"
cd /mnt/d/Code/Shroud
docker rm -f s-a s-g 2>/dev/null || true
docker network rm s-net 2>/dev/null || true
docker network create s-net 2>/dev/null || true

# Start admin with FIFO input: tail -f feeds stdin, admin --script reads from stdin
docker run -d --name s-a --network s-net --entrypoint sh shroud-test \
  -c 'touch /tmp/scmd; tail -n +1 -f /tmp/scmd | /opt/shroud/admin -l 9999 -s t3 --identity-plain --identity-dir /tmp --heartbeat --script' \
  2>/dev/null
sleep 4

# Start agent
docker run -d --name s-g --network s-net --entrypoint /opt/shroud/agent shroud-test \
  -c s-a:9999 -s "$SECRET" --identity-plain --identity-dir /tmp/ag -v 2>/dev/null
sleep 8

echo "=== Admin logs ==="
docker logs s-a 2>&1 | tail -15

echo ""
echo "=== Send detail command ==="
docker exec s-a sh -c 'echo detail >> /tmp/scmd'
sleep 3
docker logs s-a 2>&1 | tail -10

echo ""
echo "=== Test completed ==="
docker rm -f s-a s-g 2>/dev/null || true
