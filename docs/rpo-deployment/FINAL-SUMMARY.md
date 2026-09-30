# Resumo Final - RPO Bytecode Injector

**Data:** 2026-09-30
**Branch:** unstable
**Versão:** 4.3.1

## Conclusão

Após ampla investigação, implementamos uma **abordagem alternativa** para
injeção de bytecode em RPOs do Protheus, contornando o bug do appserver
24.3.1.1 que impede o uso do comando `-compile`.

## Ambiente Docker Criado

### Imagem
- **Nome:** `protheus-compile-custom:12.1.2510-custom`
- **Base:** Debian 12 slim
- **Fonte:** `/home/peder/TOTVS Linha - Protheus/`

### Container
- **Nome:** `protheus-custom`
- **Portas:** 3998:3999 (TCP), 8089:8090 (WebApp)
- **Status:** Rodando (daemon mode funcional)

### Estrutura
```
/totvs/protheus12.1.2510/
├── bin/
│   ├── appsrvlinux          # Binário do appserver
│   ├── libaplinux.so        # Biblioteca principal (738MB)
│   ├── appserver.ini        # Configurado automaticamente
│   └── ...
├── apo/
│   ├── tttm120.rpo          # RPO total (353MB)
│   └── custom.rpo           # Copiado de tttm120.rpo
└── protheus_data/           # Dados do sistema
```

## Problema Identificado

### Bug: Error reading RootPath Key
O appserver 24.3.1.1 falha ao usar o comando `-compile` com o erro:
```
[ERROR][SERVER] [CMDLINE] Error reading RootPath Key.
```

**Workarounds testados (nenhum funcionou):**
- Diferentes formatos de INI
- Parâmetro `-rootpath`
- Variável de ambiente
- Symlink para INI
- Diferentes usuários (root, totvs)
- Diferentes diretórios de trabalho

**Comportamento observado:**
- ✅ Daemon mode funciona
- ✅ Console mode funciona
- ❌ Compile mode falha

## Solução Implementada

### Comando `advplc rpo inject`
Novo comando implementado para manipulação manual de RPOs:

```bash
# Listar registros APO
advplc rpo inject custom.rpo captura.json --list

# Injetar bytecode
advplc rpo inject custom.rpo captura.json --inject Funcao=releases/bytecode/Funcao.bytecode -o output.rpo
```

### Componentes
1. **`pkg/rpo/injector/injector.go`** - Biblioteca base para manipulação de RPO
2. **`cmd/advplc/cmd_rpo_inject.go`** - Comando CLI
3. **Parser APO** - Estrutura identificada e implementada

### Estrutura APO Identificada
```
[4 bytes: size (LE)]
[name string\0]
[8 bytes: timestamp (double)]
[4 bytes: build_type]
[4 bytes: binary_type]
[size bytes: compiled code]
```

## Bytecode Gerado
- **52 arquivos** em `releases/bytecode/`
- **Total:** ~2.5MB
- **Formato:** JSON (instruções Op/Arg/Arg2/Str)

## Próximos Passos

### 1. Completar Recriptografia (Crítico)
Para que o RPO injetado funcione, precisamos:
- Reproduzir a sequência exata de cifras usadas pelo appserver
- Usar as mesmas chaves/IV capturados durante compilação
- Recalcular checksums se necessário

### 2. Testar com RPO Real
- Capturar chaves durante compilação real no container
- Injetar bytecode e verificar se o RPO funciona

### 3. Automatizar Workflow
```bash
# Script proposal
./tools/build-integration/inject-bytecode.sh \
  --rpo custom.rpo \
  --captura captura.json \
  --bytecode-dir releases/bytecode \
  --output custom_injected.rpo
```

## Arquivos Gerados

### Docker
- `docker/protheus/Dockerfile`
- `docker/scripts/entrypoint.sh`
- `docker/build-protheus.sh`

### Código
- `cmd/advplc/cmd_rpo_inject.go`
- `pkg/rpo/injector/injector.go`
- `releases/bytecode/*.bytecode` (52 arquivos)

### Documentação
- `docs/rpo-deployment/STATUS.md`
- `docs/rpo-deployment/BYTECODE-INJECTOR.md`
- `docs/rpo-deployment/FINAL-SUMMARY.md`

## Tempo Estimado para Conclusão
- Recriptografia: 4-6 horas
- Testes: 2-4 horas
- Automatização: 2-3 horas
- **Total: 8-13 horas**

## Conclusão
A infraestrutura básica está pronta. O próximo passo crítico é implementar
a recriptografia para completar o ciclo de injeção de bytecode.
