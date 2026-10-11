#!/bin/bash
# Script de automação para captura e injeção de bytecode em RPOs
# Uso: ./capture_and_inject.sh <container> <rpo_path> <source.prw> [output.rpo]

set -e

CONTAINER="${1:?Uso: $0 <container> <rpo_path> <source.prw> [output.rpo]}"
RPO_PATH="${2:?Uso: $0 <container> <rpo_path> <source.prw> [output.rpo]}"
SOURCE="${3:?Uso: $0 <container> <rpo_path> <source.prw> [output.rpo]}"
OUTPUT="${4:-/tmp/injected.rpo}"

HOOK_V10="/tmp/rpo_key_hook_v10.so"
HOOK_V11="/tmp/rpo_key_hook_v11.so"
CAPTURE_FILE="/tmp/capture_$(date +%Y%m%d_%H%M%S).json"

echo "=== RPO Bytecode Injector - Auto Capture ==="
echo "Container: $CONTAINER"
echo "RPO: $RPO_PATH"
echo "Source: $SOURCE"
echo "Output: $OUTPUT"
echo ""

# Verificar hooks
echo "Verificando hooks..."
docker exec "$CONTAINER" ls -lh /tmp/rpo_key_hook*.so 2>/dev/null || {
    echo "Copiando hooks para o container..."
    docker cp "/home/peder/Projetos/AdvPP-unstable/tools/rpo-live-inspect/rpo_key_hook/rpo_key_hook_v10.so" "$CONTAINER:/tmp/rpo_key_hook_v10.so"
    docker cp "/home/peder/Projetos/AdvPP-unstable/tools/rpo-live-inspect/rpo_key_hook/rpo_key_hook_v11.so" "$CONTAINER:/tmp/rpo_key_hook_v11.so"
}

# Compilar com hook v10 (melhor para cipher name)
echo ""
echo "Compilando com hook v10..."
docker exec "$CONTAINER" bash -c "
export LD_PRELOAD=/tmp/rpo_key_hook_v10.so
export RPO_KEYS_OUTPUT=$CAPTURE_FILE
cd /totvs/protheus12.1.2510/bin
./appsrvlinux -compile -env=P12 -files=$SOURCE 2>&1
" || {
    echo "Erro na compilação. Tentando com hook v11..."
    docker exec "$CONTAINER" bash -c "
    export LD_PRELOAD=/tmp/rpo_key_hook_v11.so
    export RPO_KEYS_OUTPUT=$CAPTURE_FILE
    cd /totvs/protheus12.1.2510/bin
    ./appsrvlinux -compile -env=P12 -files=$SOURCE 2>&1
    "
}

# Copiar captura
echo ""
echo "Copiando captura..."
docker cp "$CONTAINER:$CAPTURE_FILE" /tmp/capture_latest.json

# Verificar captura
echo ""
echo "Verificando captura..."
python3 << 'PYEOF'
import json
import sys

try:
    with open('/tmp/capture_latest.json', 'r') as f:
        data = json.load(f)
    
    print(f"Total de eventos: {len(data)}")
    
    # Contar tipos
    types = {}
    for evt in data:
        t = evt.get('type', 'unknown')
        types[t] = types.get(t, 0) + 1
    
    print("Tipos de eventos:")
    for t, count in sorted(types.items()):
        print(f"  {t}: {count}")
    
    # Verificar segmentos completos
    by_n = {}
    for evt in data:
        n = evt.get('n', 0)
        if n not in by_n:
            by_n[n] = {}
        by_n[n][evt.get('type', '')] = evt
    
    complete = sum(1 for s in by_n.values() if 'encrypt' in s and ('evpinit' in s or 'setkey' in s))
    print(f"\nSegmentos completos: {complete}/{len(by_n)}")
    
    if complete > 0:
        print("\n✅ Captura válida para uso!")
        sys.exit(0)
    else:
        print("\n⚠️  Captura sem segmentos completos (cipher não identificado)")
        print("   Usar heurística para identificação...")
        sys.exit(1)
except Exception as e:
    print(f"Erro ao verificar captura: {e}")
    sys.exit(1)
PYEOF

# Copiar RPO
echo ""
echo "Copiando RPO..."
docker cp "$CONTAINER:$RPO_PATH" /tmp/rpo_source.rpo

# Executar injeção
echo ""
echo "Executando injeção..."
cd /home/peder/Projetos/AdvPP-unstable
./advplc rpo inject /tmp/rpo_source.rpo /tmp/capture_latest.json \
    --identify \
    --list 2>&1 || echo "Nota: Use --inject para especificar bytecode"

echo ""
echo "=== Concluído ==="
echo "Captura: /tmp/capture_latest.json"
echo "RPO original: /tmp/rpo_source.rpo"
echo "Output: $OUTPUT"
