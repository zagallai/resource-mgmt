#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p data backups
chmod 700 data backups
export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
exec docker compose "$@"
