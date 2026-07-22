#!/usr/bin/env bash
# Monta o corpus de codigo para o dev_nn: concatena todos os fontes AdvPL/TLPP do
# repo (exceto binarios) em tests/llm/code_corpus.txt. Rode da raiz do projeto.
set -euo pipefail
cd "$(dirname "$0")/../.."   # raiz do projeto
OUT="tests/llm/code_corpus.txt"
: > "$OUT"
find . -type f \( -name '*.prw' -o -name '*.tlpp' -o -name '*.prg' -o -name '*.ch' \) \
    | grep -v '/bin/' \
    | sort \
    | while read -r f; do
        cat "$f" >> "$OUT"
        printf '\n\n' >> "$OUT"
    done
echo "code_corpus.txt: $(wc -c < "$OUT") bytes, $(find . -type f \( -name '*.prw' -o -name '*.tlpp' -o -name '*.prg' -o -name '*.ch' \) | grep -vc '/bin/') fontes"
