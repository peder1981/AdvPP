#!/bin/bash
# Script para construir imagem Docker do Protheus

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTHEUS_SRC="/home/peder/TOTVS Linha - Protheus"
IMAGE_NAME="protheus-compile-custom"
IMAGE_TAG="12.1.2510-custom"

echo "=== Build Imagem Protheus Custom ==="
echo "Fonte: $PROTHEUS_SRC"
echo "Imagem: ${IMAGE_NAME}:${IMAGE_TAG}"

# Verificar se fonte existe
if [ ! -d "$PROTHEUS_SRC" ]; then
    echo "Erro: Diretório fonte não encontrado: $PROTHEUS_SRC"
    exit 1
fi

# Criar contexto de build temporário
BUILD_CTX="/tmp/protheus-build-$$"
mkdir -p "$BUILD_CTX/docker/protheus"
mkdir -p "$BUILD_CTX/docker/dbaccess"

# Copiar arquivos
echo "Copiando arquivos..."
cp -r "$PROTHEUS_SRC/protheus" "$BUILD_CTX/docker/protheus/"
cp -r "$PROTHEUS_SRC/dbaccess" "$BUILD_CTX/docker/dbaccess/"
cp "$SCRIPT_DIR/protheus/Dockerfile" "$BUILD_CTX/docker/"

# Construir imagem
echo "Construindo imagem..."
cd "$BUILD_CTX"
docker build \
    --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
    --build-arg VCS_REF=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -f docker/protheus/Dockerfile \
    .

# Limpar
rm -rf "$BUILD_CTX"

echo "✅ Imagem construída: ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
echo "Para usar:"
echo "  docker run -d --name protheus-custom ${IMAGE_NAME}:${IMAGE_TAG}"
echo "  docker exec protheus-custom bash -c 'cd /totvs/protheus12.1.2510/protheus/bin/appserver && ./appsrvlinux -daemon'"
