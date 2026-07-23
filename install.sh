#!/bin/sh
# Instala o advplc (compilador/CLI do AdvPP) a partir do último release no GitHub.
#
#   curl -fsSL https://raw.githubusercontent.com/peder1981/AdvPP/master/install.sh | sh
#
# Variáveis opcionais:
#   ADVPP_VERSION   versão específica (ex.: 1.21.0), padrão: última release
#   ADVPP_INSTALL_DIR  diretório de destino, padrão: $HOME/.local/bin (ou
#                      /usr/local/bin se rodando como root)
set -e

REPO="peder1981/AdvPP"

os=$(uname -s)
arch=$(uname -m)

case "$os" in
	Linux) goos="linux" ;;
	Darwin) goos="darwin" ;;
	*)
		echo "Sistema não suportado por este script: $os" >&2
		echo "Baixe manualmente em https://github.com/$REPO/releases" >&2
		exit 1
		;;
esac

case "$arch" in
	x86_64|amd64) goarch="amd64" ;;
	arm64|aarch64) goarch="arm64" ;;
	*)
		echo "Arquitetura não suportada por este script: $arch" >&2
		echo "Baixe manualmente em https://github.com/$REPO/releases" >&2
		exit 1
		;;
esac

# Só há build darwin-arm64 publicado (sem macOS Intel) e só linux tem arm64
# no pacote CLI puro-Go — mac amd64 e outras combinações caem aqui.
if [ "$goos" = "darwin" ] && [ "$goarch" = "amd64" ]; then
	echo "Não há build pronto para macOS Intel (amd64) ainda — só darwin-arm64 (Apple Silicon)." >&2
	echo "Compile a partir do fonte: https://github.com/$REPO#compilando-do-fonte" >&2
	exit 1
fi

if [ -z "$ADVPP_VERSION" ]; then
	version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed -E 's/.*"v([^"]+)"/\1/')
	if [ -z "$version" ]; then
		echo "Não consegui detectar a última versão automaticamente." >&2
		echo "Rode com ADVPP_VERSION=x.y.z explícito." >&2
		exit 1
	fi
else
	version="$ADVPP_VERSION"
fi

asset="advpp-cli-${version}-${goos}-${goarch}.tar.gz"
url="https://github.com/$REPO/releases/download/v${version}/${asset}"

if [ -z "$ADVPP_INSTALL_DIR" ]; then
	if [ "$(id -u)" = "0" ]; then
		install_dir="/usr/local/bin"
	else
		install_dir="$HOME/.local/bin"
	fi
else
	install_dir="$ADVPP_INSTALL_DIR"
fi

echo "Baixando advplc v$version ($goos/$goarch)..."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

if ! curl -fsSL "$url" -o "$tmp/$asset"; then
	echo "Falha ao baixar $url" >&2
	echo "Confira as versões disponíveis em https://github.com/$REPO/releases" >&2
	exit 1
fi

tar xzf "$tmp/$asset" -C "$tmp"
mkdir -p "$install_dir"
mv "$tmp/advplc" "$install_dir/advplc"
chmod +x "$install_dir/advplc"

echo "advplc instalado em $install_dir/advplc"

case ":$PATH:" in
	*":$install_dir:"*) ;;
	*)
		echo ""
		echo "$install_dir não está no seu PATH. Adicione ao seu shell rc:"
		echo "  export PATH=\"$install_dir:\$PATH\""
		;;
esac

echo ""
"$install_dir/advplc" version 2>/dev/null || "$install_dir/advplc" --version 2>/dev/null || true
echo "Pronto. Experimente: advplc check algum_fonte.prw"
