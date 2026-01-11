#!/bin/sh
set -e

SOCK="/var/run/docker.sock"

if [ -S "$SOCK" ]; then
  # GID du socket docker (sur l'hôte)
  DOCKER_GID="$(stat -c '%g' "$SOCK")"

  # Trouver si un groupe avec ce GID existe déjà dans le container
  EXISTING_GROUP="$(awk -F: -v gid="$DOCKER_GID" '$3==gid {print $1; exit}' /etc/group || true)"

  if [ -z "$EXISTING_GROUP" ]; then
    # Crée un groupe "dockersock" avec le bon GID
    addgroup -S -g "$DOCKER_GID" dockersock 2>/dev/null || true
    GROUP_NAME="dockersock"
  else
    GROUP_NAME="$EXISTING_GROUP"
  fi

  # Ajoute appuser à ce groupe (pour accéder au socket)
  addgroup appuser "$GROUP_NAME" 2>/dev/null || true
fi

# Lance l'app en non-root
exec su-exec appuser:appgroup "$@"
