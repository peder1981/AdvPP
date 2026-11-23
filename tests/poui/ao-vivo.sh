#!/usr/bin/env bash
# Testes de navegador (Chromium headless via CDP) dos apps PO-UI.
#
#   ./tests/poui/ao-vivo.sh            fase1.prw — relógio do timer e digitação
#   ./tests/poui/ao-vivo.sh fixture-app  Task 2.7 — app Angular real:
#       tabela /rest/api/est/v1/armazens, "Pingar AdvPL" (pong via
#       JsToAdvpl/AdvPLToJS) e "Fechar" (callAppClose fecha a aba).
#   ./tests/poui/ao-vivo.sh servicos-3c   Onda 3c — mídia/storage/tema/guia.
#   ./tests/poui/ao-vivo.sh catalogo      Todas as classes Po*: cada uma vira o
#       elemento PO-UI certo, com conteúdo e console limpo; servido como
#       projeto (serve <dir> --main) junto do REST de armazéns da onda 1.
#
# Requer chromium e Node 22+ (WebSocket nativo). O modo fixture-app usa um
# HOME descartável com poui_apps_dirs -> tests/poui/apps e um SQLite com a
# fixture NNR/SX3 da onda 1 (25 armazéns).
set -euo pipefail
cd "$(dirname "$0")/../.."
D=$(mktemp -d)
trap 'kill $SRV $CHR 2>/dev/null || true; rm -rf "$D"' EXIT

CENARIO="${1:-}"
FONTE="examples/poui/fase1.prw"
PORTA=18899
DBARG=()
MAINARG=()

if [ "$CENARIO" = "fixture-app" ]; then
  FONTE="tests/poui/fixture-app.prw"
  PORTA=8098
elif [ "$CENARIO" = "servicos-3c" ]; then
  FONTE="tests/poui/3c_servicos.prw"
  PORTA=8097
elif [ "$CENARIO" = "catalogo" ]; then
  # Projeto (RPO): a tela e os fontes REST em arquivos separados.
  mkdir -p "$D/rpo"
  cp tests/poui/catalogo.prw tests/rest_totvs/armazens_repo.tlpp tests/rest_totvs/armazens_api.tlpp "$D/rpo/"
  FONTE="$D/rpo"
  PORTA=8096
  MAINARG=(--main U_Catalogo)
fi

# fixture-app e servicos-3c usam a mesma preparação: apps PO-UI de teste e
# SQLite com a fixture NNR/SX3 da onda 1 (25 armazéns).
if [ "$CENARIO" = "fixture-app" ] || [ "$CENARIO" = "servicos-3c" ] || [ "$CENARIO" = "catalogo" ]; then
  mkdir -p "$D/home/.advpp"
  cat > "$D/home/.advpp/advpp_config.json" <<EOF
{"poui_apps_dirs": ["$PWD/tests/poui/apps"]}
EOF
  sqlite3 "$D/t.db" < tests/rest_totvs/armazens_fixture.sql
  python3 - "$D/t.db" <<'PY'
import sqlite3, sys
c = sqlite3.connect(sys.argv[1])
for i in range(1, 26):
    c.execute("INSERT INTO NNR (NNR_FILIAL, NNR_CODIGO, NNR_DESCRI, NNR_ESTOQUE) VALUES ('01', ?, ?, ?)",
              ("%05d" % i, "Armazem %02d" % i, i * 1.5))
c.commit()
PY
  DBARG=(--db-path "$D/t.db")
fi

go build -o "$D/advplc" ./cmd/advplc
HOME="$D/home" "$D/advplc" serve "$FONTE" --port "$PORTA" "${DBARG[@]}" "${MAINARG[@]}" > "$D/serve.log" 2>&1 & SRV=$!
chromium --headless=new --no-sandbox --disable-gpu --disable-popup-blocking --remote-debugging-port=9399 --user-data-dir="$D/perfil" about:blank > /dev/null 2>&1 & CHR=$!
sleep 3
if [ "$CENARIO" = "catalogo" ]; then
  node tests/poui/catalogo.mjs 9399 "http://localhost:$PORTA/" || { echo "--- serve.log"; tail -30 "$D/serve.log"; exit 1; }
  exit 0
fi
node tests/poui/ao_vivo.mjs 9399 "http://localhost:$PORTA/" $CENARIO
