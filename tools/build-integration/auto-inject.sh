#!/bin/bash
# Workflow automatizado para injeção de bytecode em RPO
# Uso: ./auto-inject.sh [opções]

set -e

# Configurações
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_DIR"

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== AdvPP RPO Bytecode Injector ===${NC}"
echo ""

# Verificar se advplc está disponível
if ! command -v advplc &> /dev/null; then
    echo -e "${YELLOW}Construindo advplc...${NC}"
    go build -o advplc ./cmd/advplc/
fi

# Verificar argumentos
RPO_FILE=""
CAPTURE_FILE=""
BYTECODE_DIR="releases/bytecode"
OUTPUT_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --rpo) RPO_FILE="$2"; shift 2 ;;
        --captura) CAPTURE_FILE="$2"; shift 2 ;;
        --bytecode-dir) BYTECODE_DIR="$2"; shift 2 ;;
        --output) OUTPUT_FILE="$2"; shift 2 ;;
        --help)
            echo "Uso: ./auto-inject.sh [opções]"
            echo ""
            echo "Opções:"
            echo "  --rpo <arquivo.rpo>        Arquivo RPO de entrada"
            echo "  --captura <captura.json>   Arquivo de captura de chaves"
            echo "  --bytecode-dir <dir>       Diretório com bytecode (padrão: releases/bytecode)"
            echo "  --output <arquivo.rpo>     Arquivo de saída"
            echo "  --help                     Mostrar esta ajuda"
            exit 0
            ;;
        *) echo "Opção desconhecida: $1"; exit 1 ;;
    esac
done

# Validar
if [ -z "$RPO_FILE" ]; then
    echo -e "${RED}Erro: --rpo é obrigatório${NC}"
    exit 1
fi

if [ ! -f "$RPO_FILE" ]; then
    echo -e "${RED}Erro: RPO não encontrado: $RPO_FILE${NC}"
    exit 1
fi

if [ -z "$CAPTURE_FILE" ]; then
    echo -e "${RED}Erro: --captura é obrigatório${NC}"
    exit 1
fi

if [ ! -f "$CAPTURE_FILE" ]; then
    echo -e "${RED}Erro: Captura não encontrada: $CAPTURE_FILE${NC}"
    exit 1
fi

# Definir saída padrão
if [ -z "$OUTPUT_FILE" ]; then
    OUTPUT_FILE="${RPO_FILE%.rpo}_injected.rpo"
fi

echo -e "${GREEN}Passo 1: Analisando RPO...${NC}"
./advplc rpo info "$RPO_FILE"
echo ""

echo -e "${GREEN}Passo 2: Decodificando com captura...${NC}"
./advplc rpo inject "$RPO_FILE" "$CAPTURE_FILE" --list --verbose
echo ""

echo -e "${GREEN}Passo 3: Verificando bytecode disponível...${NC}"
BYTECODE_COUNT=$(ls "$BYTECODE_DIR"/*.bytecode 2>/dev/null | wc -l)
echo "  Encontrados: $BYTECODE_COUNT arquivos de bytecode"
echo ""

if [ "$BYTECODE_COUNT" -eq 0 ]; then
    echo -e "${YELLOW}Aviso: Nenhum arquivo .bytecode encontrado em $BYTECODE_DIR${NC}"
    echo "  Execute 'advplc compile *.prw -o output.bytecode' para gerar bytecode"
fi

echo -e "${GREEN}Passo 4: Preparando injeção...${NC}"
echo "  RPO: $RPO_FILE"
echo "  Output: $OUTPUT_FILE"
echo "  Bytecode dir: $BYTECODE_DIR"
echo ""

echo -e "${GREEN}✅ Workflow concluído!${NC}"
echo ""
echo "Próximos passos:"
echo "  1. Complete a implementação de recriptografia em pkg/rpo/injector/reencrypt.go"
echo "  2. Teste com um RPO real e captura ao vivo"
echo "  3. Execute: ./advplc rpo inject <rpo> <captura> --inject Funcao=bytecode.bytecode -o output.rpo"
