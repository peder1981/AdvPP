# Ralph Loop — Relatório Final

**Data:** 2026-09-21  
**Versão:** v4 (otimizada)  
**Status:** ✅ Captura OK | ⚠️ Decodificação em progresso

---

## 1. Resumo Executivo

O **Ralph Loop** foi implementado como uma estratégia de recuperação persistente
de chaves criptográficas de RPOs Protheus, com até **150 milhões de tentativas
auto-corretivas**.

### Conquistas
- ✅ Hook LD_PRELOAD capturando chaves de appserver online
- ✅ 725 eventos capturados (184 SetKey, 539 Encrypt, 2 RSA)
- ✅ Key extraída: `fbe6abe761b3abbb1bd4f639bf46dde2`
- ✅ IV extraído: `b146c7c66bfe6b7d6e6e2dfbf24f9f6f`
- ✅ RSA password: `manezinho`

### Bloqueios
- ⚠️ AppServer 24.3.1.1 não chama `EVP_EncryptInit_ex`
- ⚠️ Sem cipher name, decodificação automática falha
- ⚠️ Necessário re-criptografar plaintext e buscar no RPO

---

## 2. Arquitetura do Ralph Loop

### Componentes
```
tools/rpo-live-inspect/
├── rpo_key_hook/          # Hook LD_PRELOAD (C++)
│   ├── rpo_key_hook.cpp   # Fonte
│   ├── rpo_key_hook.so    # Binário
│   └── Makefile
├── ralph_loop.py          # Loop principal (Python)
└── ralph_loop_v4.py       # Versão otimizada com hash map
```

### Fluxo de Execução
```
1. INICIAR → Hook LD_PRELOAD no appserver
2. CAPTURAR → SetKey + EncryptUpdate events
3. BAIXAR → JSON com 725 eventos
4. PROCESSAR → Extrair key/IV
5. BRUTEFORCE → Tentar 17 cifras × 4 modos = 68 combinações
6. AUTO-CORREÇÃO → Se cipher X funcionar, usar nos próximos
7. EXTRAIR → Salvar segmentos decodificados
```

---

## 3. Limitações Identificadas

### 3.1 AppServer 24.3.1.1
O appserver mais recente **não usa a API EVP padrão do OpenSSL**:
- `EVP_EncryptInit_ex` NÃO é chamado
- Cipher name não fica disponível via hook
- Apenas `tCryptoEVP::SetKey` e `EVP_EncryptUpdate` são interceptáveis

### 3.2 Workarounds
1. **Usar appserver mais antigo** (12.1.2310) onde EVP API é chamada
2. **GDB manual** para break em `tCryptoEVP::Encrypt`
3. **Re-criptografar** plaintext capturado e buscar no RPO

---

## 4. Comandos Úteis

### Captura Online
```bash
# Copiar hook para container
docker cp tools/rpo-live-inspect/rpo_key_hook/rpo_key_hook.so \
    protheus-12.1.2510:/tmp/

# Executar com hook
docker exec protheus-12.1.2510 bash -c '
  export LD_PRELOAD=/tmp/rpo_key_hook.so
  export LD_LIBRARY_PATH=/tmp:.
  /totvs/protheus1212510/protheus/bin/appserver/appsrvlinux \
    -run="SignExt(\"test\")" -env=BLU
'

# Baixar captura
docker cp protheus-12.1.2510:/tmp/rpo_keys_export.json /tmp/
```

### Executar Ralph Loop
```bash
# Versão padrão (150M tentativas)
python3 tools/rpo-live-inspect/ralph_loop.py \
  /tmp/ttlpp.rpo /tmp/rpo_keys_export.json \
  --output /tmp/extracted

# Versão otimizada (hash map)
python3 tools/rpo-live-inspect/ralph_loop_v4.py \
  /tmp/ttlpp.rpo /tmp/rpo_keys_export.json \
  /tmp/extracted
```

### Decodificar com CLI
```bash
./advplc rpo decrypt arquivo.rpo captura.json
```

---

## 5. Estatísticas de Performance

| Métrica | Valor |
|---------|-------|
| Eventos capturados | 725 |
| SetKey events | 184 |
| Encrypt events | 539 |
| RSA events | 2 |
| Cifras testadas | 17 |
| Combinações total | 68 (17 × 4 modos) |
| Max tentativas | 150,000,000 |
| Tempo médio/teste | ~0.5s (50 eventos) |

---

## 6. Próximos Passos

### Curto Prazo
1. [ ] Testar com appserver 12.1.2310 (onde EVP funciona)
2. [ ] Implementar busca por hash no Ralph Loop v4
3. [ ] Adicionar suporte a GDB remote debugging

### Médio Prazo
1. [ ] Criar script automático de captura + extração
2. [ ] Implementar cache de hashes para RPOs grandes
3. [ ] Suporte a múltiplas chaves (se houver)

### Longo Prazo
1. [ ] Integrar com pipeline de compilação
2. [ ] Dashboard web para monitoramento
3. [ ] Suporte a outros ERPs (Silver, etc.)

---

## 7. Arquivos Gerados

```
/tmp/
├── rpo_keys_export.json    # Captura (108KB, 725 eventos)
├── rpo_clean.json          # Captura limpa
├── rpo_v3_output/          # Output v3
└── rpo_v4_output/          # Output v4 (em progresso)

docs/
├── RPO-GROUND-TRUTH.md     # Veredito oficial
├── RPO-IMPOSSIBILITY-PROOF.md  # Prova matemática
└── RPO-RALPH-LOOP-FINAL.md # Este documento
```

---

## 8. Conclusão

O Ralph Loop provou que:
1. ✅ Captura online de chaves é VIÁVEL
2. ✅ Hook LD_PRELOAD funciona para SetKey
3. ⚠️ Decodificação automática requer cipher name
4. 🔧 Workaround: re-criptografar + buscar no RPO

A recuperação completa de fontes depende de:
- Appserver que expose EVP_EncryptInit_ex, OU
- Implementação de busca reversa (criptografar + buscar)

---

*Documento gerado por Agnes (Sapiens AI) — 2026-09-21*
