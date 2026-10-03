#!/bin/sh
# Gera o .vsix da extensão AdvPL/TLPP com o compilador embutido pras 4
# plataformas suportadas (linux-x64, linux-arm64, win32-x64, darwin-arm64).
#
# Uso: tools/vscode-advpl/build-vsix.sh [VERSION]
# Sem VERSION, usa a versão do package.json. O advplc é compilado AQUI, do
# fonte atual, com as mesmas opções do workflow de release — antes o script
# copiava de dist/, que o alvo "make cross" deixou de atualizar, e o .vsix
# saía com um compilador antigo dentro.
set -e

cd "$(dirname "$0")"
ROOT="$(cd ../.. && pwd)"
PKGVER="$(sed -n 's/^[[:space:]]*"version": "\([^"]*\)".*/\1/p' package.json | head -1)"
VERSION="${1:-$PKGVER}"
VERSION="${VERSION#v}"
if [ "$VERSION" != "$PKGVER" ]; then
    echo "ERRO: versão pedida ($VERSION) difere do package.json ($PKGVER). Ajuste o package.json primeiro." >&2
    exit 1
fi

LDFLAGS="-s -w -X main.version=v$VERSION"
build() { # build GOOS GOARCH destino
    echo "  advplc $1/$2 -> $3"
    (cd "$ROOT" && CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -trimpath -ldflags "$LDFLAGS" \
        -o "$ROOT/tools/vscode-advpl/$3" ./cmd/advplc)
}

echo "Compilando advplc v$VERSION para as 4 plataformas..."
mkdir -p bin/linux-x64 bin/linux-arm64 bin/win32-x64 bin/darwin-arm64
build linux amd64 bin/linux-x64/advplc
build linux arm64 bin/linux-arm64/advplc
build windows amd64 bin/win32-x64/advplc.exe
build darwin arm64 bin/darwin-arm64/advplc
chmod +x bin/linux-x64/advplc bin/linux-arm64/advplc bin/darwin-arm64/advplc

# Cada binário tem de carregar a versão certa (vale para os 4, inclusive os
# que não rodam nesta máquina).
for f in bin/linux-x64/advplc bin/linux-arm64/advplc bin/win32-x64/advplc.exe bin/darwin-arm64/advplc; do
    if ! grep -aq "v$VERSION" "$f"; then
        echo "ERRO: $f não contém a versão v$VERSION" >&2
        exit 1
    fi
done

echo "Empacotando .vsix..."
if command -v vsce >/dev/null 2>&1; then
    vsce package
else
    npx --yes @vscode/vsce package
fi

VSIX="advpl-tlpp-advpp-$VERSION.vsix"
[ -f "$VSIX" ] || { echo "ERRO: $VSIX não foi gerado" >&2; exit 1; }
echo "Pronto: $VSIX (advplc v$VERSION nas 4 plataformas)"
