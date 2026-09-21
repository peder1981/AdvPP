#!/bin/bash
# Script para retornar ao contexto unstable
set -e

echo "═══════════════════════════════════════════════════════════════"
echo "  RETORNANDO AO CONTEXTO UNSTABLE"
echo "═══════════════════════════════════════════════════════════════"
echo ""

cd /home/peder/Projetos/AdvPP-unstable
git checkout unstable 2>/dev/null || true

echo "📍 Diretório: $(pwd)"
echo "📍 Branch: $(git branch --show-current)"
echo "📍 Commit: $(git rev-parse --short HEAD)"
echo ""
echo "📄 Contexto:"
head -40 CONTEXT.md
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  Para ver mais detalhes:"
echo "    cat CONTEXT.md"
echo "    cat git-status.txt"
echo "═══════════════════════════════════════════════════════════════"
