#!/bin/bash
# extract_rpo_sources.sh - Extrai fontes de RPOs Protheus
#
# Uso:
#   ./scripts/extract_rpo_sources.sh <container> <rpo_type> [source_file]
#
# Exemplos:
#   ./scripts/extract_rpo_sources.sh protheus-compile-12.1.2510 custom
#   ./scripts/extract_rpo_sources.sh protheus-compile-12.1.2510 tlpp
#   ./scripts/extract_rpo_sources.sh protheus-compile-12.1.2510 custom RPOMULTI.prw

set -e

CONTAINER="${1:-protheus-compile-12.1.2510}"
RPO_TYPE="${2:-custom}"
SOURCE_FILE="${3:-RPOMULTI.prw}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TOOLS_DIR="$PROJECT_DIR/tools/rpo-live-inspect"

# Paths
case $RPO_TYPE in
    custom)
        RPO_PATH="/tmp/${RPO_TYPE}-extracted.rpo"
        ;;
    tlpp)
        RPO_PATH="/tmp/tlpp-extracted.rpo"
        ;;
    tttm120)
        RPO_PATH="/tmp/tttm120-extracted.rpo"
        ;;
    *)
        echo "Erro: tipo de RPO desconhecido: $RPO_TYPE"
        echo "Tipos válidos: custom, tlpp, tttm120"
        exit 1
        ;;
esac

CAPTURE_PATH="/tmp/rpo_keys_${RPO_TYPE}.json"
OUTPUT_DIR="/tmp/rpo_output_${RPO_TYPE}"

echo "═══════════════════════════════════════════════════════════════"
echo "  EXTRAÇÃO DE FONTES RPO — Frente 1"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Container:  $CONTAINER"
echo "RPO Type:   $RPO_TYPE"
echo "Source:     $SOURCE_FILE"
echo "Capture:    $CAPTURE_PATH"
echo "Output:     $OUTPUT_DIR"
echo ""

# Step 1: Copiar RPO para o container
echo "✓ Passo 1: Preparando ambiente no container..."
docker cp "$RPO_PATH" "$CONTAINER:/tmp/target.rpo" 2>/dev/null || echo "  RPO local não encontrado, usando path remoto"

# Step 2: Executar compilação com hook
echo "✓ Passo 2: Capturando chaves em runtime..."
docker exec "$CONTAINER" sh -c "
  cd /protheus12/bin/appserver &&
  export LD_LIBRARY_PATH=. &&
  rm -f /tmp/rpo_keys_export.json &&
  LD_PRELOAD=/tmp/rpo_key_hook/rpo_key_hook.so \
  ./appsrvlinux -compile \
    -files=/totvs/protheus1212510/protheus/apo/$SOURCE_FILE \
    -includes=/totvs/protheus1212510/protheus/apo \
    -env=environment 2>&1 | grep -E 'SetKey|Error|Success' || true
" || echo "  Aviso: Compilação pode ter falhado (RPO já carregado)"

# Step 3: Baixar captura
echo "✓ Passo 3: Baixando captura..."
docker cp "$CONTAINER:/tmp/rpo_keys_export.json" "$CAPTURE_PATH" 2>/dev/null || {
    echo "  Erro: Não foi possível capturar chaves"
    echo "  Isso pode significar que o RPO não está sendo carregado"
    exit 1
}

# Step 4: Analisar captura
echo "✓ Passo 4: Analisando captura..."
mkdir -p "$OUTPUT_DIR"
python3 "$TOOLS_DIR/extract_rpo_sources.py" \
    "$RPO_PATH" "$CAPTURE_PATH" "$OUTPUT_DIR" 2>/dev/null || \
python3 -c "
import json
with open('$CAPTURE_PATH') as f:
    data = json.load(f)
print(f'Eventos total: {len(data)}')
types = {}
for e in data:
    t = e.get('type', 'unknown')
    types[t] = types.get(t, 0) + 1
print(f'Tipos: {types}')
"

# Step 5: Decodificar
echo "✓ Passo 5: Decodificando RPO..."
go run ./cmd/advplc rpo decrypt "$RPO_PATH" "$CAPTURE_PATH" 2>&1 | head -50

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  CONCLUÍDO"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Arquivos gerados:"
echo "  - $CAPTURE_PATH (captura de chaves)"
echo "  - $OUTPUT_DIR/ (análise)"
echo ""
echo "Próximo passo: Verificar se houve matches na decodificação"
