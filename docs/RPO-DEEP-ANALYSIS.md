# Análise Profunda: RPOs do Protheus 12.1.2510

**Data:** 2026-10-10  
**Metodologia:** Dissecção estrutural dos RPOs + análise de símbolos do binário  
**Conclusão:** RPOs são 100% criptografados; estrutura inferida de capturas ao vivo e binário

---

## 1. Executiva

Os 3 RPOs analisados (`custom`, `tlpp`, `tttm120`) estão **totalmente criptografados**:
- Entropia: ~8.0 bits/byte (máxima)
- Bytes únicos por bloco: 250-256/256
- APO records encontrados: **0**
- Strings ASCII: **0**

A estrutura do container é conhecida (header 38B + admin + body + footer 34B), mas o conteúdo é opaco sem as chaves de criptografia efêmeras.

---

## 2. Estrutura do Container RPO

```
┌─────────────────────────────────────────────────────────────────┐
│ HEADER (38 bytes)                                               │
│  ├─ [0x00-0x03] self_offset (uint32 LE)                        │
│  ├─ [0x04-0x0F] nome RPO (12 bytes, padded com zeros)          │
│  ├─ [0x10-0x25] reserved                                       │
│  └─ [0x26-0x27] flags                                          │
├─────────────────────────────────────────────────────────────────┤
│ ADMIN SECTION (self_offset - 38 bytes)                         │
│  ├─ Índices APO criptografados                                 │
│  └─ Formato esperado: [count:4][entry...]*                     │
├─────────────────────────────────────────────────────────────────┤
│ BODY ((total-34) - self_offset bytes)                          │
│  ├─ Bytecode P-Code criptografado                              │
│  └─ Formato esperado: [code_size:4][name\0][ts:8][bt:4][btype:4][code]
├─────────────────────────────────────────────────────────────────┤
│ FOOTER (34 bytes)                                              │
│  ├─ [0:-34:-24] magic (10 bytes): APNSRM0419/0420/0421         │
│  └─ [0:-24] trailer/hash (24 bytes)                            │
└─────────────────────────────────────────────────────────────────┘
```

### 2.1 Magnéticos por Tipo

| Magic | Significado | RPO Analisado |
|-------|-------------|---------------|
| `APNSRM0419` | custom | container_custom.rpo (461 KB) |
| `APNSRM0420` | tlpp | container_tlpp.rpo (9.9 MB) |
| `APNSRM0421` | tttm120 | container_tttm120.rpo (606 MB) |

---

## 3. Análise dos 3 RPOs

### 3.1 custom.rpo (461 KB)
```
Total:     461,312 bytes
Admin:     429,234 bytes (93%)
Body:      32,006 bytes (7%)
Ratio:     13.4:1
```
**Observação:** Body relativamente pequeno → poucas funções compiladas.

### 3.2 tlpp.rpo (9.9 MB)
```
Total:     9,955,604 bytes
Admin:     9,929,758 bytes (99.7%)
Body:      25,774 bytes (0.3%)
Ratio:     385:1
```
**Observação:** Ratio extremo admin/body → muitos metadados TLPP, poucas funções.

### 3.3 tttm120.rpo (606 MB)
```
Total:     606,303,559 bytes
Admin:     603,694,529 bytes (99.6%)
Body:      2,608,958 bytes (0.4%)
Ratio:     231:1
```
**Observação:** Body maior (2.5 MB) → ~800-2000 funções esperadas.

---

## 4. Símbolos APO/RPO no Binário (libaplinux.so)

Foram encontrados **849 símbolos** relacionados a APO/RPO:

### 4.1 Classes Principais
| Classe | Função |
|--------|--------|
| `tApoFile` | Leitura/escrita de RPO |
| `tApoFileBase` | Base para tApoFile |
| `tApoRegArr` | Array de registros APO |
| `tManagerRpo` | Gerenciador de RPOs |
| `tRPOInterface` | Interface genérica RPO |
| `tRPOAdvpl` | Implementação AdvPL |

### 4.2 Métodos Críticos
| Assinatura | Offset | Função |
|------------|--------|--------|
| `tApoRegArr::Find(char const*, int&)` | 0x25e99c0 | Busca função por nome |
| `tApoRegArr::Add(tApoReg*)` | 0x25e01d0 | Adiciona registro |
| `findFuncInRPO(tPublicEnv*, char const*, char const*, int, tString&)` | 0x1aa5820 | Busca em RPO |
| `getNameRpoType(eRpoType)` | 0x25dd1f0 | Nome do tipo RPO |
| `ApoFullName(tPublicEnv*, char const*)` | 0x1c50a40 | Nome completo da função |

### 4.3 Funções Estáticas (Commands)
| Função | Offset | Descrição |
|--------|--------|-----------|
| `getapoinfo_static` | 0x16646b0 | Retorna info de APO |
| `GetApoRes_static` | 0x1664a90 | Recupera recurso APO |
| `GetAllRpoInfo_static` | 0x16642d0 | Lista todos RPOs |
| `getrpoinfo_static` | 0x1730570 | Info do RPO |
| `GetRpoLog_static` | 0x1674670 | Log do RPO |
| `ChkRpoChg_static` | 0x162ee50 | Verifica mudanças |
| `srgetrpostatus_static` | 0x16db930 | Status do RPO |
| `srgetrpoversion_static` | 0x16dbd10 | Versão do RPO |

### 4.4 VTables (Identificação de Classes)
```
vtable for tApoFile           @ 0x46e29f8
vtable for tApoFileBase       @ 0x46e2c40
vtable for tApoRegArr         @ 0x42ac258
vtable for tManagerRpo        (implícita)
vtable for tRPOInterface      @ 0x45ce0d8
vtable for tRPOAdvpl          @ 0x4595640
```

---

## 5. Criptografia

### 5.1 Evidências
- **Entropia:** ~8.0 bits/byte (máximo teórico para 8-bit bytes)
- **Distribuição:** Uniforme (250-256 bytes únicos por bloco de 1KB)
- **Padrões:** Nenhum APO record legível encontrado
- **ZLIB:** Segmentos #1 e #15 no live_capture tinham zlib antes da cifra

### 5.2 Cifras Identificadas (live_capture.json)
| Segmento | Cipher | Key Size | IV | Localização |
|----------|--------|----------|-----|-------------|
| #1 | des_ede_ecb_cipher | 16B | - | ADMIN offset 176 |
| #2-4 | cast5_cfb64_cipher | 16B | 8B | ADMIN |
| #5-6 | idea_cfb64/cbc | 16B | 8B | ADMIN (não suportado) |
| #7,11 | cast5_cfb64_cipher | 16B | 8B | ADMIN |
| #8 | rc4_cipher | 16B | - | ADMIN |
| #9-10,12-13 | idea/bf | variado | variado | ADMIN |
| #14 | cast5_ecb_cipher | 16B | - | ADMIN |
| #15 | cast5_cbc_cipher | 16B | 8B | BODY offset 0 |

### 5.3 Pipeline de Criptografia (inferido)
```
1. Compressão zlib do bytecode APO
2. Divisão em segmentos (~256-512 bytes cada)
3. Para cada segmento:
   a. Escolher cipher da lista rotativa
   b. Gerar key/IV efêmeros
   c. Criptografar com EVP_EncryptInit_ex + EVP_EncryptUpdate
4. Concatenar segmentos criptografados
5. Anexar footer com magic + hash
```

---

## 6. Limitações e Próximos Passos

### 6.1 Limitações Atuais
- [ ] Sem chaves de criptografia dos RPOs reais
- [ ] Hook v8 não captura (compilação falha antes do encrypt)
- [ ] Binário do appserver não disassemblado (Ghidra com problema)
- [ ] RPOs reais 100% opacos sem captura ao vivo

### 6.2 Próximos Passos Sugeridos
1. **Corrigir hook v8** para capturar chaves durante compilação real
2. **Disassemblar tApoFile::ReadApo** no Ghidra para entender parser
3. **Implementar re-encryption** com chaves capturadas
4. **Testar injeção** em RPOs sintéticos (já funciona em live_capture.rpo)

---

## 7. Conclusão

A sugestão do operador foi acertada: os RPOs são "castelos de areia" relativamente simples (estrutura conhecida, formato APO documentado) escondidos atrás de "muralhas" de criptografia (binários gigantes, cifras legadas).

**O caminho mais produtivo:**
1. ✅ Estrutura do container: **Compreendida**
2. ✅ Formato APO: **Compreendido** (via live_capture)
3. ⚠️ Criptografia: **Operacional** (precisa de chaves)
4. ❌ Injeção em RPOs reais: **Bloqueada** (sem chaves)

**Recomendação:** Focar em automatizar a captura ao vivo de chaves durante compilação, em vez de tentar quebrar a criptografia.

---

**Documento gerado por Agnes (Sapiens AI)**
