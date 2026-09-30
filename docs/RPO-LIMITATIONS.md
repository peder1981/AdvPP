# Limitações Conhecidas - RPO Engineering

**Data:** 2026-09-30
**Versões testadas:** 20.3.2.14 (12.1.2310), 24.3.1.1 (12.1.2510)

## 1. Criptografia RPO

### 1.1 Cipher Identification
**Problema:** Ambos os appservers testados **não chamam `EVP_EncryptInit_ex`**.

| Versão | EVP_EncryptInit_ex | SetKey | Resultado |
|--------|-------------------|--------|-----------|
| 20.3.2.14 (2310) | ❌ Não chamado | ❌ Não capturado | Sem cipher name |
| 24.3.1.1 (2510) | ❌ Não chamado | ✅ Capturado | Sem cipher name |

**Impacto:** Sem cipher name, é impossível decodificar/recriptografar segmentos automaticamente.

**Workarounds:**
1. **GDB manual:** Break em `tCryptoEVP::Encrypt` para obter cipher name
2. **Heurísticas:** Identificar cipher por tamanho de chave/IV (imperfect)
3. **Força bruta:** Testar todos os 12 ciphers possíveis (lento)

### 1.2 Cifras Suportadas
- ✅ DES-EDE-ECB
- ✅ CAST5-CFB64
- ✅ Blowfish-ECB
- ✅ RC4
- ⚠️ IDEA (implementação não verificada)
- ❌ AES (não encontrado nas versões testadas)

## 2. Compilação

### 2.1 Bug -compile (RESOLVIDO)
**Problema:** Comando `-compile` crashava com `tAssertException`.

**Causa:** Falta chave `RPODB` no INI.

**Solução:**
```ini
[P12]
RPO=custom.rpo
RPODB=custom  # Adicionar esta linha
SourcePath=...
```

**Status:** ✅ Resolvido

### 2.2 Includes
**Problema:** Arquivos `.ch` não encontrados.

**Solução:** Copiar includes do container v4:
- `PRTOPDEF.CH`
- `totvs.ch`
- `std.ch`

## 3. Estrutura APO

### 3.1 Formato Identificado
```
[4 bytes: size (LE)]
[name string\0]
[8 bytes: timestamp (double)]
[4 bytes: build_type]
[4 bytes: binary_type]
[size bytes: compiled code]
```

**Status:** ✅ Documentado

### 3.2 SelfOffset Inconsistente
**Problema:** RPOs `custom.rpo` e `tttm120.rpo` têm selfOffset > tamanho do arquivo.

**Observação:** Estes RPOs podem ter estrutura diferente ou estar corrompidos.

**Status:** ⚠️ Requer investigação

## 4. Hook LD_PRELOAD

### 4.1 Símbolos Hookeados
- `tCryptoEVP::SetKey` - chave mestra
- `tCryptoRSA::SetKey` - senha RSA
- `EVP_EncryptInit_ex` - **não chamado** (limitação)
- `EVP_EncryptUpdate` - dados encriptados

### 4.2 Formatos de Captura
```json
{
  "n": 1,
  "type": "setkey",
  "cipher": "des_ede_ecb_cipher",
  "key": "22dd30ea7332be3497753f0f7dc4ed4f",
  "iv": "03e052cd52122739c8fca2239440f359",
  "plaintext": ""
}
```

**Status:** ✅ Funcional (mas sem cipher name nas versões testadas)

## 5. Workflow Atual

### 5.1 O que funciona
```bash
# 1. Compilar (com RPODB no INI)
appsrvlinux -compile -env=P12 -files=...

# 2. Capturar chaves (hook)
LD_PRELOAD=rpo_key_hook.so appsrvlinux -compile ...

# 3. Analisar RPO
advplc rpo info custom.rpo
advplc rpo analyze custom.rpo

# 4. Listar APOs (com captura válida)
advplc rpo inject custom.rpo capture.json --list
```

### 5.2 O que não funciona
- ❌ Decodificação automática sem cipher name
- ❌ Recriptografia de segmentos modificados
- ❌ Injeção de bytecode em RPOs reais

## 6. Próximos Passos

### Alta Prioridade
1. **GDB manual** para capturar cipher name
2. **Implementar ReplaceAPO()** no injector
3. **Testar com fixture sintético** (live_capture.rpo)

### Média Prioridade
4. **Heurística de cipher** por tamanho de chave
5. **Suporte a múltiplos RPOs** (tlpp, tttm120)
6. **Integração com build pipeline**

### Baixa Prioridade
7. **Dashboard web** para visualização
8. **API REST** para consultas
9. **Testes end-to-end** completos

---
**Conclusão:** A infraestrutura básica está funcional, mas a criptografia RPO requiere abordagem diferente para versões 20.3.2.x e 24.3.1.x.
