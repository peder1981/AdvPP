# Captura de Cipher Name via GDB/Hook

**Data:** 2026-09-30
**Metodologia:** LD_PRELOAD hook em `EVP_EncryptInit_ex`

## Progresso

### 1. Hook EVP_EncryptInit_ex ✅
- Hook implementado em `rpo_key_hook_v11.cpp`
- Captura o ponteiro `cipher` passado para a função
- Tentativa de ler nome da estrutura `EVP_CIPHER`

### 2. Estrutura EVP_CIPHER
```c
struct EVP_CIPHER {
    const char* name;      // Offset 0
    int nid;               // Offset 8
    int block_size;        // Offset 12
    int ctx_offset;        // Offset 16
    // ... outros campos
};
```

### 3. Resultados

| Versão | EVP_EncryptInit_ex | Cipher Name | Status |
|--------|-------------------|-------------|--------|
| 24.3.1.1 | ❌ Não chamado | N/A | Hook não triggera |
| 20.3.2.14 | ✅ Chamado | Parcial | Alguns ciphers identificados |

### 4. Ciphers Identificados (20.3.2.14)
- `cast5_cbc_cipher`
- `des_ede_ecb_cipher`
- `rc4_cipher`
- `bf_ecb_cipher`
- `idea_cbc_cipher` (parcial)

### 5. Limitações
- AppServer 24.3.1.1 usa caminho de criptografia diferente
- EVP_EncryptInit_ex não é chamado
- Necessita análise adicional do path de criptografia

## Próximos Passos

1. **Analisar path alternativo** no 24.3.1.1
   - Break em `tCryptoEVP::Encrypt`
   - Seguir ponteiros manualmente

2. **Heurística por tamanho**
   - Mapear key/IV sizes para ciphers conhecidos
   - DES: key=8, IV=8
   - 3DES: key=24, IV=8
   - RC4: key=5-40, IV=0
   - CAST5: key=16, IV=8

3. **Versão anterior**
   - Testar com appserver 12.1.2210
   - Onde EVP_EncryptInit_ex é chamado normalmente

## Arquivos
- `rpo_key_hook_v11.cpp` - Hook com follow de ponteiros
- `capture_v11.json` - Captura de teste
- `custom_generated.rpo` - RPO gerado durante teste

