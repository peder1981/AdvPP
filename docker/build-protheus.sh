#!/bin/bash
set -e

PROTHEUS_SRC="/home/peder/TOTVS Linha - Protheus"
DOCKER_CTX="/tmp/protheus-docker-ctx"
IMAGE_NAME="protheus-compile-custom"
IMAGE_TAG="12.1.2510-custom"

echo "=== Preparando contexto de build ==="
rm -rf "$DOCKER_CTX"
mkdir -p "$DOCKER_CTX"

# Copiar arquivos do Protheus
echo "Copiando Protheus..."
cp -r "$PROTHEUS_SRC/protheus" "$DOCKER_CTX/"

# Copiar scripts Docker
echo "Copiando scripts..."
mkdir -p "$DOCKER_CTX/docker/protheus"
mkdir -p "$DOCKER_CTX/docker/scripts"
cp "/home/peder/Projetos/AdvPP-unstable/docker/protheus/Dockerfile" "$DOCKER_CTX/docker/protheus/"
cp "/home/peder/Projetos/AdvPP-unstable/docker/scripts/entrypoint.sh" "$DOCKER_CTX/docker/scripts/"

# Construir
echo "Construindo imagem..."
cd "$DOCKER_CTX"
docker build \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -f docker/protheus/Dockerfile \
    .

# Limpar
rm -rf "$DOCKER_CTX"

echo ""
echo "✅ Imagem construída: ${IMAGE_NAME}:${IMAGE_TAG}"
