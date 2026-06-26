#!/bin/bash
set -e
CMD_FILE=/tmp/cmd
touch "$CMD_FILE"
tail -n +1 -f "$CMD_FILE" | /opt/shroud/admin "$@" --script &
ADMIN_PID=$!
sleep 3
touch /tmp/ready
wait $ADMIN_PID
