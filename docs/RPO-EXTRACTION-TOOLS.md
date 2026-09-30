> **Fonte autoritativa:** [`RPO-GROUND-TRUTH.md`](./RPO-GROUND-TRUTH.md).
> Este guia descreve as ferramentas; o ground-truth descreve o que é
> **confirmado / inferido / refutado**.

# RPO Extraction Tools

Ferramentas genéricas para análise e extração de metadata de RPOs Protheus sem necessidade de chaves de criptografia.

## Visão Geral

Estas ferramentas permitem analisar a estrutura interna dos RPOs, extrair strings, detectar padrões e calcular métricas de entropia - tudo offline, sem depender do appserver rodando.

## Ferramentas

### 1. CLI: `advplc rpo analyze`

Análise estrutural integrada ao compilador AdvPP.

```bash
# Análise básica
advplc rpo analyze custom.rpo

# Análise detalhada
advplc rpo analyze --verbose custom.rpo

# Saída mostra:
# - Metadados do RPO (nome, magic, tamanhos)
# - Distribuição de bytes
# - Entropia (bits/byte)
# - Strings legíveis
# - Padrões repetidos (com --verbose)
```

**Arquivo:** `cmd/advplc/cmd_rpo_analyze.go`

### 1b. CLI: `advplc rpo regions` (recomendado)

Classificação **honesta** do conteúdo em zero / ciphertext / mixed / plaintext
por entropia de Shannon. É a ferramenta que **não inventa estrutura**.

```bash
advplc rpo regions arquivo.rpo                # resumo + veredito
advplc rpo regions arquivo.rpo --top 5        # janelas de maior entropia
advplc rpo regions arquivo.rpo --strings      # strings só de janelas plaintext
advplc rpo regions arquivo.rpo --window 8192  # tamanho de janela
```

Saída típica de um RPO real: **100% ciphertext, 0% plaintext** → veredito
explícito de que não há estrutura legível sem captura de chave.

**Arquivo:** `cmd/advplc/cmd_rpo_regions.go` + `pkg/rpo/forensics.go`

### 2. Script Python: `rpo_extractor.py`

Ferramenta independente para análise completa.

```bash
# Análise básica
python3 tools/rpo-live-inspect/rpo_extractor.py custom.rpo

# Análise verbose
python3 tools/rpo-live-inspect/rpo_extractor.py custom.rpo --verbose

# Salvar relatório JSON
python3 tools/rpo-live-inspect/rpo_extractor.py custom.rpo --output ./reports/
```

**Saída JSON inclui:**
- `info`: Metadados do arquivo
- `encryption`: Detecção de criptografia (entropia, unique bytes)
- `byte_distribution`: Top 20 bytes mais frequentes
- `strings`: Strings legíveis encontradas
- `patterns`: Padrões repetidos
- `total_strings`: Contagem total

**Arquivo:** `tools/rpo-live-inspect/rpo_extractor.py`

### 3. Parser APO: `pkg/rpo/apo_parser.go`

Parser para escanear candidatos a registros APO (Advanced Program Object).

```go
parser := rpo.NewAPOParser(content)

// Escanear candidatos
candidates := parser.ScanForCandidates()

// Obter top candidatos
top := parser.GetTopCandidates(10)

// Parsear registros
records := parser.ParseRecords(0.5)

// Extrair funções
functions := parser.ExtractFunctions()
```

**Arquivo:** `pkg/rpo/apo_parser.go`

## Uso Prático

### Comparar RPOs entre versões

```bash
# Analisar dois RPOs e comparar
advplc rpo analyze custom_v12.1.2310.rpo > /tmp/rpo1.txt
advplc rpo analyze custom_v12.1.2510.rpo > /tmp/rpo2.txt

# Verificar diferenças
diff /tmp/rpo1.txt /tmp/rpo2.txt
```

### Gerar relatório completo

```bash
python3 tools/rpo-live-inspect/rpo_extractor.py custom.rpo \
  --verbose \
  --output ./reports/
  
# Relatório em: ./reports/custom_analysis.json
```

### Detectar mudanças estruturais

```bash
# Verificar se RPO foi modificado
md5sum custom.rpo
advplc rpo analyze custom.rpo | grep "Entropy"
```

## Métricas Importantes

| Métrica | Valor Esperado | Significado |
|---------|----------------|-------------|
| Entropia | ~8.0 bits/byte | Criptografia forte |
| Unique bytes | 256/256 | Distribuição uniforme |
| Strings | 100k+ | Metadata + nomes |
| Padrões | Baixo | Conteúdo criptografado |

## Limitações

- **Não extrai código fonte**: O conteúdo é criptografado com chaves efêmeras
- **Strings limitadas**: Apenas strings ASCII impressíveis são extraídas
- **Padrões**: Requer conteúdo não-criptografado para detecção

## Integração com Workflow

### Antes de compilar
```bash
# Capturar baseline do RPO atual
advplc rpo analyze custom.rpo > custom_baseline.txt
```

### Após patch
```bash
# Verificar impacto
advplc rpo analyze custom-patched.rpo > custom_after.txt
diff custom_baseline.txt custom_after.txt
```

### Para debugging
```bash
# Análise detalhada
python3 tools/rpo-live-inspect/rpo_extractor.py custom.rpo --verbose
```

## Arquivos Criados

```
cmd/advplc/cmd_rpo_analyze.go      # Comando CLI integrado
pkg/rpo/apo_parser.go              # Parser APO
tools/rpo-live-inspect/rpo_extractor.py  # Script Python
docs/RPO-EXTRACTION-TOOLS.md       # Esta documentação
```

## Próximos Passos

1. Integrar com sistema de versionamento de RPOs
2. Adicionar comparação automática entre versões
3. Criar alertas para mudanças estruturais
4. Expandir detecção de padrões específicos

---

*Criado em 2026-09-20 como parte do projeto AdvPP - Compilador AdvPL/TLPP*
