#!/bin/bash
# test_hook.sh — Testa o hook rpo_key_hook.so em condições controladas.
#
# Requisitos:
#   - container protheus-compile ativo (appsrvlinux em /protheus12/bin/appserver/)
#   - ou appsrvlinux em PATH
#
# Uso:
#   chmod +x test_hook.sh
#   ./test_hook.sh [caminho_do_appsrvlinux]
#
# Saída de interesse:
#   /tmp/rpo_keys_export.json  — chaves capturadas pelo hook
#   stdout/stderr              — logs de captura

set -euo pipefail

APPSRV="${1:-$(which appsrvlinux 2>/dev/null || echo '/protheus12/bin/appserver/appsrvlinux')}"
HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
SO="$HOOK_DIR/rpo_key_hook.so"
OUT="/tmp/rpo_keys_export.json"
TS=$(date +%Y%m%d-%H%M%S)
LOG="/tmp/rpo_key_hook_test_${TS}.log"

echo "=== rpo_key_hook — test hook ==="
echo "  SO:          $SO"
echo "  APPSRV:      $APPSRV"
echo "  SAIDA:       $OUT"
echo "  LOG:         $LOG"
echo ""

# Verifica pré-requisitos
if [ ! -f "$SO" ]; then
    echo "[FAIL] .so não encontrado. Execute 'make' primeiro." >&2
    exit 1
fi

if [ ! -x "$APPSRV" ]; then
    echo "[WARN] appsrvlinux não encontrado em $APPSRV" >&2
    echo "       Rodando teste de link apenas (simulação)..." >&2
    echo ""

    # Teste 1: biblioteca carrega e registra constructor
    echo "--- TESTE 1: dlopen + symbol check ---"
    tmp_prog=$(mktemp /tmp/rpo_hook_test_XXXXXX.cpp)
    cat > "$tmp_prog" <<'CPP'
#include <cstdio>
#include <dlfcn.h>
int main() {
    void* handle = dlopen("./rpo_key_hook.so", RTLD_NOW);
    if (!handle) { std::fprintf(stderr, "dlopen falhou: %s\n", dlerror()); return 1; }
    std::fprintf(stderr, "[OK] dlopen rpo_key_hook.so\n");
    // simbolos expostos pela nossa lib
    auto reg = (void(*)())dlsym(handle, "hook_register");
    if (reg) { std::fprintf(stderr, "[OK] constructor register encontrado\n"); reg(); }
    dlclose(handle);
    std::fprintf(stderr, "[OK] dlclosed OK\n");
    return 0;
}
CPP
    g++ -o /tmp/rpo_hook_test_bin "$tmp_prog" -ldl -Wl,-rpath,. 2>/dev/null
    /tmp/rpo_hook_test_bin 2>&1 | tee "$LOG"
    rm -f "$tmp_prog" /tmp/rpo_hook_test_bin
    echo ""
    echo "TESTE 1 PASSOU (simulação, sem appsrv real)"
    echo ""

    # Teste 2: lê JSON de exemplo (se existir)
    if [ -f "$OUT" ]; then
        echo "--- TESTE 2: verificar JSON exportado ---"
        python3 -c "
import json
with open('$OUT') as f:
    events = json.load(f)
print('Eventos:', len(events))
for e in events:
    if e['type'] == 'setkey':
        print('  [setkey] key=%s iv=%s' % (e.get('key'), e.get('iv')))
    elif e['type'] == 'evpinit':
        print('  [evpinit] cipher=%s key=%s iv=%s' % (e.get('cipher'), e.get('key'), e.get('iv')))
    elif e['type'] == 'encrypt':
        print('  [encrypt] plaintext=%s...' % (e.get('plaintext', '')[:32]))
" 2>&1 | tee -a "$LOG"
    else
        echo "[INFO] Nenhum $OUT ainda (nenhuma compilação RPO foi feita com o hook)"
    fi
    echo ""
    echo "=== FIM DO TESTE (modo simulação) ==="
    exit 0
fi

# Modo real: hooks + compilação de teste
echo "--- TESTE REAL: LD_PRELOAD + appsrvlinux ---"
rm -f "$OUT"
touch "$OUT"

# advplc/appsrvlinux exigem -env= e -includes= apontando pro diretório
# "apo" da instalação — sem isso o compile falha com "Invalid
# Environment" antes de tocar em qualquer cripto (nada pra capturar).
# Ajuste ENV/APO_DIR se sua instalação usar outro nome de ambiente
# (ex.: "environment" em builds 12.1.2510+, ver docs/MANUAL_ADVPLC.md).
ENV_NAME="${RPO_HOOK_ENV:-P12}"
APO_DIR="${RPO_HOOK_APODIR:-$(dirname "$APPSRV")/../../apo}"

# Compila um fonte minúsculo como teste, direto no diretório apo
# (-includes precisa resolvê-lo lá).
TMP_PRW="$APO_DIR/rpo_hook_test_$$.prw"
cat > "$TMP_PRW" <<'PRW'
#Include "totvs.ch"
User Function RPOHKTST()
    Local cMsg := "Hello RPO Hook Test " + DToS(Date())
    ConOut(cMsg)
Return .T.
PRW

echo "Fonte temporário: $TMP_PRW"
echo "Rodando (cwd=$(dirname "$APPSRV"), LD_LIBRARY_PATH=. — appsrvlinux exige isso pra achar suas próprias libs .so):"
echo "  LD_PRELOAD=$SO ./$(basename "$APPSRV") -compile -env=$ENV_NAME -files=$TMP_PRW -includes=$APO_DIR"
echo ""

(cd "$(dirname "$APPSRV")" && LD_LIBRARY_PATH=. LD_PRELOAD="$SO" "./$(basename "$APPSRV")" \
    -compile -env="$ENV_NAME" -files="$TMP_PRW" -includes="$APO_DIR" 2>&1) | tee "$LOG" || true

echo ""
echo "--- RESULTADO ---"
if [ -f "$OUT" ]; then
    echo "[OK] Arquivo de chaves gerado: $OUT"
    echo ""
    echo "Conteúdo:"
    cat "$OUT"
    echo ""
    echo ""
    echo "--- Validação JSON ---"
    python3 -c "
import json
with open('$OUT') as f:
    events = json.load(f)
print('Eventos capturados:', len(events))
by_type = {}
for e in events:
    by_type[e['type']] = by_type.get(e['type'], 0) + 1
print('Por tipo:', by_type)
for e in events:
    if e['type'] == 'evpinit':
        print('  [evpinit] cipher=%s key=%s iv=%s' % (e.get('cipher'), e.get('key'), e.get('iv')))
    elif e['type'] == 'rsakey':
        print('  [rsakey] password=%s' % e.get('password'))
print('[OK] JSON válido')
" 2>&1 | tee -a "$LOG"
else
    echo "[WARN] Nenhum arquivo de chaves gerado."
    echo "       Isso pode significar que tCryptoEVP::SetKey não foi chamado"
    echo "       durante esta compilação específica, ou que o symbol mangling"
    echo "       precisa ser ajustado para esta versão do binário."
    echo ""
    echo "       Symbols procurados (em libaplinux.so, não no appsrvlinux):"
    nm -D "$(dirname "$APPSRV")/libaplinux.so" 2>/dev/null | grep -i 'crypto\|EVP_Encrypt' | head -20 || true
fi

echo ""
echo "--- Log do hook (últimas 30 linhas) ---"
grep -i "rpo_key_hook" "$LOG" 2>/dev/null | tail -30 || true

echo ""
echo "=== FIM DO TESTE ==="
echo "Log completo: $LOG"
rm -f "$TMP_PRW"
