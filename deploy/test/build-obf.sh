#!/bin/bash
set -e
cd /mnt/d/Code/Shroud
G=/home/worker/go/bin/garble

echo "Building obfuscated admin..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $G -literals -tiny -seed=random build -trimpath -ldflags="-s -w" -o /tmp/sa_obf ./admin 2>&1

echo "Building obfuscated agent..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $G -literals -tiny -seed=random build -trimpath -ldflags="-s -w" -o /tmp/sg_obf ./agent 2>&1

echo "=== File sizes ==="
ls -lh /tmp/sa_obf /tmp/sg_obf

echo ""
echo "=== Admin strings check ==="
echo "Total strings: $(strings /tmp/sa_obf 2>/dev/null | wc -l)"
echo "Shroud mentions: $(strings /tmp/sa_obf 2>/dev/null | grep -ic shroud)"

echo ""
echo "=== Agent strings check ==="
echo "Total strings: $(strings /tmp/sg_obf 2>/dev/null | wc -l)"
echo "Shroud mentions: $(strings /tmp/sg_obf 2>/dev/null | grep -ic shroud)"

echo ""
echo "=== Obfuscated vs normal ==="
ls -lh /tmp/shroud_admin /tmp/shroud_agent /tmp/sa_obf /tmp/sg_obf 2>/dev/null
