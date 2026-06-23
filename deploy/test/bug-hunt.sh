#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
export SHROUD_SECRET='test-shroud-secret-do-not-use-in-production'
C="docker compose -f docker-compose.test.yml"

PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo -e "\033[32m  PASS\033[0m  $1"; }
fail() { FAIL=$((FAIL+1)); echo -e "\033[31m  FAIL\033[0m  $1: $2"; }
banner() { echo; echo "========== $1 =========="; }

cleanup() { $C --profile multihop --profile test down -v --remove-orphans 2>/dev/null || true; }
trap cleanup EXIT

cleanup
$C build 2>&1 | tail -1
$C up -d admin target
for i in $(seq 1 30); do if $C ps admin 2>/dev/null | grep -q healthy; then break; fi; sleep 2; done
$C up -d agent-0; sleep 10

banner "BUG 1: ARGV Scrubbing"
echo "Checking /proc/1/cmdline for secret leakage..."
CMDLINE=$(docker exec shroud-agent-0 cat /proc/1/cmdline 2>/dev/null | tr '\0' ' ') || CMDLINE="cannot read"
if echo "$CMDLINE" | grep -q "$SHROUD_SECRET"; then
  fail "BUG-1" "SECRET VISIBLE in cmdline: $CMDLINE"
  echo ""
  echo "  ROOT CAUSE: scrubSecretArgs() in agent/initial/parser.go sets:"
  echo "    os.Args[i+1] = strings.Repeat(\"x\", len(os.Args[i+1]))"
  echo "  This creates a NEW Go string in heap memory. Go strings are immutable."
  echo "  The kernel's original argv memory (mm->arg_start) is never modified."
  echo "  /proc/self/cmdline reads from the kernel's argv region, not Go's os.Args."
  echo ""
  echo "  FIX (Linux): Use unsafe.Pointer to directly overwrite the const argv area:"
  echo "    ArgsPtr := unsafe.Pointer(&os.Args[0]) -- doesn't work (Go copies)"
  echo "    Better: use syscall to call prctl(PR_SET_MM, PR_SET_MM_ARG_START, ...)"
  echo "    Best: zero the argv memory directly via pointer arithmetic from the"
  echo "    ELF auxiliary vector, or use /proc/self/mem"
else
  ok "ARGV scrubbing works (secret hidden)"
fi

banner "BUG 2: Identity File Validity"
echo "Checking agent_identity.json integrity..."
if docker exec shroud-agent-0 test -f /data/agent_identity.json; then
  JSON=$(docker exec shroud-agent-0 cat /data/agent_identity.json 2>/dev/null)
  echo "File size: $(echo "$JSON" | wc -c) bytes"
  if echo "$JSON" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
    ok "Identity file is valid JSON"
    echo "First 200 chars: $(echo "$JSON" | head -c 200)"
  else
    FIRST_BYTE=$(docker exec shroud-agent-0 xxd -l 1 /data/agent_identity.json 2>/dev/null | awk '{print $2}')
    fail "BUG-2" "agent_identity.json is NOT valid JSON. First byte: 0x$FIRST_BYTE"
    echo "  If first byte is NOT '{' (0x7b), the file may be encrypted or binary."
    echo "  Check: is --identity-plain flag actually preventing encryption?"
    echo "  Or: is the storePassphrase somehow set causing encryption?"
  fi
else
  fail "BUG-2" "agent_identity.json not found at /data/"
fi

banner "BUG 3: --script mode listen command"
echo "Trying to drive listen command through piped stdin..."
echo "=== Script-mode admin output (captured) ==="
timeout 15 docker exec shroud-admin sh -c "printf 'use 0\nlisten\n1\n0.0.0.0:10000\ntopo\n' | /opt/shroud/admin -l 19999 -s test_script_bug --identity-plain --identity-dir /tmp/script-test --tls-enable --tls-insecure --down ws --script" 2>&1 || true
echo "==="
echo "If the output shows 'Please choose' but no 'Node is listening', listen failed in script mode."
echo ""
echo "  ROOT CAUSE: The admin CLI uses interactive terminal reads (termbox-go style)"
echo "  that read KEYPRESS events, not line-buffered stdin. Script mode --script"
echo "  pipes stdin through 'tail -f | admin --script' which reads LINES."
echo "  Multi-step commands like 'listen' (which first asks for mode, then port)"
echo "  issue multiple prompts. If the terminal layer reads keypress events"
echo "  instead of line-buffered io.Reader, it breaks in non-interactive mode."

banner "RESULTS"
echo "PASS: $PASS  FAIL: $FAIL"
if [ "$FAIL" -gt 0 ]; then
  echo "Bugs confirmed. See details above."
fi
