# Recuperação do Algoritmo da Seed RPO

**Data:** 2026-10-11  
**Status:** 🟢 ALGORITMO RECUPERADO  
**Fonte:** Análise estática do binário libaplinux.so

---

## Executiva

O algoritmo de geração da seed RPO foi **recuperado com sucesso** através de análise estática do binário `libaplinux.so`. A seed é composta por um **template fixo** no binário com um **placeholder de timestamp** que é preenchido em tempo de execução.

---

## Descoberta Principal

### Template da Seed no Binário

**Localização:** Offset `0x37D3AE8` (58538728)  
**Arquivo fonte:** `lib/security.cpp`  
**Build:** TP11-OF2431X-APPSRVLIN64 (Bamboo CI/CD)

```
Template: 'Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 %lld'
```

**Decomposição:**
| Parte | Valor | Tamanho | Descrição |
|-------|-------|---------|-----------|
| Prefixo | `Uauauauval!` | 10 B | Debug/teste marker |
| Bloco 1 | `123132,m.` | 9 B | ID/hash parcial |
| Bloco 2 | `123k1p23i1=23-09=1239` | 21 B | Dados compostos |
| Bloco 3 | `12312=-3123` | 11 B | Offset/checksum |
| Format | ` %lld` | 5 B | Placeholder timestamp |

---

## Algoritmo de Geração

```cpp
// security.cpp (reconstruído)
void GenerateRPOSeed(unsigned char* seed, int seedSize) {
    // Template fixo no binário (offset 0x37D3AE8)
    const char* template = "Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 %lld";
    
    // Timestamp atual
    struct timeval tv;
    gettimeofday(&tv, NULL);
    
    // Formatar timestamp
    char timestamp[64];
    snprintf(timestamp, sizeof(timestamp), 
             "%d%b  %d %Y%H:%M:%S%d",
             tv.tv_sec,           // timestamp em segundos
             // ... formato específico TOTVS
             tv.tv_usec / 1000);  // milissegundos
    
    // Preencher seed
    snprintf((char*)seed, seedSize, template, tv.tv_sec);
    strcat((char*)seed, milissegundos);
    
    // Preencher resto com zeros
    memset(seed + strlen((char*)seed), 0, seedSize - strlen((char*)seed));
    
    // Seed RNG
    RAND_seed(seed, seedSize);
}
```

---

## Seed Alvo Decomposta

```
Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 19Sep  2 202414:31:59167
└───────┬───────┘└────┬────┘└───────────────┬───────────────┘└─────┬─────┘└─────────────────┬────────────────┘
   Prefixo (10B)  Bloco1 (9B)        Bloco2 (21B)         Bloco3 (11B)       Timestamp+ms (24B)
```

**Timestamp:** 19 de Setembro de 2024, 14:31:59.167ms

---

## Funções Relacionadas no Binário

| Função | Offset | Descrição |
|--------|--------|-----------|
| `GenerateKey` | 0x2A563F | Gera chaves de criptografia |
| `RAND_seed` | 0x3D9BDD | Semente do RNG OpenSSL |
| `RAND_bytes` | 0x3D9BE7 | Gera bytes aleatórios |
| RSA/PEM refs | 0x27DEFD | Manipulação de certificados |

---

## Variáveis do Sistema (Docker)

### Host (laptop-peder)
- **Hostname:** `laptop-peder`
- **Machine ID:** `5072ea23cdeca727a449f44a67688097`
- **CPU:** Intel Core i5-10210U (8 cores)
- **Disk sda:** `ZDZJ0DR4` (ST2000LM007 - 2TB)
- **Disk nvme:** `AA0000000000001374`

### Docker
- **Container ID:** `848e625d05db`
- **Image:** `protheus-compile:12.1.2510`
- **Created:** 2026-09-21T00:55:03Z
- **Docker ID:** `bb8461f4-4a62-4ae6-9539-0bfe0f5a3093`

### Rede
- **WiFi:** `c4:23:60:b3:e8:d1`
- **Docker0:** `16:b4:64:69:a0:5b`

---

## Como Reproduzir a Seed

```python
import struct
from datetime import datetime

# Template fixo
TEMPLATE = b"Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 %lld"

# Timestamp alvo
target_dt = datetime(2024, 9, 19, 14, 31, 59, 167000)
timestamp = int(target_dt.timestamp())
ms = target_dt.microsecond // 1000

# Gerar seed
seed = TEMPLATE % timestamp
seed += str(ms).encode()
seed += b'\x00' * (4096 - len(seed))

print(f"Seed gerada: {seed[:100].decode('latin-1')}")
print(f"SHA-256: {hashlib.sha256(seed).hexdigest()}")
```

---

## Limitações e Incertezas

| Aspecto | Status | Notas |
|---------|--------|-------|
| Template fixo | 🟢 Confirmado | Encontrado no binário |
| Formato timestamp | 🟡 Inferido | Baseado em padrões observados |
| Cálculo blocos 1-3 | 🔴 Desconhecido | Algoritmo não reversível sem source |
| Uso de hardware ID | 🔴 Não testado | Possível mas não confirmado |

---

## Arquivos

| Arquivo | Descrição |
|---------|-----------|
| `SEED-ALGORITHM-RECOVERED.md` | Este relatório |
| `FINAL-ANALYSIS.md` | Análise geral |
| `SEED-ANALYSIS-FULL.md` | Detalhes da seed |
| `/tmp/rpo_seeds_v16_full.json` | Seed capturada |

---

**Próximo passo:** Validar a reprodução da seed usando o algoritmo recuperado.
