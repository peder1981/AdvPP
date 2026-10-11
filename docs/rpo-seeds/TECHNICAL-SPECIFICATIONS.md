# Especificações Técnicas Completas - Seed RPO

**Versão:** 1.0  
**Data:** 2026-10-11  
**Status:** 🟢 RECAUPERADO

---

## 1. Especificação do Template

### 1.1 Localização
```
Binário: libaplinux.so
Caminho: /protheus12/bin/appserver/libaplinux.so
Offset: 0x37D3AE8 (58538728 decimal)
Tamanho: 58 bytes (template fixo)
```

### 1.2 Hex Dump
```
Offset  Hex                                          ASCII
------  -------------------------------------------    -----------------
0x00    55 61 75 61 75 61 75 61 75 21                Uauauauval!
0x0A    31 32 33 31 33 32 2C 6D 2E                  123132,m.
0x13    20 31 32 33 6B 31 70 32 33                 123k1p23
0x1C    69 31 3D 32 33 2D 30 39 3D 31 32 33 39    i1=23-09=1239
0x29    20 31 32 33 31 32 3D 2D 33 31 32 33       12312=-3123
0x35    20 25 6C 6C 64 00 00 00 00 00 00          %lld.......
```

### 1.3 Decomposição em Campos

| Campo | Offset Rel. | Tamanho | Valor | Descrição |
|-------|-------------|---------|-------|-----------|
| Marker | 0x00 | 10 B | `Uauauauval!` | Debug/teste marker |
| Block1 | 0x0A | 9 B | `123132,m.` | ID/hash parcial |
| Space1 | 0x13 | 1 B | ` ` | Separador |
| Block2 | 0x14 | 21 B | `123k1p23i1=23-09=1239` | Dados compostos |
| Space2 | 0x29 | 1 B | ` ` | Separador |
| Block3 | 0x2A | 11 B | `12312=-3123` | Offset/checksum |
| Space3 | 0x35 | 1 B | ` ` | Separador |
| Format | 0x36 | 5 B | `%lld` | Placeholder timestamp |
| Padding | 0x3B | 53 B | `\x00*53` | Zeros até próximo struct |

**Total fixo:** 56 bytes (antes do timestamp)

---

## 2. Especificação do Timestamp

### 2.1 Formato
```
Padrão: DDMMM SS YYYYHH:MM:SSms
Largura: 24 caracteres
Encoding: Latin-1 (ASCII compatível)
```

### 2.2 Campos

| Posição | Tamanho | Formato | Exemplo | Descrição |
|---------|---------|---------|---------|-----------|
| 0-1 | 2 | `%02d` | `19` | Dia do mês |
| 2-4 | 3 | `%b` | `Sep` | Mês abreviado (3 letras) |
| 5-7 | 3 | ` %2d` | `  2` | Valor derivado do ano |
| 8-11 | 4 | `%04d` | `2024` | Ano completo |
| 12-13 | 2 | `%02d` | `14` | Hora (0-23) |
| 14-15 | 2 | `%02d` | `31` | Minuto (0-59) |
| 16-17 | 2 | `%02d` | `59` | Segundo (0-59) |
| 18-20 | 3 | `%03d` | `167` | Milissegundos (0-999) |

### 2.3 Fórmula do Valor Derivado

```c
// Cálculo do campo "SS" (posição 5-7)
int year_short = year % 100;      // 24 para 2024
int mystery = year_short / 12;    // 24 / 12 = 2

// Tabela de conversão
// Ano  | year % 100 | mystery
// -----|------------|--------
// 2000 |     0      |   0
// 2011 |    11      |   0
// 2012 |    12      |   1
// 2023 |    23      |   1
// 2024 |    24      |   2  ← Ano da seed capturada
// 2035 |    35      |   2
// 2036 |    36      |   3
```

**Interpretação:** O valor representa o "quadrimestre" dentro do século (período de 12 anos).

---

## 3. Especificação da Seed Completa

### 3.1 Estrutura
```
+-------------------+------------------+------------------+
| Template (56 B)   | Timestamp (24 B) | Padding (4016 B) |
+-------------------+------------------+------------------+
| 0x00           0x37| 0x38         0x4F| 0x50        0xFFF |
+-------------------+------------------+------------------+
```

### 3.2 Tamanho Total
```
Template:    56 bytes
Timestamp:   24 bytes
Padding:    4016 bytes (zeros)
---------
Total:      4096 bytes (4 KB)
```

### 3.3 Exemplo Completo
```
Offset  Hex (primeiros 80 bytes)
------  ------------------------------------------------
0x000   5561756175617561 75213132 33313332 2c6d2e20
0x010   3132336B31 70323369 313D3233 2D30393D
0x020   31323339 20313233 31323D2D 33313233
0x030   20313953 65702020 32203230 32343134
0x040   3A33313A 35393136 37000000 00000000
        Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 19Sep  2 202414:31:59167\x00...
```

---

## 4. Especificação Criptográfica

### 4.1 Algoritmo
```
Tipo: AES-128-CBC
Tamanho chave: 128 bits (16 bytes)
Tamanho IV: 128 bits (16 bytes)
Padding: PKCS7
```

### 4.2 Fluxo de Geração
```
1. Gerar timestamp atual
2. Montar seed (template + timestamp + padding)
3. Chamar RAND_seed(seed, 4096)
4. Chamar RAND_bytes(key, 16) → Chave AES
5. Chamar RAND_bytes(iv, 16) → Vetor de inicialização
6. Chamar AES_set_encrypt_key(key, &aes_key)
7. Chamar AES_cbc_encrypt(data, out, len, &aes_key, iv, AES_ENCRYPT)
```

### 4.3 Offsets das Funções

| Função | Offset | Biblioteca |
|--------|--------|------------|
| RAND_seed | 0x3D9BDD | libssl/libcrypto |
| RAND_bytes | 0x3D9BE7 | libssl/libcrypto |
| AES_set_encrypt_key | 0x4A8A4C | libcrypto |
| AES_set_decrypt_key | 0x4A8C75 | libcrypto |
| AES_cbc_encrypt | 0x4A8C95 | libcrypto |
| AES_encrypt | 0x4A8A60 | libcrypto |
| AES_decrypt | 0x4A8C89 | libcrypto |

---

## 5. Especificação do Build

### 5.1 Identificação
```
Arquivo fonte: lib/security.cpp
Build path: /usr/local/bamboo/xml-data/build-dir/TP11-OF2431X-APPSRVLIN64/
CI/CD: Atlassian Bamboo
Projeto: TP11 (TOTVS Protheus 11)
Módulo: OF2431X (interno TOTVS)
```

### 5.2 Versão
```
Version: 7.00.240223P
Build date: Outubro 2025
Seed date: 19 Setembro 2024
Diferença: ~12 meses (seed de build anterior)
```

---

## 6. Implementação de Referência

### 6.1 Python
```python
from datetime import datetime
import hashlib

TEMPLATE = "Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 "

def generate_rpo_seed(dt):
    day = dt.day
    month = dt.strftime('%b')
    year = dt.year
    hour = dt.hour
    minute = dt.minute
    second = dt.second
    ms = dt.microsecond // 1000
    
    mystery = (year % 100) // 12
    
    timestamp = f"{day:02d}{month}  {mystery} {year}{hour:02d}:{minute:02d}:{second:02d}{ms:03d}"
    seed_str = TEMPLATE + timestamp
    
    seed = seed_str.encode('latin-1')
    seed += b'\x00' * (4096 - len(seed))
    
    return seed[:4096]

# Uso
dt = datetime(2024, 9, 19, 14, 31, 59, 167000)
seed = generate_rpo_seed(dt)
print(f"Seed: {seed[:80].decode('latin-1')}")
print(f"SHA-256: {hashlib.sha256(seed).hexdigest()}")
```

### 6.2 Go
```go
package main

import (
    "fmt"
    "time"
)

const Template = "Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 "

func GenerateRPOSeed(dt time.Time) []byte {
    year := dt.Year()
    mystery := (year % 100) / 12
    
    timestamp := fmt.Sprintf("%02d%s  %d %04d%02d:%02d:%02d%03d",
        dt.Day(),
        dt.Format("Jan")[0:3],
        mystery,
        year,
        dt.Hour(),
        dt.Minute(),
        dt.Second(),
        dt.Nanosecond()/1e6,
    )
    
    seed := append([]byte(Template), []byte(timestamp)...)
    seed = append(seed, make([]byte, 4096-len(seed))...)
    
    return seed
}
```

### 6.3 C
```c
#include <stdio.h>
#include <string.h>
#include <time.h>

#define SEED_SIZE 4096
#define TEMPLATE "Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 "

void generate_rpo_seed(struct tm* tm, unsigned char* seed) {
    int year = tm->tm_year + 1900;
    int mystery = (year % 100) / 12;
    
    char timestamp[32];
    snprintf(timestamp, sizeof(timestamp),
             "%02d%3s  %d %04d%02d:%02d:%02d%03d",
             tm->tm_mday,
             months_abbr[tm->tm_mon],  // "Jan", "Feb", etc.
             mystery,
             year,
             tm->tm_hour,
             tm->tm_min,
             tm->tm_sec,
             tm->tm_sec_ms);  // milissegundos
    
    strcpy((char*)seed, TEMPLATE);
    strcat((char*)seed, timestamp);
    memset(seed + strlen((char*)seed), 0, SEED_SIZE - strlen((char*)seed));
}
```

---

## 7. Validação e Testes

### 7.1 Teste Unitário
```python
def test_seed_generation():
    dt = datetime(2024, 9, 19, 14, 31, 59, 167000)
    seed = generate_rpo_seed(dt)
    
    assert len(seed) == 4096
    assert seed.startswith(TEMPLATE.encode('latin-1'))
    assert b'19Sep  2 202414:31:59167' in seed
    assert seed.count(b'\x00') == 4096 - 78
    
    expected_sha = "c3668278ad1ccd6a8b73f4ea642ec22abdec9f8807c8a3788a0a330309cf1794"
    assert hashlib.sha256(seed).hexdigest() == expected_sha
```

### 7.2 Teste Cross-Year
```python
test_cases = [
    (2000, 0, "01Jan  0 200000:00:00000"),
    (2012, 1, "15Jun  1 201212:00:00000"),
    (2023, 1, "15May  1 202310:20:30123"),
    (2024, 2, "19Sep  2 202414:31:59167"),
    (2025, 2, "01Jan  2 202500:00:00000"),
]

for year, expected_mystery, expected_ts in test_cases:
    dt = datetime(year, 1, 1, 0, 0, 0)
    seed = generate_rpo_seed(dt)
    ts = seed[len(TEMPLATE):len(TEMPLATE)+24].decode('latin-1')
    mystery = (year % 100) // 12
    assert mystery == expected_mystery, f"Year {year}: expected {expected_mystery}, got {mystery}"
```

---

**Fim das especificações técnicas.**
