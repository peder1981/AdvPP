#!/bin/bash
# Script para injetar bytecode em RPO do Protheus
# Uso: ./inject-bytecode.sh [opções]

set -e

# Configurações padrão
RPO_FILE=""
CAPTURE_FILE=""
BYTECODE_DIR="releases/bytecode"
OUTPUT_FILE=""
LIST_ONLY=false
VERBOSE=false

# Função de help
show_help() {
    cat << 'HELP'
Uso: inject-bytecode.sh [opções]

Opções:
  --rpo <arquivo.rpo>          Arquivo RPO de entrada
  --captura <captura.json>     Arquivo de captura de chaves
  --bytecode-dir <diretório>   Diretório com bytecode (padrão: releases/bytecode)
  --output <arquivo.rpo>       Arquivo de saída
  --list                       Apenas listar registros APO
  --verbose                    Mostrar detalhes
  --help                       Mostrar esta ajuda

Exemplo:
  ./inject-bytecode.sh --rpo custom.rpo --captura keys.json --list
  ./inject-bytecode.sh --rpo custom.rpo --captura keys.json --output novo.rpo
HELP
}

# Parsear argumentos
while [[ $# -gt 0 ]]; do
    case $1 in
        --rpo)
            RPO_FILE="$2"
            shift 2
            ;;
        --captura)
            CAPTURE_FILE="$2"
            shift 2
            ;;
        --bytecode-dir)
            BYTECODE_DIR="$2"
            shift 2
            ;;
        --output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        --list)
            LIST_ONLY=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            echo "Erro: opção desconhecida $1"
            show_help
            exit 1
            ;;
    esac
done

# Validar argumentos
if [ -z "$RPO_FILE" ]; then
    echo "Erro: arquivo RPO não especificado"
    show_help
    exit 1
fi

if [ ! -f "$RPO_FILE" ]; then
    echo "Erro: arquivo RPO não encontrado: $RPO_FILE"
    exit 1
fi

if [ -z "$CAPTURE_FILE" ]; then
    echo "Erro: arquivo de captura não especificado"
    show_help
    exit 1
fi

if [ ! -f "$CAPTURE_FILE" ]; then
    echo "Erro: arquivo de captura não encontrado: $CAPTURE_FILE"
    exit 1
fi

# Definir arquivo de saída padrão
if [ -z "$OUTPUT_FILE" ]; then
    OUTPUT_FILE="${RPO_FILE%.rpo}_injected.rpo"
fi

echo "=== Injetor de Bytecode RPO ==="
echo "RPO: $RPO_FILE"
echo "Captura: $CAPTURE_FILE"
echo "Bytecode Dir: $BYTECODE_DIR"
echo "Output: $OUTPUT_FILE"
echo "List Only: $LIST_ONLY"
echo "Verbose: $VERBOSE"
echo ""

# Verificar se advplc está disponível
if ! command -v advplc &> /dev/null; then
    echo "Error: advplc não encontrado no PATH"
    echo "Construindo advplc..."
    cd "$(dirname "$0")/../.."
    go build -o advplc ./cmd/advplc/
fi

# Executar injeção
cd "$(dirname "$0")/../.."

if [ "$LIST_ONLY" = true ]; then
    echo "=== Listando registros APO ==="
    ./advplc rpo inject "$RPO_FILE" "$CAPTURE_FILE" --list --verbose
else
    echo "=== Injetando bytecode ==="
    # Primeiro listar
    ./advplc rpo inject "$RPO_FILE" "$CAPTURE_FILE" --list --verbose
    
    echo ""
    echo "=== Processando bytecode ==="
    
    # Processar cada arquivo .bytecode no diretório
    INJECT_ARGS=""
    for bytecode_file in "$BYTECODE_DIR"/*.bytecode; do
        if [ -f "$bytecode_file" ]; then
            func_name=$(basename "$bytecode_file" .bytecode)
            echo "  Processando: $func_name"
            INJECT_ARGS="$INJECT_ARGS --inject ${func_name}=${bytecode_file}"
        fi
    done
    
    if [ -n "$INJECT_ARGS" ]; then
        echo ""
        echo "=== Executando injeção ==="
        ./advplc rpo inject "$RPO_FILE" "$CAPTURE_FILE" $INJECT_ARGS --out "$OUTPUT_FILE"
    else
        echo "Nenhum arquivo .bytecode encontrado em $BYTECODE_DIR"
    fi
fi

echo ""
echo "✅ Conclusão"
echo "   Arquivo de saída: $OUTPUT_FILE"
