#!/bin/sh
# Portable builder; inspect and install packages with dpkg on Linux.
set -eu
exec python3 "$(dirname -- "$0")/build-deb.py" "$@"
