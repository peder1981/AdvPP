# RPO Extraction - Extração de Fontes Protheus

## 🎯 Objetivo

Extrair código fonte compilado em RPOs (Repositório de Programas Objeto) do Protheus.

## ⚡ Quick Start

### 1. Capturar Chaves em Runtime

```bash
# No container com appserver rodando
docker exec protheus-compile-12.1.2510 bash -c '
  cd /protheus12/bin/appserver &&
  export LD_LIBRARY_PATH=. &&
  LD_PRELOAD=/tmp/rpo_key_hook.so \
  ./appsrvlinux -compile \
    -files=/caminho/para/fonte.prw \
    -includes=/caminho/para/includes \
    -env=ambiente
'
```

### 2. Baixar Captura

```bash
docker cp protheus-compile-12.1.2510:/tmp/rpo_keys_export.json /tmp/
```

### 3. Decodificar RPO

```bash
# Usar CLI
go run ./cmd/advplc rpo decrypt <rpo_file> <capture.json>

# Ou script automatizado
./scripts/rpo-extract.sh <rpo> <capture.json> [output_dir]
```

## 📊 RPOs Suportados

| RPO | Tamanho | Magic | Versões |
|-----|---------|-------|---------|
| `custom.rpo` | ~1MB | APNSRM0419 | 12.x |
| `tlpp.rpo` | ~4MB | APNSRM0420 | 12.x (idêntico 2310↔2510) |
| `tttm120.rpo` | ~379MB | APNSRM0421 | 12.x (idêntico 2310↔2510) |

## 🔧 Componentes

### Parser RPO (`pkg/rpo/`)
- `rpo.go` — Parser container
- `cipher_dispatch.go` — 11 cifras implementadas
- `extract.go` — Extração de funções
- `apo_parser.go` — Parser APO records

### Hook LD_PRELOAD (`tools/rpo-live-inspect/`)
- Captura chaves SetKey
- Captura plaintext EncryptUpdate
- Gera JSON de captura

### CLI (`cmd/advplc/`)
```bash
advplc rpo info <rpo>      # Metadados
advplc rpo identify <rpo>  # Identificar tipo
advplc rpo decompose <rpo> <dir>  # Decompor
advplc rpo decrypt <rpo> <capture>  # Decodificar
```

## 🧪 Testes

```bash
# Testes unitários
go test ./pkg/rpo/ -v

# Teste com RPO real
go test ./pkg/rpo/ -run TestDecryptRealRPO -v
```

## 📚 Documentação

- `docs/rpo-format.md` — Especificação completa (85KB)
- `docs/RPO-EXTRACTION-TWO-FRONT.md` — Relatório técnico
- `docs/RPO-EXTRACTION-INTEGRATION.md` — Guia de integração
- `docs/RPO-DECRYPTION-FINAL-REPORT.md` — Relatório final

## ⚠️ Limitações

1. **Chaves efêmeras** — Requer captura no momento da compilação
2. **IDEA não implementado** — 3 segmentos podem falhar
3. **tttm120** — Chaves originais indisponíveis (compilação original)

## ✅ Status

| Componente | Status |
|------------|--------|
| Parser RPO | ✅ Completo |
| Decryptor (11 cifras) | ✅ Testado |
| Hook LD_PRELOAD | ✅ Funcional |
| CLI | ✅ Implementado |
| Testes | ✅ 26/26 passing |
| Documentação | ✅ Completa |

## 🚀 Próximos Passos

1. Integrar com VSCode extension
2. Automatizar captura em CI/CD
3. Implementar IDEA cipher
4. Criar interface visual para extração

---

**Autor:** Agnes (Sapiens AI)  
**Data:** 2026-09-20
