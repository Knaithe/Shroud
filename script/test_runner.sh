#!/bin/bash
# Shroud Integration Test Runner
# Usage: ./test_runner.sh [case_pattern]
#   ./test_runner.sh              # run all cases
#   ./test_runner.sh 01           # run only 01_*.sh
#   ./test_runner.sh "01 02 03"   # run specific cases
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test_env.sh"
source "$SCRIPT_DIR/lib/helpers.sh"

CASES_DIR="$SCRIPT_DIR/cases"

echo "========================================"
echo " Shroud Integration Tests"
echo "========================================"
echo " Admin: $IP_A"
echo " Agent1: $IP_B"
echo " Agent2: $IP_C"
echo " Secret: $SECRET"
echo " Port base: $PORT_BASE"
echo "========================================"
echo ""

# Determine which cases to run
if [ $# -gt 0 ]; then
  CASES=()
  for pattern in $1; do
    for f in "$CASES_DIR"/${pattern}*.sh; do
      [ -f "$f" ] && CASES+=("$f")
    done
  done
else
  CASES=("$CASES_DIR"/*.sh)
fi

if [ ${#CASES[@]} -eq 0 ]; then
  echo "No test cases found."
  exit 1
fi

echo "Running ${#CASES[@]} test case(s)..."
echo ""

for case_script in "${CASES[@]}"; do
  run_case "$case_script"
done

report_summary
exit $?
