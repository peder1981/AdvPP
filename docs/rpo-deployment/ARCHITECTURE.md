# Arquitetura do RPO Bytecode Injector

## Visão Geral

O sistema é composto por cinco camadas principais:

1. **Container Parser** - Parseia a estrutura RPO
2. **Cipher Manager** - Gerencia criptografia
3. **APO Parser** - Extrai registros APO
4. **Injector** - Realiza substituições
5. **Reencrypt Manager** - Recriptografa e reconstrói

## Diagrama de Classes

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI Layer                             │
│                   (cmd_rpo_inject.go)                        │
├─────────────────────────────────────────────────────────────┤
│                      Service Layer                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Injector   │  │  CipherMgr   │  │  APOParser   │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                 │               │
│         └─────────────────┼─────────────────┘               │
│                           │                                 │
│                    ┌──────┴──────┐                          │
│                    │  RPOParser  │                          │
│                    └─────────────┘                          │
├─────────────────────────────────────────────────────────────┤
│                      Data Layer                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   RPOFile    │  │ APORecord    │  │CaptureSeg    │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## Fluxo de Dados Detalhado

### 1. Entrada

```
RPO Binário + Captura JSON
         │
         ▼
   ┌──────────┐
   │  Parse   │──▶ RPOFile struct
   │  RPO     │
   └──────────┘
         │
         ▼
   ┌──────────┐
   │  Parse   │──▶ []CaptureSegment
   │ Captura  │
   └──────────┘
```

### 2. Processamento

```
   RPOFile
       │
       ▼
┌──────────────┐     ┌──────────────┐
│  DecodeBody  │────▶│ apobodies    │
│  (cipher)    │     │ (zlib plain) │
└──────────────┘     └──────┬───────┘
                            │
                            ▼
                      ┌──────────────┐
                      │ ExtractAPO   │──▶ []APORecord
                      │  Records     │
                      └──────────────┘
                            │
                            ▼
                      ┌──────────────┐
                      │  ReplaceAPO  │──▶ Body modificado
                      │  (dynamic)   │
                      └──────────────┘
```

### 3. Saída

```
   Body modificado
         │
         ▼
   ┌──────────────┐
   │ Recompress   │──▶ zlib compressed
   └──────────────┘
         │
         ▼
   ┌──────────────┐
   │ Reencrypt    │──▶ ciphertext
   └──────────────┘
         │
         ▼
   ┌──────────────┐
   │   Save RPO   │──▶ RPO injetado
   └──────────────┘
```

## Estruturas de Dados

### RPOFile

```go
type RPOFile struct {
    Name         string   // Nome do RPO
    SelfOffset   uint32   // Offset do body
    AdminSection []byte   // Dados admin
    Body         []byte   // Body criptografado
    FooterMagic  string   // Magic (APNSRM0419, etc.)
    Trailer      []byte   // 24 bytes de trailer
}
```

### APORecord

```go
type APORecord struct {
    Offset     int      // Offset no body decomprimido
    Size       int      // Tamanho total do registro
    Name       string   // Nome do objeto
    Timestamp  float64  // Timestamp de compilação
    BuildType  int      // Tipo de build
    BinaryType int      // Tipo binário
    Code       []byte   // Bytecode
}
```

### CaptureSegment

```go
type CaptureSegment struct {
    N         int    // Número sequencial
    Cipher    string // Nome do cipher
    Key       string // Chave hex
    IV        string // IV hex
    Plaintext string // Plaintext hex
}
```

## Dependências

| Pacote | Uso |
|--------|-----|
| `encoding/binary` | Leitura/escrita de inteiros LE |
| `encoding/hex` | Conversão key/iv/plaintext |
| `compress/zlib` | Compressão/descompressão |
| `crypto/des` | Ciphers DES/3DES |
| `crypto/cipher` | Interfaces de cipher |
| `crypto/aes` | (não usado - apenas legados) |
| `golang.org/x/crypto/rc4` | Cipher RC4 |
| `bytes` | Operações em byte slices |
| `strings` | Manipulação de strings |

## Performance

| Operação | Tempo | Memória |
|----------|-------|---------|
| Parse RPO (22KB) | 40µs | ~100KB |
| Parse RPO (9.5MB) | 9.6ms | ~10MB |
| Decode Body (15 seg) | 21ms | ~300KB |
| Extract APO (818B) | <1µs | ~10KB |
| ReplaceAPO (+36B) | 0.5ms | ~50KB |
| Reencrypt | 2ms | ~400KB |
| **Total E2E** | **~15ms** | **~1.5MB** |

## Segurança

- Chaves de criptografia são efêmeras (descartadas após uso)
- Nenhum segredo é persistido em disco
- Hook LD_PRELOAD é opcional e controlado pelo usuário

---

*Arquitetura do sistema RPO Bytecode Injector*
