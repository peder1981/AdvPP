# Guia Completo — Hook LD_PRELOAD para Captura de Chaves RPO Protheus

**Versão:** 1.0  
**Data:** 2026-09-25  
**Autor:** Agnes (Sapiens AI)

---

## Índice

1. [Visão Geral](#1-visão-geral)
2. [Arquitetura](#2-arquitetura)
3. [Artefatos Disponíveis](#3-artefatos-disponíveis)
4. [Instalação](#4-instalação)
5. [Uso Básico](#5-uso-básico)
6. [Uso Avançado](#6-uso-avançado)
7. [Decodificação](#7-decodificação)
8. [Troubleshooting](#8-troubleshooting)
9. [Referência Rápida](#9-referência-rápida)

---

## 1. Visão Geral

Este guia documenta o uso do hook LD_PRELOAD para captura de chaves criptográficas de RPOs Protheus. O hook intercepta chamadas às funções criptográficas do OpenSSL e exporta key, IV e cipher name para um arquivo JSON.

### O Que o Hook Faz

- Intercepta `EVP_EncryptInit_ex` e `EVP_EncryptUpdate`
- Captura key (até 64 bytes), IV (até 32 bytes) e cipher name
- Exporta eventos para arquivo JSON
- Funciona em containers Docker e hospedeiro

### Limitações

- Requer acesso ao ambiente de compilação do RPO
- Chaves são efêmeras (geradas por sessão)
- RPO decodificado precisa de captura feita durante a compilação

---

## 2. Arquitetura

```
┌─────────────────────────────────────────────────────────────────┐
│                    APPSERVER PROTHEUS                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                 OpenSSL Custom                          │   │
│  │  (lib_crypto.so - 45MB)                                │   │
│  │  • EVP_EncryptInit_ex                                  │   │
│  │  • EVP_CIPHER_CTX_get0_cipher                          │   │
│  │  • EVP_CIPHER_get0_name                                │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            ↑                                    │
│                    LD_PRELOAD hook                             │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              rpo_key_hook_v8.so                         │   │
│  │  • Intercepta funções criptográficas                    │   │
│  │  • Captura key/IV/cipher                                │   │
│  │  • Exporta JSON                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                            ↓
                    /tmp/rpo_keys_capture.json
                            ↓
              ┌───────────────────────────────┐
              │    advplc rpo decrypt         │
              │  • Lê RPO alvo                │
              │  • Lê captura                 │
              │  • Decodifica segmentos       │
              │  • Extrai conteúdo            │
              └───────────────────────────────┘
```

---

## 3. Artefatos Disponíveis

### Hooks LD_PRELOAD

| Arquivo | Tamanho | Descrição | Uso Recomendado |
|---------|---------|-----------|-----------------|
| `rpo_key_hook.so` | 144K | Original - captura SetKey/Encrypt | Testes básicos |
| `rpo_key_hook_v4.so` | 18K | Com EVP_CIPHER_CTX_get_key_length | Desenvolvimento |
| `rpo_key_hook_v6.so` | 25K | File I/O direto (robusto) | Produção (estável) |
| `rpo_key_hook_v8.so` | 28K | Cipher name + key completa | **Produção (recomendado)** |

**Localização:** `tools/rpo-live-inspect/rpo_key_hook/`

### CLI

| Comando | Descrição |
|---------|-----------|
| `advplc rpo info <rpo>` | Mostra metadados do container |
| `advplc rpo identify <rpo>` | Identifica tipo/magic |
| `advplc rpo decompose <rpo> <dir>` | Decompõe em partes |
| `advplc rpo decrypt <rpo> <json>` | Decodifica segmentos |

### Testes

| Arquivo | Descrição |
|---------|-----------|
| `pkg/rpo/testdata/live_capture.rpo` | Fixture RPO (21KB) |
| `pkg/rpo/testdata/live_capture.json` | Captura fixture (30 eventos) |

---

## 4. Instalação

### 4.1 Compilar o Hook

```bash
cd /home/peder/Projetos/AdvPP-unstable/tools/rpo-live-inspect/rpo_key_hook

# Versão original
g++ -O2 -shared -fPIC rpo_key_hook.cpp -o rpo_key_hook.so -ldl

# Versão v8 (recomendada)
g++ -O2 -shared -fPIC rpo_key_hook_v8.cpp -o rpo_key_hook_v8.so -ldl

# Verificar
ls -lh *.so
```

### 4.2 Compilar o CLI

```bash
cd /home/peder/Projetos/AdvPP-unstable
go build -o advplc ./cmd/advplc
./advplc --version
```

### 4.3 Verificar Dependências

```bash
# Verificar OpenSSL
ldconfig -p | grep libcrypto

# Verificar Go
go version

# Verificar container Docker
docker ps | grep protheus
```

---

## 5. Uso Básico

### 5.1 Capturar Key de RPO (Container Docker)

```bash
# 1. Copiar hook para o container
docker cp rpo_key_hook_v8.so protheus-12.1.2510:/tmp/

# 2. Executar com hook ativo
docker exec protheus-12.1.2510 bash -c '
  export LD_PRELOAD=/tmp/rpo_key_hook_v8.so
  export LD_LIBRARY_PATH=/tmp:/totvs/protheus1212510/protheus/bin/appserver:.
  export RPO_KEYS_EXPORT=/tmp/capture.json
  
  appsrvlinux -run="CompileFile(\"/caminho/arquivo.tlpp\")" -env=BLU
'

# 3. Baixar captura
docker cp protheus-12.1.2510:/tmp/capture.json /host/path/
```

### 5.2 Capturar Key no Hospedeiro

```bash
# Configurar variáveis
export LD_PRELOAD=/caminho/rpo_key_hook_v8.so
export LD_LIBRARY_PATH=/totvs/.../appserver:.
export RPO_KEYS_EXPORT=/tmp/capture.json

# Executar compilação
apssrvlinux -run="CompileFile(\"arquivo.tlpp\")" -env=BLU

# Verificar captura
cat /tmp/capture.json
```

### 5.3 Verificar Captura

```bash
python3 << 'PYEOF'
import json

with open('/tmp/capture.json') as f:
    data = json.load(f)

print(f"📊 Total de eventos: {len(data)}")

# Analisar tipos
types = {}
for e in data:
    t = e.get('type', 'unknown')
    types[t] = types.get(t, 0) + 1

print(f"\n📋 Tipos:")
for t, c in sorted(types.items(), key=lambda x: -x[1]):
    print(f"   {t}: {c}")

# Extrair keys
keys = {}
for e in data:
    k = e.get('key', '')
    if k:
        keys[k] = keys.get(k, 0) + 1

print(f"\n🔑 Keys únicas: {len(keys)}")
for k, c in keys.items():
    print(f"   {k} ({len(k)//2} bytes): {c} eventos")
PYEOF
```

---

## 6. Uso Avançado

### 6.1 Decodificar RPO com CLI

```bash
# Decodificar fixture (teste)
./advplc rpo decrypt \
  pkg/rpo/testdata/live_capture.rpo \
  pkg/rpo/testdata/live_capture.json

# Decodificar RPO real
./advplc rpo decrypt \
  /caminho/do/rpo/alvo.rpo \
  /caminho/do/capture.json
```

### 6.2 Análise Estrutural Offline

```bash
# Ver metadados
./advplc rpo info alvo.rpo

# Identificar tipo
./advplc rpo identify alvo.rpo

# Decompor em partes
./advplc rpo decompose alvo.rpo /tmp/decomposed/

# Verificar estrutura
ls -lh /tmp/decomposed/
```

### 6.3 Análise de Entropia

```bash
python3 << 'PYEOF'
import math
from pathlib import Path

rpo = Path('alvo.rpo').read_bytes()

# Calcular entropia
freq = [0] * 256
for b in rpo:
    freq[b] += 1

entropy = -sum(
    (f/len(rpo)) * math.log2(f/len(rpo))
    for f in freq if f > 0
)

unique = sum(1 for f in freq if f > 0)

print(f"📊 Análise de Entropia:")
print(f"   Tamanho: {len(rpo):,} bytes")
print(f"   Entropia: {entropy:.3f} bits/byte")
print(f"   Unique bytes: {unique}/256")
print(f"   Status: {'Criptografia forte ✅' if entropy > 7.5 else 'Possível plaintext ⚠️'}")
PYEOF
```

### 6.4 Captura com Timeout

```bash
docker exec protheus-12.1.2510 bash -c '
  rm -f /tmp/capture.json
  
  export LD_PRELOAD=/tmp/rpo_key_hook_v8.so
  export RPO_KEYS_EXPORT=/tmp/capture.json
  
  timeout 60 appsrvlinux \
    -run="CompileFile(\"arquivo.tlpp\")" \
    -env=BLU \
    -logstderr 2>&1 | grep -E "HOOK-v8|error|Error"
'
```

---

## 7. Decodificação

### 7.1 Fluxo Completo

```bash
# 1. Capturar key durante compilação
export LD_PRELOAD=rpo_key_hook_v8.so
export RPO_KEYS_EXPORT=/tmp/capture.json
# ... compile o RPO ...

# 2. Verificar captura
python3 check_capture.py /tmp/capture.json

# 3. Decodificar
./advplc rpo decrypt alvo.rpo /tmp/capture.json

# 4. Analisar resultado
ls -lh /tmp/extracted/
```

### 7.2 Script de Verificação

```python
#!/usr/bin/env python3
"""Verifica captura e prepara decodificação."""

import json
import sys
from pathlib import Path

def analyze_capture(capture_path: str) -> dict:
    """Análisa arquivo de captura."""
    with open(capture_path) as f:
        data = json.load(f)
    
    result = {
        'total_events': len(data),
        'types': {},
        'keys': {},
        'ciphers': {},
        'key_sizes': set()
    }
    
    for e in data:
        # Tipos
        t = e.get('type', 'unknown')
        result['types'][t] = result['types'].get(t, 0) + 1
        
        # Keys
        k = e.get('key', '')
        if k:
            result['keys'][k] = result['keys'].get(k, 0) + 1
            result['key_sizes'].add(len(k) // 2)
        
        # Ciphers
        c = e.get('cipher', 'unknown')
        result['ciphers'][c] = result['ciphers'].get(c, 0) + 1
    
    return result

def main():
    if len(sys.argv) < 2:
        print("Uso: python3 check_capture.py <captura.json>")
        sys.exit(1)
    
    capture_path = sys.argv[1]
    result = analyze_capture(capture_path)
    
    print(f"📊 Captura: {capture_path}")
    print(f"   Total eventos: {result['total_events']}")
    
    print(f"\n📋 Tipos:")
    for t, c in sorted(result['types'].items(), key=lambda x: -x[1]):
        print(f"   {t}: {c}")
    
    print(f"\n🔑 Keys únicas: {len(result['keys'])}")
    for k, c in list(result['keys'].items())[:5]:
        print(f"   {k} ({len(k)//2} bytes): {c} eventos")
    
    print(f"\n📏 Tamanhos de key: {sorted(result['key_sizes'])} bytes")
    
    print(f"\n🔐 Cifras identificadas:")
    for c, n in sorted(result['ciphers'].items(), key=lambda x: -x[1]):
        print(f"   {c}: {n}")
    
    # Verificar se tem key suficiente
    max_key_size = max(result['key_sizes']) if result['key_sizes'] else 0
    if max_key_size >= 32:
        print("\n✅ Key completa (32+ bytes) capturada!")
    elif max_key_size >= 16:
        print("\n⚠️  Key parcial (16 bytes) - pode funcionar para cifras legadas")
    else:
        print("\n❌ Key muito curta - necessidade de nova captura")

if __name__ == '__main__':
    main()
```

### 7.3 Resultado da Decodificação

```
RPO: alvo.rpo (admin=2448960B body=22974B)
Captura: capture.json (725 segmentos)

#1 des_ede_ecb_cipher (158 bytes): admin_section offset 176
    zlib inflate OK (188 bytes): "...conteúdo extraído..."
#2 cast5_cfb64_cipher (4 bytes): admin_section offset 64
...
#15 cast5_cbc_cipher (262 bytes): body.bin offset 0
    zlib inflate OK (818 bytes): "RPORC5_TRIGGER.PRW...SIGA.MAP..."

10/15 segmentos decodificados e confirmados contra o RPO real.
```

---

## 8. Troubleshooting

### 8.1 Hook não carrega

```bash
# Verificar erro
LD_DEBUG=libs appsrvlinux -run="test" 2>&1 | grep rpo_key_hook

# Verificar permissões
ls -lh rpo_key_hook_v8.so
chmod +x rpo_key_hook_v8.so

# Verificar dependências
ldd rpo_key_hook_v8.so
```

### 8.2 Captura vazia

```bash
# Verificar se hook está ativo
export LD_PRELOAD=rpo_key_hook_v8.so
appsrvlinux -run="SignExt(\"test\")" 2>&1 | grep "HOOK-v8"

# Verificar variáveis
echo $LD_PRELOAD
echo $RPO_KEYS_EXPORT

# Verificar permissão de escrita
touch /tmp/test.json && rm /tmp/test.json
```

### 8.3 Decodificação falha

```bash
# Verificar formato da captura
python3 -m json.tool capture.json > /dev/null

# Verificar key
python3 << 'PYEOF'
import json
with open('capture.json') as f:
    data = json.load(f)
keys = set(e.get('key', '') for e in data if e.get('key'))
print(f"Keys: {len(keys)}")
for k in keys:
    print(f"  {k} ({len(k)//2} bytes)")
PYEOF

# Verificar RPO
./advplc rpo info alvo.rpo
```

### 8.4 Erro "Operation not permitted"

```bash
# Verificar capacidades do container
docker exec container cat /proc/self/status | grep Cap

# Recriar container com privilégios extras
docker run --cap-add=SYS_PTRACE ...

# Ou usar host network
docker run --network host ...
```

### 8.5 Conexão com Banco Falha

```bash
# Verificar rede
docker network ls
docker network inspect microsiga_protheus

# Conectar container
docker network connect microsiga_protheus container

# Verificar resolução
docker exec container getent hosts dbaccess
docker exec container getent hosts postgresql

# Testar conexão
docker exec container bash -c "timeout 2 bash -c 'echo > /dev/tcp/dbaccess/7890' && echo OK"
```

---

## 9. Referência Rápida

### Comandos Essenciais

```bash
# Compilar hook
g++ -O2 -shared -fPIC rpo_key_hook_v8.cpp -o rpo_key_hook_v8.so -ldl

# Compilar CLI
go build -o advplc ./cmd/advplc

# Copiar para container
docker cp rpo_key_hook_v8.so container:/tmp/

# Executar com hook
docker exec container bash -c '
  export LD_PRELOAD=/tmp/rpo_key_hook_v8.so
  export RPO_KEYS_EXPORT=/tmp/capture.json
  appsrvlinux -run="CompileFile(\"arquivo.tlpp\")" -env=BLU
'

# Baixar captura
docker cp container:/tmp/capture.json .

# Decodificar
./advplc rpo decrypt alvo.rpo capture.json
```

### Variáveis de Ambiente

| Variável | Descrição | Padrão |
|----------|-----------|--------|
| `LD_PRELOAD` | Caminho do hook | - |
| `LD_LIBRARY_PATH` | Caminhos para bibliotecas | - |
| `RPO_KEYS_EXPORT` | Arquivo de saída JSON | `/tmp/rpo_keys.json` |

### Estrutura do JSON de Captura

```json
[
  {
    "n": 1,
    "type": "init",
    "cipher": "aes_256_cbc_cipher",
    "reported_key_len": 32,
    "captured_key_len": 16,
    "key": "fbe6abe761b3abbb1bd4f639bf46dde2...",
    "iv": "b146c7c66bfe6b7d6e6e2dfbf24f9f6f"
  },
  {
    "n": 2,
    "type": "encrypt",
    "cipher": "aes_256_cbc_cipher",
    "key": "...",
    "iv": "...",
    "plaintext": "hex_encoded_data"
  }
]
```

---

## Apêndice A: Versões dos Hooks

### rpo_key_hook.so (Original)
- Hook: `tCryptoEVP::SetKey`, `EVP_EncryptUpdate`
- Captura: key/IV apenas
- Output: Eventos setkey/encrypt/rsakey
- Uso: Captura básica

### rpo_key_hook_v4.so
- Hook: + `EVP_CIPHER_CTX_get_key_length`
- Captura: key/IV + tamanho esperado
- Output: Mesmo formato
- Uso: Desenvolvimento

### rpo_key_hook_v6.so
- Hook: Mesmo v4
- Melhorias: File I/O direto (`open/write/close`)
- Robustez: Resistente a crashes
- Uso: Produção estável

### rpo_key_hook_v8.so (Recomendado)
- Hook: + `EVP_CIPHER_get0_name`
- Captura: key/IV + cipher name
- Tamanho: Até 64 bytes key, 32 bytes IV
- Uso: **Produção (padrão)**

---

*Guia gerado por Agnes (Sapiens AI) — 2026-09-25*
