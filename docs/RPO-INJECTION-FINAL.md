# Injeção de Bytecode em RPOs — Status Final

**Data:** 2026-10-10  
**Status:** ✅ FUNCIONAL (RPOs sintéticos) / 🔴 BLOQUEADO (RPOs reais)

---

## 1. Descobertas Críticas

### 1.1 Senha RSA: "manezinho"
- **Função:** Descriptografar chave RSA privada do RPO
- **Local:** `tCryptoRSA::SetKey(password="manezinho")`
- **Impacto:** Senha fixa/embedded no binário do appserver
- **Derivação de chaves AES:** KDF personalizado (não MD5/SHA padrão)

### 1.2 Chaves Capturadas (Live Capture)
| Sessão | Key 1 | Key 2 |
|--------|-------|-------|
| RPO Sintético | `05e677fe...` | `4c58980b...` |
| RPOs Reais | ❌ DIFERENTES | ❌ DIFERENTES |

**Conclusão:** Chaves são **efêmeras por compilação**. Cada `appsrvlinux -compile` gera novo par key/IV.

### 1.3 Criptografia dos RPOs Reais
| Métrica | Valor |
|---------|-------|
| Entropia | ~8.0 bits/byte (máxima) |
| Bytes únicos/bloco | 250-256/256 |
| APO records legíveis | **0** |
| Strings ASCII | **0** |

**Status:** RPOs reais (custom, tlpp, tttm120) estão **100% criptografados**.

---

## 2. Pipeline de Injeção (Funcional)

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. CARREGAR RPO                                                 │
│    ├─ Parse header (38B)                                        │
│    ├─ Separar admin/body/footer                                 │
│    └─ Validar self_offset                                       │
├─────────────────────────────────────────────────────────────────┤
│ 2. CARREGAR CAPTURA DE CHAVES                                   │
│    ├─ JSON: [encrypt, evpinit, setkey, ...]                     │
│    └─ Correlacionar cipher/key/iv/plaintext por segmento        │
├─────────────────────────────────────────────────────────────────┤
│ 3. DECODIFICAR BODY                                             │
│    ├─ Re-criptografar plaintext capturado                       │
│    ├─ Encontrar no body/admin                                   │
│    └─ Influxar zlib (se aplicável)                              │
├─────────────────────────────────────────────────────────────────┤
│ 4. EXTRAIR APO RECORDS                                          │
│    ├─ Parsear [code_size:4][name\0][ts:8][bt:4][btype:4][code] │
│    └─ Buscar função por nome                                    │
├─────────────────────────────────────────────────────────────────┤
│ 5. SUBSTITUIR BYTECODE                                          │
│    ├─ Pad/truncate para mesmo tamanho                           │
│    └─ Reconstruir APO record                                    │
├─────────────────────────────────────────────────────────────────┤
│ 6. RE-ENCRYPT + SAVE                                            │
│    ├─ Comprimir zlib                                            │
│    ├─ Criptografar com mesmas chaves                            │
│    └─ Manter tamanho total original                             │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Comandos CLI

```bash
# Análise
./advplc rpo analyze custom.rpo
./advplc rpo info custom.rpo

# Injeção
./advplc rpo inject input.rpo capture.json \
  --inject "NomeFuncao=/path/to/code.bin" \
  --out output.rpo

# Exemplo prático
./advplc rpo inject \
  pkg/rpo/testdata/live_capture.rpo \
  pkg/rpo/testdata/live_capture.json \
  --inject "RPORC5_TRIGGER.PRW=/tmp/test_code.bin" \
  --out /tmp/injected.rpo
```

---

## 4. Testes

```bash
# Todos os testes passando
go test ./pkg/rpo/... -count=1
# ok  github.com/advpl/compiler/pkg/rpo
# ok  github.com/advpl/compiler/pkg/rpo/injector

# Teste específico de injeção
go test ./pkg/rpo/injector/ -v -run TestReplaceFunction_MesmoTamanho
# --- PASS: TestReplaceFunction_MesmoTamanho (0.00s)
```

---

## 5. Limitações Atuais

| Limitação | Impacto | Workaround |
|-----------|---------|------------|
| RPOs reais criptografados | Não consegue injetar diretamente | Capturar chaves durante compilação real |
| Hook v8 não produz JSON | Sem chaves de compilação real | Debug do hook ou GDB |
| IDEA não suportado | 5/15 segmentos ilegíveis | Implementar IDEA ou ignorar |
| Re-encryption complexa | Requer mesmas chaves/IV | Capturar todos os segmentos |

---

## 6. Próximos Passos Recomendados

### 6.1 Imediatos (Baixo Esforço)
1. ✅ **Fix GetAPORecords** — Ordenar chaves do map para ordem determinística
2. ✅ **Testes passando** — Validação end-to-end em RPO sintético
3. 🔄 **Capturar chaves de RPO real** — Rodar compilação com hook v8

### 6.2 Méd Prazo (Médio Esforço)
1. Disassemblar `tApoFile::ReadApo` no Ghidra
2. Entender pipeline exato de criptografia
3. Automatizar captura de chaves em compilações reais

### 6.3 Longo Prazo (Alto Esforço)
1. Implementar re-encryption completa
2. Suporte a todos os ciphers (IDEA, etc.)
3. Injeção direta em RPOs reais do container

---

## 7. Arquivos Modificados

| Arquivo | Alteração |
|---------|-----------|
| `pkg/rpo/injector/injector.go` | Reescrito com parser APO completo |
| `pkg/rpo/injector/admin_index.go` | Novo: parser de índice admin |
| `pkg/rpo/injector/reencrypt.go` | Simplificado (utilitários apenas) |
| `pkg/rpo/injector/injector_test.go` | Teste de injeção corrigido |
| `cmd/advplc/cmd_rpo_inject.go` | CLI com suporte a batch inject |

---

## 8. Conclusão

**O "castelo de areia" (RPO) está compreendido e injetável em ambiente controlado.**

A "muralha" (appserver) ainda precisa ser escalada para:
1. Obter chaves de criptografia de RPOs reais
2. Entender o pipeline exato de leitura (`tApoFile::ReadApo`)

**Recomendação:** Focar na automação da captura ao vivo (hook v8) em vez de tentar quebrar a criptografia.

---

**Documento gerado por Agnes (Sapiens AI)**
