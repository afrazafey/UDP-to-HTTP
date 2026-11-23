#!/bin/sh
set -eu
config=${1:?Usage: backup.sh config.yaml new-backup.db}
output=${2:?New backup destination required}
exec udphttp-proxy database backup --config "$config" --output "$output"
