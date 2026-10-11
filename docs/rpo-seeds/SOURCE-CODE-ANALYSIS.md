# Análise do Código-Fonte Identificado

**Data:** 2026-10-11  
**Status:** 🟡 CÓDIGO NÃO DISPONÍVEL (interno TOTVS)

---

## 1. Informações do Build

### 1.1 Caminho do Build
```
/usr/local/bamboo/xml-data/build-dir/TP11-OF2431X-APPSRVLIN64/lib/security.cpp
```

### 1.2 Decodificação
| Componente | Valor | Significado |
|------------|-------|-------------|
| `/usr/local/bamboo/` | Path | Instalação do Atlassian Bamboo |
| `xml-data/build-dir/` | Path | Diretório de builds do Bamboo |
| `TP11` | Projeto | TOTVS Protheus 11 |
| `OF2431X` | Código | Identificador interno TOTVS |
| `APPSRVLIN64` | Target | AppServer Linux 64-bit |
| `lib/security.cpp` | Arquivo | Módulo de segurança |

### 1.3 Implicações
- Build realizado em servidor interno TOTVS
- Código-fonte é propriedade intelectual da TOTVS
- Não há acesso público ao repositório
- A análise foi feita exclusivamente por engenharia reversa do binário

---

## 2. Reconstrução do Código

Baseado na análise do binário e no template encontrado, reconstruímos o código provável:

### 2.1 Estrutura Provável

```cpp
// security.cpp (reconstruído)
#include <openssl/rand.h>
#include <openssl/aes.h>
#include <cstring>
#include <ctime>

// Template fixo no binário (offset 0x37D3AE8)
static const char* RPO Seed_TEMPLATE = 
    "Uauauauval!123132,m. 123k1p23i1=23-09=1239 12312=-3123 ";

// Tabela de nomes de meses abreviados
static const char* MONTHS_ABBR[] = {
    "Jan", "Feb", "Mar", "Apr", "May", "Jun",
    "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"
};

void GenerateRPOSeed(unsigned char* seed, int seedSize) {
    // Obter timestamp atual
    struct timeval tv;
    gettimeofday(&tv, NULL);
    
    struct tm* tm_info = localtime(&tv.tv_sec);
    
    // Calcular valor misterioso
    int year = tm_info->tm_year + 1900;
    int mystery = (year % 100) / 12;
    
    // Formatar timestamp
    char timestamp[32];
    snprintf(timestamp, sizeof(timestamp),
             "%02d%3s  %d %04d%02d:%02d:%02d%03d",
             tm_info->tm_mday,
             MONTHS_ABBR[tm_info->tm_mon],
             mystery,
             year,
             tm_info->tm_hour,
             tm_info->tm_min,
             tm_info->tm_sec,
             (int)(tv.tv_usec / 1000));
    
    // Montar seed
    strncpy((char*)seed, RPO Seed_TEMPLATE, seedSize);
    strncat((char*)seed, timestamp, seedSize - strlen((char*)seed) - 1);
    
    // Preencher com zeros
    memset(seed + strlen((char*)seed), 0, seedSize - strlen((char*)seed));
}

void InitializeCrypto() {
    unsigned char seed[4096];
    unsigned char key[16];
    unsigned char iv[16];
    AES_KEY aes_key;
    
    // Gerar seed
    GenerateRPOSeed(seed, sizeof(seed));
    
    // Sementar RNG
    RAND_seed(seed, sizeof(seed));
    
    // Gerar chaves
    RAND_bytes(key, 16);
    RAND_bytes(iv, 16);
    
    // Configurar AES
    AES_set_encrypt_key(key, 128, &aes_key);
    
    // Armazenar para uso posterior
    gAESKey = aes_key;
    gAESIV = iv;
}
```

### 2.2 Funções Relacionadas

```cpp
// Funções criptográficas encontradas no binário

// Offset 0x4A8A4C
void AES_set_encrypt_key(const unsigned char *userKey, int bits, AES_KEY *key);

// Offset 0x4A8C75  
void AES_set_decrypt_key(const unsigned char *userKey, int bits, AES_KEY *key);

// Offset 0x4A8C95
void AES_cbc_encrypt(const unsigned char *in, unsigned char *out, 
                     long length, const AES_KEY *key, 
                     unsigned char *ivec, int enc);

// Offset 0x4A8A60
void AES_encrypt(const unsigned char *in, unsigned char *out, const AES_KEY *key);

// Offset 0x4A8C89
void AES_decrypt(const unsigned char *in, unsigned char *out, const AES_KEY *key);

// Offset 0x3D9BDD
int RAND_seed(const void *buf, int num);

// Offset 0x3D9BE7
int RAND_bytes(unsigned char *buf, int num);
```

---

## 3. Análise de Segurança

### 3.1 Vulnerabilidades Identificadas

| # | Vulnerabilidade | Severidade | Descrição |
|---|-----------------|------------|-----------|
| 1 | Seed determinística | CRÍTICA | Seed previsível baseada em timestamp |
| 2 | Template fixo | ALTA | Template hardcoded no binário |
| 3 | Sem hardware binding | MÉDIA | Seed não usa IDs de hardware |
| 4 | Valor misterioso previsível | MÉDIA | Fórmula matemática simples |(year % 100) / 12` |

### 3.2 Impacto
- **Confidencialidade:** Comprometida (seed recuperável)
- **Integridade:** Comprometida (chave recuperável)
- **Disponibilidade:** Não afetada

### 3.3 Recomendações
1. Usar hardware IDs (CPU, MAC, disk serial) na geração da seed
2. Implementar rotation de chaves mais frequente
3. Usar CSRNG (Cryptographically Secure RNG) adequado
4. Esconder o template em código ofuscado

---

## 4. Métodos de Análise Utilizados

### 4.1 Engenharia Reversa Estática
```bash
# Strings extraction
strings -n 4 libaplinux.so | grep -E "Uau|123|Sep"

# Hex dump analysis
xxd -s 0x37D3AE8 -l 128 libaplinux.so

# Symbol lookup
nm libaplinux.so | grep -E "RAND|AES"
```

### 4.2 Engenharia Reversa Dinâmica
```bash
# LD_PRELOAD hook
LD_PRELOAD=./hook.so appsrvlinux

# Memory inspection
gdb -p $(pgrep appsrvlinux)
```

### 4.3 Análise de Padrões
```python
# Busca por padrões no binário
patterns = ["Uauauau", "123132", "123k1p", "23-09"]
for pat in patterns:
    offsets = grep_binary(binary, pat)
```

---

## 5. Limitações da Análise

| Limitação | Impacto | Mitigação |
|-----------|---------|-----------|
| Código-fonte inacessível | Não posso verificar implementação exata | Inferência baseada no binário |
| Apenas uma seed conhecida | Não posso validar cross-year | Hipótese formulada e testada |
| Build de debug | Seed pode ser diferente em release | Analisar build de produção |
| Sem acesso a variáveis de runtime | Não posso ver o processo exato | Hook LD_PRELOAD foi usado |

---

## 6. Próximos Passos para Validação

1. **Coletar mais seeds** de diferentes datas/anos
2. **Analisar build de produção** (não debug)
3. **Obter acesso ao repositório** security.cpp internamente
4. **Implementar testes** contra RPOs reais
5. **Validar a fórmula** do valor misterioso com mais amostras

---

**Nota:** Esta análise é baseada exclusivamente em engenharia reversa do binário. O código-fonte original permanece proprietÁRIO da TOTVS.
