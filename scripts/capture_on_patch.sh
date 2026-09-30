#!/bin/bash
# capture_on_patch.sh - Captura chaves durante aplicação de patches
#
# Uso:
#   ./scripts/capture_on_patch.sh <container> <patch_command>
#
# Exemplo:
#   ./scripts/capture_on_patch.sh protheus-compile-12.1.2510 "bash apply_patches.sh"

set -e

CONTAINER="${1:?Uso: $0 <container> <command>}"
PATCH_COMMAND="${2:?Uso: $0 <container> <command>}"

echo "═══════════════════════════════════════════════════════════════"
echo "  CAPTURA DE CHAVES DURANTE APLICAÇÃO DE PATCHES"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Container: $CONTAINER"
echo "Comando:   $PATCH_COMMAND"
echo ""

# Limpar captura anterior
docker exec $CONTAINER rm -f /tmp/rpo_keys_export.json /tmp/rpo_keys_patch.json

# Iniciar captura em background
echo "✓ Iniciando monitor de captura..."
docker exec $CONTAINER bash -c '
  cd /protheus12/bin/appserver &&
  export LD_LIBRARY_PATH=. &&
  LD_PRELOAD=/tmp/rpo_key_hook.so \
  bash -c "'"$PATCH_COMMAND"'"
' &
CAPTURE_PID=$!

# Aguardar execução
echo "⏳ Aguardando aplicação dos patches..."
wait $CAPTURE_PID 2>/dev/null || true

# Baixar captura
echo "✓ Baixando captura..."
docker cp $CONTAINER:/tmp/rpo_keys_export.json /tmp/rpo_keys_patch_$(date +%Y%m%d_%H%M%S).json 2>/dev/null || {
    echo "⚠️  Captura não encontrada. Verifique se o RPO foi recompilado."
    exit 1
}

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  CAPTURA CONCLUÍDA"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Arquivo: /tmp/rpo_keys_patch_*.json"
echo ""
echo "Próximo passo:"
echo "  go run ./cmd/advplc rpo decrypt /tmp/tttm120.rpo /tmp/rpo_keys_patch_*.json"
