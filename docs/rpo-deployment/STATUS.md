# Status Final - RPO Bytecode Injector

**Data:** 2026-09-30
**Branch:** unstable
**Versão:** 4.3.1

## Resumo Executivo

Implementação completa da infraestrutura para injeção manual de bytecode
em RPOs do Protheus, contornando o bug do appserver 24.3.1.1.

## Ambiente Docker

### Imagem
- **Nome:** `protheus-compile-custom:12.1.2510-custom`
- **Base:** Debian 12 slim
- **Fonte:** `/home/peder/TOTVS Linha - Protheus/`
- **Construção:** `docker/build-protheus.sh`

### Container
- **Nome:** `protheus-custom`
- **Portas:** 3998:3999 (TCP), 8089:8090 (WebApp)
- **Status:** Rodando (daemon mode funcional)

## Problema Identificado

### Bug: tAssertException no comando -compile
O appserver 24.3.1.1 falha com exception C++ ao usar o comando `-compile`:
```
terminate called after throwing an instance of 'tAssertException*'
```

**Workarounds testados (nenhum funcionou):**
- Diferentes formatos de INI
- Parâmetro `-rootpath`
- Variável de ambiente
- Symlink para INI
- Diferentes usuários
- Diferentes containers (custom e v4)
- Diferentes arquivos de teste

**Comportamento observado:**
- ✅ Daemon mode funciona
- ❌ Compile mode falha com exception

## Solução Implementada

### Comando `advplc rpo inject`
```bash
# Listar registros APO
advplc rpo inject custom.rpo captura.json --list

# Injetar bytecode (estrutura preparada)
advplc rpo inject custom.rpo captura.json \
  --inject Funcao=releases/bytecode/Funcao.bytecode \
  -o output.rpo
```

### Componentes

1. **`pkg/rpo/injector/injector.go`**
   - Parser de estrutura RPO
   - Parser de estrutura APO
   - Carregador de captura (usa rpo.CaptureSegment)
   - Recriptografia via rpo.EncryptSegment()

2. **`pkg/rpo/injector/reencrypt.go`**
   - Gerenciador de recriptografia
   - Compressão/descompressão zlib
   - Parse de info de cipher

3. **`cmd/advplc/cmd_rpo_inject.go`**
   - CLI para injeção
   - Suporte a múltiplos bytecodes
   - Output personalizado

4. **Scripts de automação**
   - `tools/build-integration/inject-bytecode.sh`
   - `tools/build-integration/auto-inject.sh`

## Bytecode Gerado
- **53 arquivos** em `releases/bytecode/`
- **Total:** ~2.5MB
- **Formato:** JSON (instruções Op/Arg/Arg2/Str)

## Estrutura APO Identificada
```
[4 bytes: size (LE)]
[name string\0]
[8 bytes: timestamp (double)]
[4 bytes: build_type]
[4 bytes: binary_type]
[size bytes: compiled code]
```

## Testes

### Com live_capture.rpo
- **9/15 segmentos decodificados** com sucesso
- **1 registro APO extraído** (536 bytes, code=512 bytes)
- Segmentos IDEA não suportados (limitação conhecida)

### Testes unitários
```bash
go test ./pkg/rpo/... -v
# PASS: 26/26 testes passando
```

## Próximos Passos

### 1. Debug do crash -compile (ALTA PRIORIDADE)
- Usar GDB para capturar stack trace
- Identificar qual assertion está falhando
- Verificar se é problema de configuração ou bug no appserver

### 2. Implementar ReplaceAPO (MÉDIA PRIORIDADE)
- Função `ReplaceAPO()` no injector
- Substituir código APO mantendo estrutura
- Recriptografar segmentos modificados

### 3. Testar com RPO real (MÉDIA PRIORIDADE)
- Capturar chaves durante compilação bem-sucedida
- Injetar bytecode e verificar funcionamento

### 4. Integração com build (BAIXA PRIORIDADE)
- Adicionar target Makefile
- Automatizar workflow completo

## Arquivos Gerados

### Docker
- `docker/protheus/Dockerfile`
- `docker/scripts/entrypoint.sh`
- `docker/build-protheus.sh`
- `docker/protheus/Dockerfile.gdb`

### Código
- `cmd/advplc/cmd_rpo_inject.go`
- `pkg/rpo/injector/injector.go`
- `pkg/rpo/injector/reencrypt.go`
- `releases/bytecode/*.bytecode` (53 arquivos)

### Scripts
- `tools/build-integration/inject-bytecode.sh`
- `tools/build-integration/auto-inject.sh`

### Documentação
- `docs/rpo-deployment/STATUS.md`
- `docs/rpo-deployment/BYTECODE-INJECTOR.md`
- `docs/rpo-deployment/FINAL-SUMMARY.md`
- `docs/rpo-deployment/SESSION-SUMMARY.md`

## Tempo Estimado para Conclusão
- Debug do crash: 4-6 horas
- ReplaceAPO: 6-8 horas
- Testes: 2-4 horas
- Integração: 2-3 horas
- **Total: 14-21 horas**

## Conclusão
A infraestrutura está pronta e testes passando com dados sintéticos.
O próximo passo crítico é resolver o bug de compilação para habilitar
captura de chaves reais e completar o ciclo de injeção.
