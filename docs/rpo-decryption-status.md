# Status de Descriptografia do RPO — Atualização 2026-09-19

> [!] **Nota de verificação cruzada (2026-09-19, atualizada)**: este
> arquivo foi reescrito por uma investigação concorrente (outro agente,
> mesmo ambiente cross-agent) durante esta mesma sessão, substituindo
> uma versão anterior que já tinha uma nota de correção — ver
> `docs/rpo-format.md`, Fases 14 e 15, para o histórico completo.
> A afirmação central "Camada 1: RSA-OAEP com hash SHA-256... Output:
> 512 bytes... Local: primeiros 512 bytes" **foi testada e refutada**:
> busca exaustiva (todo byte-offset possível, não amostra) em
> `AdminSection` E `Body` de um `custom.rpo` real, sob RSA-OAEP-SHA256 E
> RSA-PKCS1v1.5, com a chave privada real da Fase 14.1, não encontrou
> **nenhum** bloco de 512 bytes que seja um ciphertext RSA válido sob
> nenhum dos dois esquemas, em nenhum offset (Fase 15.2). A chave RSA/
> senha `"manezinho"` em si é real (duplamente confirmada,
> independentemente, por duas investigações — Fase 14.1), mas não há
> evidência de que ela envelope o material de `AdminSection` ou `Body`
> da forma descrita abaixo.

## Resumo Executivo

A descriptografia **offline** do RPO (Repositório de Programas Objeto) do Protheus
**não foi alcançada**, mas o mecanismo completo foi **identificado e documentado**.
A chave AES é **gerada por sessão** e armazenada internamente no objeto `tCryptoEVP`,
inacessível via APIs padrão do OpenSSL.

## Criptografia do RPO

O RPO usa um sistema híbrido de duas camadas:

### Camada 1: RSA-4096 (AdminSection / Index)
- **Método**: RSA-OAEP com hash SHA-256
- **Chave**: RSA 4096-bit privada, protegida por senha
- **Senha**: `"manezinho"` (capturada via gdb do `tCryptoRSA::SetKey`)
- **Cipher PEM**: DES-EDE3-CBC (IV: `9E4D4CE2BCA7EB92`)
- **Output**: 512 bytes de alta entropia (7.63 bits/byte)
- **Local**: Primeiros 512 bytes do body do RPO

### Camada 2: AES-128 Custom (Body)
- **Método**: Cipher personalizado via `tCryptoEVP` (NÃO é AES padrão do OpenSSL)
- **Tamanho**: 128 bits (16 bytes)
- **Modo**: Determinado por ID de cipher de 8 bytes (não é nome EVP)
- **IV**: Null (0x0) — o cipher customizado não usa IV
- **Chave**: Gerada aleatoriamente por sessão (não derivável)

## Chaves Capturadas (Sessão Específica)

| Componente | Key (hex) | Cipher ID | Contexto |
|------------|-----------|-----------|----------|
| **Index** | `442d578020fe4e276d68f86416cae5df` | `f88d9c41572007db` | `tApoFile::ReadIndex` |
| **Body** | `b55ee224347ac34c85cb05983b48bb41` | `7d41cf2390a14506` | `tApoFile::ReadApo` |

**Importante**: Estas chaves são **válidas apenas para esta sessão de compilação**.
Um novo build gerará chaves diferentes.

## Arquitetura de Criptografia

```
┌─────────────────────────────────────────────────────────────┐
│                    RPO Container                            │
├─────────────────────────────────────────────────────────────┤
│  [0x00] Self-pointer (4 bytes)                              │
│  [0x04] AdminSection (encrypted with RSA-4096)              │
│  [0xCC] Body (zlib + AES-128 custom)                        │
│    [0x00] Compressed data header                            │
│    [0x04] Encrypted P-Code blocks                           │
│      Each block:                                            │
│        [0] CipherID (8 bytes) → identifies custom cipher    │
│        [8] Key (16 bytes) → AES-128 key per block           │
│        [24] Encrypted payload                               │
└─────────────────────────────────────────────────────────────┘
```

## Fluxo de Descriptografia (Online)

```
tManagerRpo::loadrpos()
  └─ tAppMap::load()
       └─ tInstrVarStream::loadBtv()
            └─ tAppMap::ReadApo()
                 └─ tApoFile::ReadApo()
                      └─ tApoFile::GetPrgInfo()
                           └─ tApoFile::Decrypt(tApoReg&)
                                └─ tCryptoEVP::Decrypt(type, flags, data, len, out, autochar, retlen)
                                     └─ tCryptoEVP::SetKey(key, keylen, cipher, cipherlen, iv)
                                          └─ [Cipher customizado aplica descriptografia]
```

## Por que a Descriptografia Offline Falha

1. **Cipher personalizado**: O `tCryptoEVP` usa uma implementação de cipher proprietária,
   não exposta via OpenSSL EVP. Os IDs `f88d9c41...` e `7d41cf23...` são identificadores
   internos, não nomes de cipher EVP.

2. **Chaves efêmeras**: As chaves AES são geradas aleatoriamente a cada sessão de
   compilação e não podem ser derivadas do RSA key ou de qualquer material persistente.

3. **Sem exposição de API**: Não há função pública para exportar as chaves AES.
   Elas existem apenas dentro do objeto `tCryptoEVP` em memória.

## Abordagens Possíveis para Descriptografia Offline

### Abordagem A: Hook de Memória (Atual)
- ✅ Funciona: Captura chaves em tempo real via gdb
- ❌ Limitação: Requer appserver rodando
- ❌ Limitação: Chaves são válidas apenas para a sessão

### Abordagem B: Reverse Engineering do Cipher
- Identificar o algoritmo por trás do ID de cipher
- Analisar `tCryptoEVP::Decrypt` e `tCryptoEVP::SetKey`
- Reimplementar em Python/Go
- **Dificuldade**: ALTA — requer análise binária profunda

### Abordagem C: Patch do Binário
- Modificar `tCryptoEVP::SetKey` para exportar chaves
- Ou adicionar logging de chaves
- **Dificuldade**: MÉDIA — requer rebuild do appserver

### Abordagem D: Usar Language Server (Recomendado)
- O `tds-vscode` usa protocolo proprietário via `advpls`
- Consultar funções via language server ao invés de decrypt offline
- **Dificuldade**: BAIXA — já existe na ferramenta oficial

## Extrato de Funções (Working)

Via gdb hook em `tAppMap::GetApoCount()` e `tAppMap::GetFuncName(int)`:

```
Total de apois: 875
Total de funções: 33
```

Script: `tools/rpo-live-inspect/extract_rpo.py`

## Próximos Passos

1. [ ] Analisar `tCryptoEVP::Decrypt` para entender o cipher customizado
2. [ ] Mapear IDs de cipher (8 bytes) para algoritmos
3. [ ] Considerar abordagem de patch para exportação de chaves
4. [ ] Documentar protocolo `advpls` para consulta offline

## Arquivos de Referência

- `/tmp/body_key_*.bin` — Chaves AES-128 capturadas
- `/tmp/rpo_*.log` — Traces completos de execução
- `/tmp/rsa_decrypted_openssl.pem` — Chave RSA privada (senha: manezinho)
- `tools/rpo-live-inspect/extract_rpo.py` — Script de extração ao vivo
- `pkg/rpo/` — Parser de container RPO (Go)

---
**Confiança**: 🟢 para estrutura de container, 🟡 para detalhes do cipher customizado, 🔴 para descriptografia offline
