#!/bin/bash
# capture-keys.sh — Captura chaves/cifras reais durante compilação Protheus
# via LD_PRELOAD do rpo_key_hook.so (ver
# tools/rpo-live-inspect/rpo_key_hook/rpo_key_hook.cpp).
#
# Uso:
#   source capture-keys.sh
#   LD_PRELOAD="$HOOK_PATH" appsrvlinux -compile -env=... -files=... -includes=...
#   wait_for_keys        # opcional, bloqueia até o arquivo ter eventos
#
# [!] CORREÇÃO (2026-09-19): esta versão apontava para um arquivo
# ("/tmp/rpo_keys_latest.json") DIFERENTE do que o hook realmente
# escreve ("/tmp/rpo_keys_export.json" — ver rpo_key_hook.cpp) e usava
# o schema antigo (objeto único com campo "keys", "type"/"keylen"/
# "cipherlen"/"cipher_id"). O hook corrigido escreve uma LISTA de
# eventos ("type": "setkey"|"evpinit"|"encrypt", campos "cipher"/"key"/
# "iv"/"plaintext") — ver pkg/rpo/cipher_dispatch_test.go pro formato
# exato consumido pelo compilador Go.

set -e

HOOK_PATH="$(cd "$(dirname "$0")/../rpo-live-inspect/rpo_key_hook" && pwd)/rpo_key_hook.so"
KEYS_OUTPUT="/tmp/rpo_keys_export.json"

# Compila o hook se ainda não existir (nunca comitado — ver .gitignore).
if [ ! -f "$HOOK_PATH" ]; then
    echo "[capture-keys] Hook não encontrado, compilando..."
    (cd "$(dirname "$0")/../rpo-live-inspect/rpo_key_hook" && make)
fi

rm -f "$KEYS_OUTPUT"

# Espera o hook escrever pelo menos um evento (ele mesmo cria o
# arquivo — não pré-criamos um placeholder aqui, pra não mascarar o
# caso de a compilação nunca chamar nenhuma cifra).
wait_for_keys() {
    local timeout=${1:-60}
    local elapsed=0
    echo "[capture-keys] Aguardando eventos em $KEYS_OUTPUT (timeout: ${timeout}s)..."
    while [ $elapsed -lt $timeout ]; do
        if [ -s "$KEYS_OUTPUT" ]; then
            local n
            n=$(python3 -c "import json; print(len(json.load(open('$KEYS_OUTPUT'))))" 2>/dev/null || echo 0)
            if [ "$n" -gt 0 ] 2>/dev/null; then
                echo "[capture-keys] ✓ $n eventos capturados"
                return 0
            fi
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done
    echo "[capture-keys] ✗ Timeout — nenhum evento capturado (o hook nunca chamou tCryptoEVP::SetKey/EVP_EncryptInit_ex?)"
    return 1
}

# get_key <cipher-name-substring>: devolve a chave (hex) do primeiro
# evento "evpinit" cujo campo "cipher" contém o filtro (vazio = qualquer).
get_key() {
    local filter="${1:-}"
    [ -f "$KEYS_OUTPUT" ] || return 1
    python3 -c "
import json
events = json.load(open('$KEYS_OUTPUT'))
for e in events:
    if e.get('type') == 'evpinit' and '$filter' in (e.get('cipher') or ''):
        print(e.get('key', ''))
        break
"
}

export HOOK_PATH KEYS_OUTPUT
export -f wait_for_keys get_key

echo "[capture-keys] Hook: $HOOK_PATH"
echo "[capture-keys] Saída: $KEYS_OUTPUT"
