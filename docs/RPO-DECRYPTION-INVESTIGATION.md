# Investigação de Descriptografia RPO — Status

**Data:** 2026-10-10  
**Status:** 🔴 BLOQUEADO — Chaves capturadas não correspondem aos RPOs reais

---

## 1. Senha RSA: "manezinho"

**DESCOBERTA CRÍTICA:** A senha `"manezinho"` é usada para proteger a chave RSA privada do RPO.

### Evidências
```
[rpo_key_hook] tCryptoRSA::SetKey: password=manezinho
[rpo_key_hook] SetKey: key(16B)=05e677fe954895cb458417da7dbb9039 iv(16B)=78389e079bb46f5681bdc5e93f305392
[rpo_key_hook] SetKey: key(16B)=4c58980b6cebcd9c5eaa07836806fd23 iv(16B)=850ab320e38596a1a75d60e8c957293f
```

### Análise da Derivação de Chaves
- **Senha:** `"manezinho"` (9 bytes, UTF-8)
- **MD5:** `78952adf2ad11495d3d188d69f5c55b2` ≠ Key1/Key2
- **SHA1:** `bd41f086c2a9bbf35f08091ca0a24a1e28599ad6` ≠ Key1/Key2
- **SHA256:** `302e4c2183de4ff87e86dac46df989e101e950376ea1aaee971c11ae8cb04fea` ≠ Key1/Key2

**Conclusão:** A derivação de chaves usa um KDF personalizado (não MD5/SHA padrão).

---

## 2. Chaves Capturadas vs. RPOs Reais

### Chaves Capturadas (live_capture.json)
| Index | Key | IV | Cipher |
|-------|-----|----|--------|
| 1 | `05e677fe...` | `78389e07...` | des_ede_ecb |
| 2-4 | `05e677fe...` | `78389e07...` | cast5_cfb64 |
| 5-6 | `05e677fe...` | `78389e07...` | idea_* (não suportado) |
| 7,11 | `05e677fe...` | `78389e07...` | cast5_cfb64 |
| 8 | `05e677fe...` | `78389e07...` | rc4 |
| 9-10,12-13 | `05e677fe...` | `78389e07...` | idea/bf |
| 14 | `05e677fe...` | `78389e07...` | cast5_ecb |
| 15 | `4c58980b...` | `850ab320...` | cast5_cbc |

### Tentativa de Decodificação dos RPOs Reais
```
TENTANDO: container_custom.rpo com Key1/Key2
RESULTADO: ❌ FALLOUT — plaintext não contém padrões esperados

TENTANDO: container_tlpp.rpo com Key1/Key2  
RESULTADO: ❌ FALLOUT — plaintext não contém padrões esperados

TENTANDO: container_tttm120.rpo com Key1/Key2
RESULTADO: ❌ FALLOUT — plaintext não contém padrões esperados
```

**Conclusão:** As chaves capturadas pertencem ao **RPO sintético** (`live_capture.rpo`), não aos RPOs reais do container. Cada compilação gera chaves únicas.

---

## 3. Pipeline de Criptografia (Reconstruído)

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. COMPILAÇÃO                                                   │
│    ├─ Source (.prw/.tlpp) → P-Code                              │
│    ├─ Empacotar em APO records                                  │
│    └─ Comprimir com zlib                                      │
├─────────────────────────────────────────────────────────────────┤
│ 2. CRIPTOGRAFIA                                                 │
│    ├─ Dividir em segmentos (~256-512 bytes)                    │
│    ├─ Para cada segmento:                                       │
│    │   ├─ Escolher cipher da lista rotativa                    │
│    │   ├─ Gerar key/IV efêmeros (derivação de "manezinho")      │
│    │   └─ Criptografar com EVP_EncryptInit_ex + Update         │
│    └─ Concatenar segmentos                                     │
├─────────────────────────────────────────────────────────────────┤
│ 3. EMPACOTAMENTO RPO                                            │
│    ├─ Header (38 bytes)                                         │
│    ├─ Admin section (índice criptografado)                      │
│    ├─ Body (bytecode criptografado)                            │
│    ├─ Footer magic (10 bytes)                                   │
│    └─ Trailer/hash (24 bytes)                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Lista Rotativa de Ciphers
1. des_ede_ecb_cipher
2. cast5_cfb64_cipher
3. bf_ecb_cipher
4. cast5_ofb_cipher
5. idea_cfb64_cipher ⚠️
6. idea_cbc_cipher ⚠️
7. cast5_cfb64_cipher
8. rc4_cipher
9. idea_cbc_cipher ⚠️
10. idea_ecb_cipher ⚠️
11. cast5_cfb64_cipher
12. idea_cbc_cipher ⚠️
13. bf_ofb_cipher
14. cast5_ecb_cipher
15. cast5_cbc_cipher

⚠️ IDEA não implementada no hook atual.

---

## 4. Próximos Passos

### 4.1 Imediatos
1. **Rodar compilação real** com hook v8 para capturar chaves do RPO real
2. **Disassemblar tApoFile::ReadApo** no Ghidra (pendente)
3. **Implementar re-encryption** com chaves capturadas

### 4.2 Se Hook Não Funcionar
- Usar GDB no container (se possível)
- Analisar binário diretamente para encontrar rotina de criptografia
- Tentar brute-force na derivação de chaves (senhas comuns)

### 4.3 Abordagem Alternativa
Se a criptografia for muito difícil de quebrar:
1. **Inject directly in plaintext** — modificar o RPO antes da criptografia
2. **Patch appserver** — interceptar ReadApo e injetar diretamente
3. **Usar API do Protheus** — adicionar função via mecanismo oficial

---

## 5. Arquivos de Referência

| Arquivo | Descrição |
|---------|-----------|
| `docs/RPO-GROUND-TRUTH.md` | Especificação autoritativa do formato |
| `pkg/rpo/testdata/live_capture.rpo` | RPO sintético (decodificável) |
| `pkg/rpo/testdata/live_capture.json` | Captura de chaves (RPO sintético) |
| `pkg/rpo/testdata/container_*.rpo` | RPOs reais (criptografados) |
| `tools/rpo-live-inspect/rpo_key_hook/` | Hook LD_PRELOAD |
| `pkg/rpo/injector/` | Implementação Go do injetor |

---

**Documento gerado por Agnes (Sapiens AI)**
