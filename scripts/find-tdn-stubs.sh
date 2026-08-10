#!/usr/bin/env bash
# scripts/find-tdn-stubs.sh
# Varre ~/tdn-advpl-mirror/ e lista toda página que os crawlers marcaram
# como stub genuíno do TDN (sem Sintaxe/Parâmetros/Retorno reais).
set -euo pipefail
MIRROR="$HOME/tdn-advpl-mirror"
OUT="docs/tdn-gap-stubs.md"

echo "# TDN — Gaps sem especificação real" > "$OUT"
echo "" >> "$OUT"
echo "Gerado por \`scripts/find-tdn-stubs.sh\` a partir de \`~/tdn-advpl-mirror/\`." >> "$OUT"
echo "Páginas do TDN sem corpo real (stub) — não implementar sem spec." >> "$OUT"
echo "" >> "$OUT"

find "$MIRROR" -name "*.md" | while read -r f; do
  rel="${f#"$MIRROR"/}"
  if grep -qi "stub\|página vazia\|sem conteúdo\|genuine.*empty\|nota:.*vazi" "$f" 2>/dev/null; then
    lines=$(wc -l < "$f")
    if [ "$lines" -lt 20 ]; then
      name=$(basename "$f" .md)
      cat_path=$(dirname "$rel")
      url=$(grep -m1 '^https://tdn.totvs.com' "$f" || echo "?")
      echo "- $name ($cat_path) — $url" >> "$OUT"
    fi
  fi
done
