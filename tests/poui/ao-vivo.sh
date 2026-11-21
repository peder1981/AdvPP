#!/usr/bin/env bash
# Sobe examples/poui/fase1.prw e confere no Chromium (headless) que o relógio
# atualizado por timer muda na tela sozinho. Requer chromium e Node 22+.
set -euo pipefail
cd "$(dirname "$0")/../.."
D=$(mktemp -d)
trap 'kill $SRV $CHR 2>/dev/null || true; rm -rf "$D"' EXIT
go build -o "$D/advplc" ./cmd/advplc
"$D/advplc" serve examples/poui/fase1.prw --port 18899 > "$D/serve.log" 2>&1 & SRV=$!
chromium --headless=new --no-sandbox --disable-gpu --remote-debugging-port=9399 --user-data-dir="$D/perfil" about:blank > /dev/null 2>&1 & CHR=$!
sleep 3
node tests/poui/ao_vivo.mjs 9399 http://localhost:18899/
