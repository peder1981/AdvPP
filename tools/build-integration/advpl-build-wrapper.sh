#!/bin/bash
# Wrapper script for AdvPL builds with automatic key capture
# Usage: advpl-build-wrapper.sh <appsrv_args...>

HOOK_DIR="$(cd "$(dirname "$0")/../rpo-live-inspect/rpo_key_hook" && pwd)"
HOOK_LIB="$HOOK_DIR/rpo_key_hook.so"
KEYS_FILE="/tmp/rpo_keys_export.json"

# Ensure hook is built
if [ ! -f "$HOOK_LIB" ]; then
    echo "[wrapper] Building key capture hook..."
    cd "$HOOK_DIR" && make
fi

# Export keys file path for hook (limpa execução anterior pra não
# confundir "nenhum evento novo" com "evento de uma rodada antiga")
export RPO_KEYS_OUTPUT="$KEYS_FILE"
rm -f "$KEYS_FILE"

echo "[wrapper] Starting compilation with key capture..."
echo "[wrapper] Hook: $HOOK_LIB"
echo "[wrapper] Keys output: $KEYS_FILE"
echo ""

# Run with LD_PRELOAD
LD_PRELOAD="$HOOK_LIB" "$@"

EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo ""
    echo "[wrapper] ✓ Compilation successful"
    if [ -f "$KEYS_FILE" ]; then
        echo "[wrapper] Captured keys:"
        cat "$KEYS_FILE"
    else
        echo "[wrapper] ⚠ No keys captured (hook may not have triggered)"
    fi
else
    echo ""
    echo "[wrapper] ✗ Compilation failed with exit code $EXIT_CODE"
fi

exit $EXIT_CODE
