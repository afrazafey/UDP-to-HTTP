#!/bin/sh
set -eu
input=${1:?Usage: restore.sh backup.db new-destination.db --confirm-offline}
output=${2:?New destination required}
test "${3:-}" = --confirm-offline || { echo 'Stop the service and pass --confirm-offline. Existing destinations are never replaced.' >&2; exit 1; }
exec udphttp-proxy database restore --input "$input" --output "$output" --confirm-offline
