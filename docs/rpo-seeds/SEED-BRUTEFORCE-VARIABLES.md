# Variáveis para Brute-Force da Seed RPO

**Data:** 2026-10-11  
**Status:** 🟡 INFERIDO - Variáveis coletadas, algoritmo desconhecido

---

## Seed Alvo

```
Primeiros 100 bytes (de 4096):
Uauauauau!123132,m. 123k1p23i1=23-09=1239 12312=-3123 19Sep  2 202414:31:59167

Resto (bytes 100-4095):
Padding nulo (0x00)
```

**Timestamp decodificado:** 19 de Setembro de 2024, 14:31:59.167ms

---

## Variáveis do Sistema Coletadas

### Identificadores Únicos
| Variável | Valor | Fonte |
|----------|-------|-------|
| **Hostname** | `laptop-peder` | `hostname` |
| **Machine ID** | `5072ea23cdeca727a449f44a67688097` | `/etc/machine-id` |
| **Docker ID** | `bb8461f4-4a62-4ae6-9539-0bfe0f5a3093` | `docker info` |

### Discos
| Device | Serial | Modelo |
|--------|--------|--------|
| `/dev/sda` | `ZDZJ0DR4` | ST2000LM007 ( HDD 2TB) |
| `/dev/nvme0n1` | `AA0000000000001374` | NVMe |

### Redes (MAC Addresses)
| Interface | MAC |
|-----------|-----|
| `wlp0s20f3` (WiFi) | `c4:23:60:b3:e8:d1` |
| `docker0` (Bridge) | `16:b4:64:69:a0:5b` |
| `br-f4c92a97e43e` | `e2:91:33:b8:63:43` |
| `br-8a776af16292` | `f2:fb:b7:1a:a5:7c` |

### Sistema
| Variável | Valor |
|----------|-------|
| **Kernel** | `7.1.5-76070105-generic` |
| **Entropy disponível** | `256` bits |
| **Tamanho do pool** | `256` bits |

---

## Decomposição da Seed

```
┌─────────────────────────────────────────────────────────────────────┐
│ OFFSET │ TAMANHO │ CONTEÚDO                              │
├─────────────────────────────────────────────────────────────────────┤
│ 0x00   │ 10 B    │ "Uauauauau!" (prefixo debug)           │
│ 0x0A   │ 9 B     │ "123132,m." (bloco 1)                │
│ 0x13   │ 1 B     │ " " (espaço)                         │
│ 0x14   │ 21 B    │ "123k1p23i1=23-09=1239" (bloco 2)    │
│ 0x29   │ 1 B     │ " " (espaço)                         │
│ 0x2A   │ 11 B    │ "12312=-3123" (bloco 3)              │
│ 0x35   │ 1 B     │ " " (espaço)                         │
│ 0x36   │ 21 B    │ "19Sep  2 202414:31:59" (timestamp)  │
│ 0x4D   │ 3 B     │ "167" (milissegundos)                │
│ 0x50   │ ~56 B   │ Padding/transição                    │
│ 0x80   │ 4096 B  │ Padding nulo (0x00)                  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Hipóteses para os Blocos

### Bloco 1: "123132,m."
- Pode ser truncated hash (6 chars) + suffix ",m."
- ",m." pode significar: "memo", "main", "machine"

### Bloco 2: "123k1p23i1=23-09=1239"
- Contém "=23-09=" que poderia ser data (23 de setembro)
- Mas timestamp principal é 19 de setembro
- Pode ser: timestamp de evento anterior, ou ID composto

### Bloco 3: "12312=-3123"
- Formato "num=-num" sugere: offset, diferença, checksum

---

## Algoritmo Provável

A seed parece ser gerada por:

```c
void generate_seed(unsigned char* seed, int len) {
    // 1. Copiar buffer de teste
    memcpy(seed, "Uauauauau!", 10);
    
    // 2. Coletar entropy do sistema
    collect_entropy(seed + 10, 9);   // bloco 1
    collect_entropy(seed + 19, 21);  // bloco 2
    collect_entropy(seed + 40, 11);  // bloco 3
    
    // 3. Adicionar timestamp
    sprintf(seed + 51, "%d%b  %d %Y%H:%M:%S", 
            dt->tm_mday, months[dt->tm_mon], 
            dt->tm_year + 1900,
            dt->tm_hour, dt->tm_min, dt->tm_sec);
    
    // 4. Adicionar milissegundos
    sprintf(seed + 72, "%03d", dt->tm_ms);
    
    // 5. Preencher resto com zeros
    memset(seed + 75, 0, len - 75);
}
```

---

## Próximos Passos

1. **Coletar mais amostras** de seeds de diferentes compilações
2. **Analisar correlações** entre variáveis do sistema e conteúdo dos blocos
3. **Reverter o gerador** para encontrar a função exata
4. **Testar combinações** das variáveis coletadas com diferentes hash functions

---

## Arquivos de Referência

| Arquivo | Descrição |
|---------|-----------|
| `SEED-ANALYSIS-FULL.md` | Análise estrutural completa |
| `rpo_seeds_v16_full.json` | Seed capturada (4096 bytes) |
| `rpo_key_hook_v16.so` | Hook que capturou a seed |

---

**Nota:** Esta seed contém "Uauauauau!" no início, indicando que foi gerada em ambiente de **teste/debug**, não produção. Isso aumenta significativamente a probabilidade de reprodução, pois os dados podem ser pseudo-aleatórios determinísticos.
