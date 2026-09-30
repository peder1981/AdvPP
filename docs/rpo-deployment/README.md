# RPO Bytecode Injector

Sistema completo para injetar bytecode compilado em RPOs do Protheus, bypassando limitações do compilador oficial.

## Status

✅ **INFRAESTRUTURA FUNCIONAL** - Teste E2E validado com RPO sintético

## Workflow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Fonte .prw │────▶│  Compilação  │────▶│  Bytecode   │
└─────────────┘     └──────┬──────┘     └──────┬──────┘
                           │                   │
                           │                   ▼
                           │            ┌─────────────┐
                           │            │  Parser RPO │
                           │            └──────┬──────┘
                           │                   │
                           ▼                   ▼
                    ┌─────────────┐     ┌─────────────┐
                    │  Captura    │────▶│  Injector   │
                    │  (hook)     │     │  (modify)   │
                    └─────────────┘     └──────┬──────┘
                                               │
                                               ▼
                                        ┌─────────────┐
                                        │  RPO Injetado│
                                        └─────────────┘
```

## Comandos

### Analisar RPO
```bash
./advplc rpo info arquivo.rpo
./advplc rpo regions arquivo.rpo
./advplc rpo analyze arquivo.rpo
```

### Injetar Bytecode
```bash
# Listar segmentos
./advplc rpo inject arquivo.rpo captura.json --list

# Injetar segmento específico
./advplc rpo inject arquivo.rpo captura.json \
  --inject 1=releases/bytecode/Funcao.bytecode \
  -o output.rpo
```

### Compilar com Hook
```bash
docker exec protheus-custom bash -c '
  export LD_PRELOAD=/tmp/rpo_key_hook.so
  export RPO_KEYS_OUTPUT=/tmp/capture.json
  cd /totvs/protheus12.1.2510/bin
  ./appsrvlinux -compile -env=P12 -files=fonte.prw
'
```

## Estrutura de Projeto

```
pkg/rpo/
├── rpo.go              # Parser container
├── decrypt.go          # Decryptor AES
├── apo_parser.go       # Parser APO records
├── cipher_heuristic.go # Identificação de cipher
├── capture_parser.go   # Parser de captura
├── cipher_dispatch.go  # Encrypt/Decrypt segments
└── injector/
    ├── injector.go     # Injeção e ReplaceAPO
    └── reencrypt.go    # Re-criptografia

cmd/advplc/
└── cmd_rpo_inject.go   # CLI command

releases/bytecode/      # Bytecodes compilados
docs/rpo-deployment/    # Documentação
```

## Limitações

| Limitação | Detalhes |
|-----------|----------|
| IDEA cipher | Não implementado (4/15 segmentos em teste) |
| Tamanho fixo | ReplaceAPO suporta apenas mesmo tamanho |
| RPO-specific | Captura só funciona com RPO gerado |

## Próximos Passos

1. Testar com RPO real de produção
2. Implementar ReplaceAPO dinâmico (tamanho variável)
3. Integração com CI/CD
4. Dashboard web

## Recursos

- [Relatório Final](SESSIONFINAL-E2E.md)
- [Especificação do Formato](RPO-FORMAT-SPEC.md)
- [Ground Truth](../../RPO-GROUND-TRUTH.md)

---
**Versão:** 4.3.1
**Última atualização:** 2026-09-30
