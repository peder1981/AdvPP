#!/bin/bash
# Script de deploy de fontes AdvPL no RPO custom
# Uso: ./tools/deploy-rpo.sh [container] [ambiente]

CONTAINER="${1:-protheus-tir-test}"
ENV="${2:-TM1}"
SRC_DIR="/home/peder/Projetos/AdvPP-unstable/src/functions"
HOOK_DIR="/home/peder/Projetos/AdvPP-unstable/tools/rpo-live-inspect/rpo_key_hook"
KEYS_FILE="/tmp/rpo_keys_export.json"

echo "============================================================"
echo "  AdvPP RPO Deploy Script"
echo "============================================================"
echo ""
echo "Container: $CONTAINER"
echo "Ambiente: $ENV"
echo "Fonte: $SRC_DIR"
echo ""

# Verificar se o container existe
if ! docker ps -a | grep -q "$CONTAINER"; then
    echo "[ERROR] Container $CONTAINER nao encontrado"
    exit 1
fi

# Verificar se ha fontes para compilar
SRC_COUNT=$(find $SRC_DIR -name "*.prw" -type f 2>/dev/null | wc -l)
if [ "$SRC_COUNT" -eq 0 ]; then
    echo "[ERROR] Nenhuma fonte .prw encontrada em $SRC_DIR"
    exit 1
fi

echo "[INFO] Encontradas $SRC_COUNT fontes para compilar"
echo ""

# Passo 1: Copiar fontes para o container
echo "[PASSO 1] Copiando fontes para o container..."
docker cp $SRC_DIR $CONTAINER:/tmp/sources 2>&1
if [ $? -ne 0 ]; then
    echo "[ERROR] Falha ao copiar fontes para o container"
    exit 1
fi
echo "[OK] Fontes copiadas"
echo ""

# Passo 2: Compilar com hook de captura de chaves
echo "[PASSO 2] Compilando fontes com capturade chaves..."
docker exec $CONTAINER bash -c "
export LD_LIBRARY_PATH=/totvs/protheus1212510/protheus/bin/appserver:/tmp
export LD_PRELOAD=/tmp/rpo_key_hook.so
export RPO_KEYS_OUTPUT=$KEYS_FILE
rm -f $KEYS_FILE

cd /totvs/protheus1212510/protheus/bin/appserver
./appsrvlinux -compile \
  -files=/tmp/sources/*.prw \
  -includes=/tmp/sources \
  -env=$ENV \
  -nodelfiles \
  -outreport=/tmp/compile_report.log \
  2>&1
" 2>&1

if [ $? -ne 0 ]; then
    echo "[WARNING] Compilacao falhou ou foi interrompida"
    echo "[INFO] Verifique os logs em /tmp/compile_report.log"
fi

# Passo 3: Verificar se ha chaves capturadas
echo ""
echo "[PASSO 3] Verificando chaves capturadas..."
if docker exec $CONTAINER test -s $KEYS_FILE 2>/dev/null; then
    echo "[OK] Chaves capturadas com sucesso"
    docker exec $CONTAINER cat $KEYS_FILE | head -50
else
    echo "[WARNING] Nenhuma chave capturada (compilacao pode ter falhado)"
fi

# Passo 4: Verificar custom.rpo
echo ""
echo "[PASSO 4] Verificando custom.rpo..."
docker exec $CONTAINER ls -lh /totvs/protheus1212510/protheus/bin/appserver/custom.rpo 2>/dev/null
docker exec $CONTAINER ls -lh /totvs/protheus1212510/protheus/apo/custom.rpo 2>/dev/null

# Passo 5: Copiar custom.rpo atualizado para o volume
echo ""
echo "[PASSO 5] Atualizando volume..."
docker run --rm \
  -v protheus12_apo:/dest:rw \
  -v $CONTAINER:/src:ro \
  alpine sh -c "cp /src/totvs/protheus1212510/protheus/apo/custom.rpo /dest/custom.rpo 2>/dev/null && echo 'custom.rpo atualizado'" 2>&1 || echo "[INFO] custom.rpo pode nao ter sido atualizado"

echo ""
echo "============================================================"
echo "  Deploy concluido!"
echo "============================================================"
echo ""
echo "Proximos passos:"
echo "1. Reiniciar o container: docker restart $CONTAINER"
echo "2. Verificar logs: docker logs $CONTAINER"
echo "3. Executar testes TIR: cd tests/tir && python3 SA1TESTSUITE.py"
echo ""
