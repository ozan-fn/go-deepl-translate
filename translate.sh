#!/usr/bin/env bash
# Terjemahkan file .srt: ./translate.sh file.srt  ->  file.id.srt
set -euo pipefail
cd "$(dirname "$0")"

if [ $# -lt 1 ]; then
	echo "Usage: ./translate.sh <file.srt> [output.srt]" >&2
	exit 1
fi

in="$1"
out="${2:-${in%.*}.id.srt}"

go run . "$in" > "$out"
echo "wrote $out" >&2
