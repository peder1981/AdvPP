#!/bin/sh
# Build do advpp-dbapi-bridge (ponte AdvPP <-> dbapi.so/dbapi.dll TOTVS).
# Uso: ./build.sh [saida]   (default: ./advpp-dbapi-bridge)
#      CXX=x86_64-w64-mingw32-g++-posix ./build.sh out.exe   (Windows)
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
OUT=${1:-$DIR/advpp-dbapi-bridge}
CXX=${CXX:-g++}
case "$($CXX -dumpmachine)" in
  *mingw*) LIBS="-static -lws2_32 -ldbghelp" ;;  # .exe sem DLL de runtime
  *)       LIBS="-ldl -lpthread" ;;
esac
$CXX -O2 -Wall -Wextra -o "$OUT" "$DIR/bridge.cpp" $LIBS
echo "build ok: $OUT"
