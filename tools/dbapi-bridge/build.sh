#!/bin/sh
# Build do advpp-dbapi-bridge (ponte AdvPP <-> dbapi.so TOTVS).
# Uso: ./build.sh [saida]   (default: ./advpp-dbapi-bridge)
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
OUT=${1:-$DIR/advpp-dbapi-bridge}
g++ -O2 -Wall -Wextra -o "$OUT" "$DIR/bridge.cpp" -ldl -lpthread
echo "build ok: $OUT"
