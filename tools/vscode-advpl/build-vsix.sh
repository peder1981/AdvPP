#!/bin/sh
# Gera o .vsix da extensão AdvPL/TLPP com o compilador embutido pras 4
# plataformas suportadas (linux-x64, linux-arm64, win32-x64, darwin-arm64).
#
# Uso: tools/vscode-advpl/build-vsix.sh [VERSION]
# (VERSION só entra no ldflags -X main.version do binário embutido; a
# versão da extensão em si é a de package.json.)
set -e

cd "$(dirname "$0")"
ROOT="$(cd ../.. && pwd)"
# O Makefile ja monta -X main.version=v$(VERSION), entao um "v" no argumento
# vira "vv2.0.7" no --version. Tira o prefixo se vier.
VERSION="${1:-dev}"
VERSION="${VERSION#v}"

echo "Cross-compilando advplc ($VERSION) para as 4 plataformas..."
(cd "$ROOT" && make cross VERSION="$VERSION")

mkdir -p bin/linux-x64 bin/linux-arm64 bin/win32-x64 bin/darwin-arm64
cp "$ROOT/dist/advplc-linux-amd64" bin/linux-x64/advplc
cp "$ROOT/dist/advplc-linux-arm64" bin/linux-arm64/advplc
cp "$ROOT/dist/advplc-windows-amd64.exe" bin/win32-x64/advplc.exe
cp "$ROOT/dist/advplc-darwin-arm64" bin/darwin-arm64/advplc
chmod +x bin/linux-x64/advplc bin/linux-arm64/advplc bin/darwin-arm64/advplc

echo "Empacotando .vsix..."
vsce package

# -t (mais recente primeiro) e nao ordem alfabetica: "2.0.8" vem depois de
# "2.0.14" numa comparacao de texto, entao a linha anunciava o .vsix errado.
echo "Pronto: $(ls -t *.vsix | head -1)"
