#!/bin/bash
HOST="$1"; shift
PORT="$1"; shift
echo "[wait] Waiting for $HOST:$PORT..."
while ! nc -z "$HOST" "$PORT" 2>/dev/null; do sleep 1; done
echo "[wait] $HOST:$PORT ready, starting agent"
exec /opt/shroud/agent "$@"
