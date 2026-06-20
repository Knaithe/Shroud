#!/bin/bash
# Integration test helper functions

PIDS_TO_KILL=()
ADMIN_FIFO=""
ADMIN_LOG=""
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

cleanup() {
  for pid in "${PIDS_TO_KILL[@]}"; do
    kill "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null
  done
  PIDS_TO_KILL=()
  [ -n "$ADMIN_FIFO" ] && rm -f "$ADMIN_FIFO"
  ssh_cmd B "pkill -f shroud_agent" 2>/dev/null
  ssh_cmd C "pkill -f shroud_agent" 2>/dev/null
  pkill -f shroud_admin 2>/dev/null
  sleep 0.5
}

trap cleanup EXIT

ssh_cmd() {
  local host_var="IP_${1}"
  local host="${!host_var}"
  shift
  ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${SSH_USER}@${host}" "$@"
}

ssh_bg() {
  local host_var="IP_${1}"
  local host="${!host_var}"
  shift
  ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${SSH_USER}@${host}" "$@" &
  PIDS_TO_KILL+=($!)
}

start_admin_listen() {
  local port="$1"; shift
  local secret="$1"; shift
  local extra_flags="$*"

  mkdir -p "$LOG_DIR"
  ADMIN_LOG="${LOG_DIR}/admin_${port}.log"
  ADMIN_FIFO="${LOG_DIR}/admin_${port}.fifo"
  rm -f "$ADMIN_FIFO"
  mkfifo "$ADMIN_FIFO"

  # Start admin in script mode, reading from FIFO
  (tail -f "$ADMIN_FIFO" | $ADMIN_BIN -l "$port" -s "$secret" --script $extra_flags > "$ADMIN_LOG" 2>&1) &
  PIDS_TO_KILL+=($!)
  sleep 0.3
}

start_admin_connect() {
  local target="$1"; shift
  local secret="$1"; shift
  local extra_flags="$*"

  mkdir -p "$LOG_DIR"
  ADMIN_LOG="${LOG_DIR}/admin_connect.log"
  ADMIN_FIFO="${LOG_DIR}/admin_connect.fifo"
  rm -f "$ADMIN_FIFO"
  mkfifo "$ADMIN_FIFO"

  (tail -f "$ADMIN_FIFO" | $ADMIN_BIN -c "$target" -s "$secret" --script $extra_flags > "$ADMIN_LOG" 2>&1) &
  PIDS_TO_KILL+=($!)
  sleep 0.3
}

admin_cmd() {
  echo "$1" >> "$ADMIN_FIFO"
  sleep 1
}

admin_output() {
  cat "$ADMIN_LOG" 2>/dev/null
}

start_agent_remote() {
  local host="$1"; shift
  local flags="$*"
  mkdir -p "$LOG_DIR"
  local log="${LOG_DIR}/agent_${host}.log"
  ssh_cmd "$host" "nohup $AGENT_BIN $flags > /tmp/shroud_agent.log 2>&1 &"
  sleep 0.5
}

stop_agent_remote() {
  local host="$1"
  ssh_cmd "$host" "pkill -f shroud_agent" 2>/dev/null
  sleep 0.5
}

wait_for_port() {
  local host_var="IP_${1}"
  local host="${!host_var}"
  local port="$2"
  local timeout="${3:-$CONNECT_TIMEOUT}"
  local elapsed=0
  while [ $elapsed -lt $timeout ]; do
    if nc -z -w1 "$host" "$port" 2>/dev/null; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  return 1
}

wait_for_log() {
  local pattern="$1"
  local timeout="${2:-$CONNECT_TIMEOUT}"
  local log="${3:-$ADMIN_LOG}"
  local elapsed=0
  while [ $elapsed -lt $timeout ]; do
    if grep -qi "$pattern" "$log" 2>/dev/null; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  return 1
}

wait_for_remote_log() {
  local host="$1"
  local pattern="$2"
  local timeout="${3:-$CONNECT_TIMEOUT}"
  local elapsed=0
  while [ $elapsed -lt $timeout ]; do
    if ssh_cmd "$host" "grep -qi '$pattern' /tmp/shroud_agent.log 2>/dev/null"; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  return 1
}

assert_output_contains() {
  local pattern="$1"
  if grep -qi "$pattern" "$ADMIN_LOG" 2>/dev/null; then
    return 0
  fi
  echo "  ASSERT FAILED: output does not contain '$pattern'"
  echo "  Actual output:"
  tail -20 "$ADMIN_LOG" 2>/dev/null | sed 's/^/    /'
  return 1
}

assert_output_not_contains() {
  local pattern="$1"
  if ! grep -qi "$pattern" "$ADMIN_LOG" 2>/dev/null; then
    return 0
  fi
  echo "  ASSERT FAILED: output should not contain '$pattern'"
  return 1
}

assert_remote_log_contains() {
  local host="$1"
  local pattern="$2"
  if ssh_cmd "$host" "grep -qi '$pattern' /tmp/shroud_agent.log 2>/dev/null"; then
    return 0
  fi
  echo "  ASSERT FAILED: $host agent log does not contain '$pattern'"
  return 1
}

assert_exit_nonzero() {
  local cmd="$1"
  if eval "$cmd" 2>/dev/null; then
    echo "  ASSERT FAILED: expected nonzero exit, got 0"
    return 1
  fi
  return 0
}

pass() {
  local name="$1"
  local duration="$2"
  PASS_COUNT=$((PASS_COUNT + 1))
  printf "\033[32m[PASS]\033[0m %s" "$name"
  [ -n "$duration" ] && printf " (%ss)" "$duration"
  printf "\n"
}

fail() {
  local name="$1"
  local reason="$2"
  FAIL_COUNT=$((FAIL_COUNT + 1))
  printf "\033[31m[FAIL]\033[0m %s" "$name"
  [ -n "$reason" ] && printf " - %s" "$reason"
  printf "\n"
}

skip() {
  local name="$1"
  local reason="$2"
  SKIP_COUNT=$((SKIP_COUNT + 1))
  printf "\033[33m[SKIP]\033[0m %s - %s\n" "$name" "$reason"
}

report_summary() {
  local total=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
  echo ""
  echo "========================================"
  printf "Results: %d/%d passed" "$PASS_COUNT" "$total"
  [ $FAIL_COUNT -gt 0 ] && printf ", \033[31m%d failed\033[0m" "$FAIL_COUNT"
  [ $SKIP_COUNT -gt 0 ] && printf ", %d skipped" "$SKIP_COUNT"
  printf "\n"
  echo "========================================"
  [ $FAIL_COUNT -gt 0 ] && return 1
  return 0
}

run_case() {
  local script="$1"
  local name
  name=$(basename "$script" .sh)
  local start_time
  start_time=$(date +%s)

  cleanup 2>/dev/null

  if bash "$script"; then
    local end_time
    end_time=$(date +%s)
    pass "$name" "$((end_time - start_time))"
  else
    fail "$name"
  fi

  cleanup 2>/dev/null
}
