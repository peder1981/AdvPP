#!/bin/bash
# rpo-extract.sh - Extrai fontes de RPOs Protheus
#
# Uso:
#   ./scripts/rpo-extract.sh <rpo_file> <capture_json> [output_dir]
#
# Exemplo:
#   ./scripts/rpo-extract.sh /tmp/tlpp.rpo /tmp/capture.json /tmp/output/

set -e

RPO_FILE="${1:?Uso: $0 <rpo_file> <capture_json> [output_dir]}"
CAPTURE_JSON="${2:?Uso: $0 <rpo_file> <capture_json> [output_dir]}"
OUTPUT_DIR="${3:-/tmp/rpo_output}"

echo "═══════════════════════════════════════════════════════════════"
echo "  RPO EXTRACTOR - AdvPP Compiler"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "RPO:      $RPO_FILE"
echo "Capture:  $CAPTURE_JSON"
echo "Output:   $OUTPUT_DIR"
echo ""

# Verificar arquivos
if [ ! -f "$RPO_FILE" ]; then
    echo "ERROR: RPO file not found: $RPO_FILE"
    exit 1
fi

if [ ! -f "$CAPTURE_JSON" ]; then
    echo "ERROR: Capture file not found: $CAPTURE_JSON"
    exit 1
fi

# Criar diretório de saída
mkdir -p "$OUTPUT_DIR"

# Analisar RPO
echo "✓ Analisando RPO..."
go run ./cmd/advplc rpo info "$RPO_FILE" 2>&1 | tee "$OUTPUT_DIR/rpo_info.txt"

# Decompor RPO
echo "✓ Decompondo RPO..."
go run ./cmd/advplc rpo decompose "$RPO_FILE" "$OUTPUT_DIR" 2>&1 | tee -a "$OUTPUT_DIR/rpo_info.txt"

# Decodificar
echo "✓ Decodificando..."
go run ./cmd/advplc rpo decrypt "$RPO_FILE" "$CAPTURE_JSON" 2>&1 | tee -a "$OUTPUT_DIR/decrypt.log"

# Analisar captura
echo "✓ Analisando captura..."
python3 << PYEOF
import json
with open('$CAPTURE_JSON') as f:
    data = json.load(f)
print(f'Total events: {len(data)}')
types = {}
for e in data:
    t = e.get('type', 'unknown')
    types[t] = types.get(t, 0) + 1
print(f'By type: {types}')
keys = set()
for e in data:
    if e.get('type') == 'setkey' and 'key' in e:
        keys.add(e['key'])
print(f'Unique keys: {len(keys)}')
with open('$OUTPUT_DIR/capture_analysis.json', 'w') as f:
    json.dump({'total': len(data), 'types': types, 'keys': list(keys)}, f, indent=2)
PYEOF

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  CONCLUÍDO"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Output files:"
ls -la "$OUTPUT_DIR/"
echo ""
echo "Próximo passo: Verificar decrypt.log para resultados"
