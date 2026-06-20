#!/bin/bash
# Build admin and agent for linux/amd64, deploy to test machines
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test_env.sh"

PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$BUILD_DIR"

echo "[*] Building linux/amd64 admin..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-w -s" \
  -o "$BUILD_DIR/shroud_admin" "$PROJECT_DIR/admin/admin.go"

echo "[*] Building linux/amd64 agent..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-w -s" \
  -o "$BUILD_DIR/shroud_agent" "$PROJECT_DIR/agent/agent.go"

echo "[*] Deploying to machines..."
for host in B C; do
  host_var="IP_${host}"
  ip="${!host_var}"
  echo "  -> $host ($ip)"
  ssh -o StrictHostKeyChecking=no "${SSH_USER}@${ip}" "mkdir -p /opt/shroud" 2>/dev/null
  scp -o StrictHostKeyChecking=no "$BUILD_DIR/shroud_agent" "${SSH_USER}@${ip}:${AGENT_BIN}"
  ssh -o StrictHostKeyChecking=no "${SSH_USER}@${ip}" "chmod +x ${AGENT_BIN}"
done

# Admin stays on machine A
cp "$BUILD_DIR/shroud_admin" "$ADMIN_BIN" 2>/dev/null || \
  sudo cp "$BUILD_DIR/shroud_admin" "$ADMIN_BIN"
chmod +x "$ADMIN_BIN"

echo "[*] Build and deploy complete."
echo "  Admin: $ADMIN_BIN"
echo "  Agent: $AGENT_BIN (on B and C)"
