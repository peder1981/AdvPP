# Análise Estrutural dos RPOs do Protheus 12.1.2510

**Data:** 2026-10-10  
**Autores:** Peder Munksgaard  
**Status:** 🟡 INFERIDO - Requer validação com captura ao vivo

---

## 1. Visão Geral

Foram analisados 3 arquivos RPO reais extraídos do container Protheus 12.1.2510:

| Arquivo | Tamanho | Magic | SelfOffset | Admin | Body |
|---------|---------|-------|------------|-------|------|
| `container_custom.rpo` | 461 KB | APNSRM0419 | 429,272 | 429,234 B | 32,006 B |
| `container_tlpp.rpo` | 9.9 MB | APNSRM0420 | 9,929,796 | 9,929,758 B | 25,774 B |
| `container_tttm120.rpo` | 606 MB | APNSRM0421 | 603,694,567 | 603,694,529 B | 2,608,958 B |

---

## 2. Estrutura Confirmada do Container RPO

```
+------------------+------------------+------------------+--------+
|     HEADER       |   ADMIN SECTION  |      BODY        | FOOTER |
|    (38 bytes)    |   (variável)     |    (variável)    |(34 B)  |
+------------------+------------------+------------------+--------+
| 0x00-0x03        | 0x26-0x??        | 0x??-0x??        | xxxxx  |
| self_offset:LE   | (criptografado)  | (criptografado)  | magic  |
| 0x04-0x0F        |                  |                  | trail  |
| nome RPO (12B)   |                  |                  |(24 B)  |
| 0x10-0x25        |                  |                  |        |
| padding/zeros    |                  |                  |        |
| 0x26-0x27        |                  |                  |        |
| flags?           |                  |                  |        |
+------------------+------------------+------------------+--------+
```

### 2.1 Header (38 bytes)
- **Offsets 0x00-0x03:** `self_offset` (uint32 LE) — indica onde começa o body
- **Offsets 0x04-0x0F:** Nome do RPO (12 bytes, padding com zeros)
- **Offsets 0x10-0x25:** Reserved/padding
- **Offsets 0x26-0x27:** Flags (?)

### 2.2 Admin Section
- **Tamanho:** `self_offset - 38` bytes
- **Conteúdo:** Índices APO criptografados
- **Formato esperado:** `[count:4][entry...]*` onde cada entry tem:
  - `name_len:4` + `name\0`
  - `timestamp:8` (double)
  - `9 × int32` fields (offset, size, etc.)
  - `flag:1`

### 2.3 Body
- **Tamanho:** `(total_size - 34) - self_offset` bytes
- **Conteúdo:** Bytecode P-Code criptografado
- **Formato esperado:** Sequência de APO records:
  ```
  [code_size:4][name\0][timestamp:8][build_type:4][binary_type:4][code:code_size]
  ```

### 2.4 Footer (34 bytes)
- **Offsets 0:-34:-24:** Magic type (10 bytes)
  - `APNSRM0419` = custom
  - `APNSRM0420` = tlpp
  - `APNSRM0421` = tttm120
- **Offsets 0:-24:** Trailer/hash (24 bytes)

---

## 3. Criptografia

### 3.1 Evidências de Criptografia

| Métrica | Valor | Interpretação |
|---------|-------|---------------|
| Entropia (body) | ~8.0 bits/byte | Máxima aleatoriedade = criptografado |
| Bytes únicos/bloco | 250-256/256 | Distribuição uniforme |
| APO records encontrados | 0 | NENHUM record legível |
| Strings ASCII | 0 | Nenhuma string identificável |

### 3.2 Tipo de Criptografia (inferido)

Baseado na investigação anterior com `live_capture.rpo`:
- **Algoritmo:** Cifras legadas OpenSSL rotativas (DES-EDE-ECB, CAST5-CFB64, BF-ECB, RC4, IDEA)
- **Chaves:** Efêmeras, geradas por sessão
- **IV:** Presente em modos CFB/ CBC, ausente em ECB
- **Payload:** zlib compressão antes da cifra

### 3.3 Segmentos de Captura (referência)

Do arquivo `live_capture.json` (RPO sintético):
- **Segmento #1:** des_ede_ecb_cipher → ADMIN offset 176
- **Segmento #15:** cast5_cbc_cipher → BODY offset 0
- **Total:** 15 segmentos, 5 com IDEA (não suportado)

---

## 4. Comparação entre RPOs

### 4.1 Proporção Admin/Body

| RPO | Admin (KB) | Body (KB) | Ratio Admin:Body |
|-----|------------|-----------|------------------|
| custom | 419.2 | 31.3 | 13.4:1 |
| tlpp | 9696.8 | 25.2 | 384:1 |
| tttm120 | 586399 | 2548 | 230:1 |

**Observação:** O RPO `tlpp` tem ratio admin/body extremamente alto (384:1), sugerindo:
- Muitos metadados de compilação TLPP
- Poucas funções compiladas (body pequeno)

### 4.2 Tamanhos de Body

- **custom:** 32 KB body → ~10-20 funções
- **tlpp:** 25 KB body → ~8-15 funções TLPP
- **tttm120:** 2.5 MB body → ~800-2000 funções

---

## 5. Próximos Passos Sugeridos

1. **Captura ao vivo:** Hook LD_PRELOAD em `EVP_EncryptInit_ex` + `EVP_EncryptUpdate`
2. **Análise binária:** Disassembly de `tApoFile::ReadApo`, `tApoFileBase::WriteIndex`
3. **Teste de injetor:** Usar RPO sintético (live_capture.rpo) para validar pipeline
4. **Decodificação real:** Aplicar chaves capturadas nos RPOs reais

---

## 6. Limitações Atuais

- ❌ Sem chaves de criptografia dos RPOs reais
- ❌ Hook v8 não produziu JSON de captura (compilação falha antes)
- ⚠️ RPOs reais são 100% opacos sem captura ao vivo

---

**Documento gerado automaticamente por Agnes (Sapiens AI)**
