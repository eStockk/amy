#!/bin/sh
set -eu

skin_dir="${SKIN_STORAGE_DIR:-data/skins}"
mkdir -p "$skin_dir"
chown -R 10001:10001 "$skin_dir"

exec su-exec amy "$@"
