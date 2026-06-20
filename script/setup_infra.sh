#!/bin/bash
# Setup infrastructure on test machines
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test_env.sh"

echo "[*] Setting up machine A (local)..."
mkdir -p "$LOG_DIR"

echo "[*] Setting up machine B..."
ssh_cmd() {
  local ip="$1"; shift
  ssh -o StrictHostKeyChecking=no "${SSH_USER}@${ip}" "$@"
}

# Ensure sshd is running on B
ssh_cmd "$IP_B" "systemctl is-active sshd || systemctl start sshd" 2>/dev/null || true

# Create test user on B for SSH tests (if not exists)
ssh_cmd "$IP_B" "id $SSH_TEST_USER 2>/dev/null || (useradd -m $SSH_TEST_USER && echo '$SSH_TEST_USER:$SSH_TEST_PASS' | chpasswd)" 2>/dev/null || true

# Ensure agent directory exists
ssh_cmd "$IP_B" "mkdir -p /opt/shroud /tmp"
ssh_cmd "$IP_C" "mkdir -p /opt/shroud /tmp"

# Open firewall ports (if firewalld is active)
for host_ip in "$IP_B" "$IP_C"; do
  ssh_cmd "$host_ip" "
    if command -v firewall-cmd &>/dev/null && systemctl is-active firewalld &>/dev/null; then
      for port in \$(seq $PORT_BASE $((PORT_BASE+20))); do
        firewall-cmd --add-port=\${port}/tcp --permanent 2>/dev/null
      done
      firewall-cmd --reload 2>/dev/null
    fi
  " 2>/dev/null || true
done

# Clean up any leftover agent processes
ssh_cmd "$IP_B" "pkill -f shroud_agent" 2>/dev/null || true
ssh_cmd "$IP_C" "pkill -f shroud_agent" 2>/dev/null || true
pkill -f shroud_admin 2>/dev/null || true

echo "[*] Infrastructure setup complete."
echo "  Machine A: $IP_A (admin)"
echo "  Machine B: $IP_B (agent1, sshd)"
echo "  Machine C: $IP_C (agent2)"
