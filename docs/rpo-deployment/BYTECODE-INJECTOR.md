# Bytecode Injector para RPO Protheus

**Data:** 2026-09-30
**Status:** Infraestrutura básica criada, funcionalidade completa pendente

## Visão Geral

Este documento descreve a implementação de um injetor de bytecode para RPOs do Protheus,
capaz de decodificar, modificar e recodificar o conteúdo do RPO sem depender do
comando `-compile` do appserver (que apresenta bug de RootPath na versão 24.3.1.1).

## Arquitetura

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  RPO original   │────▶│  Decomposição    │────▶│  Decodificação  │
│  (criptografado)│     │  container       │     │  com chaves     │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                                                          ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  RPO final      │◀────│  Recodificação   │◀────│  Modificação    │
│  (criptografado)│     │  + recompressão  │     │  APO/bytecode   │
└─────────────────┘     └──────────────────┘     └─────────────────┘
```

## Componentes

### 1. Decomposição do Container (`pkg/rpo/rpo.go`)
- Lê header (38 bytes): selfOffset + name block
- Extrai admin section e body como blobs opacos
- Preserva footer (34 bytes): magic + trail

### 2. Decodificação (`pkg/rpo/cipher_dispatch.go`)
- Implementa ~12 cifras legadas do OpenSSL:
  - DES, 3DES (DES-EDE), RC4, RC5, CAST5, Blowfish, RC2
- Modos: ECB, CBC, CFB64, OFB
- Chave/IV efêmeros capturados durante compilação

### 3. Parser APO (`pkg/rpo/injector/injector.go`)
- Estrutura identificada:
  ```
  [4 bytes: size (LE)]
  [name string\0]
  [8 bytes: timestamp (double)]
  [4 bytes: build_type]
  [4 bytes: binary_type]
  [size bytes: compiled code]
  ```

### 4. Injeção de Bytecode
- Substituir código existente por novo bytecode
- Manter estrutura APO intacta
- Recodificar com mesmas chaves

## Uso

### Decodificar RPO
```bash
./advplc rpo decrypt arquivo.rpo captura.json
```

### Analisar estrutura
```bash
go run pkg/rpo/injector/example.go arquivo.rpo captura.json
```

### Injetar bytecode (futuro)
```go
inj, _ := injector.NewInjector(rpoData)
inj.LoadCapture("captura.json")
body, decoded := inj.DecodeBody()

// Parsear APO records
records := injector.ExtractAPORecords(body)

// Modificar record
for _, rec := range records {
    if rec.Name == "MinhaFuncao.PRW" {
        rec.Code = novoBytecode
        break
    }
}

// Salvar
inj.Save("output.rpo")
```

## Limitações Atuais

1. **Decodificação incompleta**: Apenas 10/15 segmentos decodificados
   - 5 segmentos usam IDEA (não implementado)
   
2. **Recriptografia**: Não implementada ainda
   - Requer reproduzir exatamente o padrão de cifras
   - Necessita capturar novas chaves para o RPO modificado

3. **Checksum/Footer**: O footer (24 bytes) pode conter checksum
   - Pode precisar ser recalculado

## Próximos Passos

1. Implementar recriptografia dos segmentos modificados
2. Adicionar suporte para IDEA (ou pular segmentos IDEA)
3. Testar com RPO real (custom.rpo do container)
4. Criar CLI para injeção simples

## Arquivos

- `pkg/rpo/injector/injector.go` - Implementação base
- `pkg/rpo/cipher_dispatch.go` - Algoritmos de cifra
- `pkg/rpo/rpo.go` - Parser do container
- `docs/rpo-deployment/` - Documentação relacionada

## Referências

- `docs/rpo-format.md` - Especificação completa do formato RPO
- `docs/RPO-GROUND-TRUTH.md` - Status confirmado/refutado das alegações
- `tools/rpo-live-inspect/` - Ferramentas de captura ao vivo
