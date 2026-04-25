#!/bin/sh
set -e

SOCK="/var/run/docker.sock"

if [ -S "$SOCK" ]; then
  DOCKER_GID="$(stat -c '%g' "$SOCK" 2>/dev/null || stat -f '%g' "$SOCK")"

  EXISTING_GROUP="$(awk -F: -v gid="$DOCKER_GID" '$3==gid {print $1; exit}' /etc/group || true)"

  if [ -z "$EXISTING_GROUP" ]; then
    addgroup -S -g "$DOCKER_GID" dockersock 2>/dev/null || true
    GROUP_NAME="dockersock"
  else
    GROUP_NAME="$EXISTING_GROUP"
  fi

  addgroup appuser "$GROUP_NAME" 2>/dev/null || true
fi

exec gosu appuser "$@"
