# Relatório Final: Engenharia Reversa da Seed RPO

**Data:** 2026-10-11  
**Status:** 🟡 INCOMPLETO - Aguardando mais amostras  
**Seed Alvo:** 4096 bytes (1 amostra capturada)

---

## Executiva

Foi capturada **uma seed de 4096 bytes** durante compilação de RPO no container Protheus. A seed contém metadata legível indicando ser de um **ambiente de debug/teste**, com timestamp exato de **19 de Setembro de 2024, 14:31:59.167ms**.

A análise demonstrou que a seed **NÃO é um arquivo CTREE** (formato de banco de dados legítimo do Protheus), mas sim um buffer de seed OpenSSL composto por:
- ~100 bytes de metadata legível
- ~3996 bytes de padding nulo

O algoritmo exato de geração dos blocos de dados **não foi identificado** após testes com hashes simples (MD5, SHA1, SHA256, SHA512) e transformações básicas (XOR, ROT).

---

## Seed Capturada

```
Offset 0x0000 (primeiros 100 bytes):
Uauauauau!123132,m. 123k1p23i1=23-09=1239 12312=-3123 19Sep  2 202414:31:59167

Offset 0x0064-0x0FFF (resto):
Padding nulo (0x00)
```

**SHA-256 da seed:** `024a2879f3eeb29921ee10aef04c7f2c0e5c7dae47e8b05162b8f78276c92e06`

---

## Decomposição Estrutural

| Componente | Offset | Tamanho | Conteúdo |
|------------|--------|---------|----------|
| Prefixo | 0x0000 | 10 B | `"Uauauauau!"` (debug marker) |
| Bloco 1 | 0x000A | 9 B | `"123132,m."` |
| Bloco 2 | 0x0014 | 21 B | `"123k1p23i1=23-09=1239"` |
| Bloco 3 | 0x002A | 11 B | `"12312=-3123"` |
| Timestamp | 0x0036 | 21 B | `"19Sep  2 202414:31:59"` |
| Milissegundos | 0x004D | 3 B | `"167"` |
| Padding | 0x0050 | ~3996 B | `0x00` |

---

## Variáveis do Sistema Coletadas

### Identificadores Únicos
| Variável | Valor | Fonte |
|----------|-------|-------|
| Hostname | `laptop-peder` | `hostname` |
| Machine ID | `5072ea23cdeca727a449f44a67688097` | `/etc/machine-id` |
| Docker ID | `bb8461f4-4a62-4ae6-9539-0bfe0f5a3093` | `docker info` |

### Hardware
| Device | Serial | Tipo |
|--------|--------|------|
| `/dev/sda` | `ZDZJ0DR4` | HDD 2TB (ST2000LM007) |
| `/dev/nvme0n1` | `AA0000000000001374` | NVMe |

### Rede (MACs)
| Interface | MAC |
|-----------|-----|
| `wlp0s20f3` | `c4:23:60:b3:e8:d1` |
| `docker0` | `16:b4:64:69:a0:5b` |
| `br-f4c92a97e43e` | `e2:91:33:b8:63:43` |
| `br-8a776af16292` | `f2:fb:b7:1a:a5:7c` |

### Sistema
| Variável | Valor |
|----------|-------|
| Kernel | `7.1.5-76070105-generic` |
| Entropy disponível | 256 bits |
| Pool size | 256 bits |

---

## Análise do Algoritmo

### Testes Realizados

| Método | Resultado |
|--------|-----------|
| SHA-256(hostname) | ❌ Sem match |
| SHA-512(hostname+machine_id) | ❌ Sem match |
| HMAC-SHA256(variáveis) | ❌ Sem match |
| XOR com constantes | ❌ Sem match |
| ROT13/ROT47 | ❌ Sem match |
| Busca em binários | ❌ Padrões não encontrados |

### Observações

1. **Todos os blocos começam com "123"**:
   - Block1: `123132`
   - Block2: `123k1p...`
   - Block3: `12312...`
   
   Isso sugere um **contador ou ID sequencial**.

2. **Timestamp embutido**:
   - Formato: `%d%b  %y %Y%H:%M:%S%03d`
   - Exato: 19 Sep 2024, 14:31:59.167ms

3. **Build de debug**:
   - Prefixo `"Uauauauau!"` é claramente um marker de teste
   - Indica que a seed foi gerada em ambiente não-produção

---

## Veredito: NÃO é CTREE

| Critério | CTREE Esperado | Observado |
|----------|----------------|-----------|
| Magic signature | `DTB0`/`DTB1` | ❌ Ausente |
| Entropia header | ~7-8 bits/byte | 3.4 bits/byte |
| Estrutura | Records + tree pointers | Metadata + zeros |
| Assinaturas | Presentes | Ausentes |

**Conclusão:** A seed é um buffer OpenSSL RAND_seed(), não um arquivo de banco de dados.

---

## Próximos Passos para Encontrar o Algoritmo

### Imediatos
1. **Coletar mais amostras** - Rodar múltiplas compilações com o hook v17
2. **Analisar correlações** - Comparar seeds de diferentes momentos
3. **Identificar componentes variáveis** - O que muda vs o que é fixo

### Avançados
4. **Engenharia reversa do binário** - Usar Ghidra/IDA para encontrar função geradora
5. **Análise dinâmica** - Debugging do processo para rastrear geração de seed
6. **Comparação com builds** - Testar seeds de diferentes versões do Protheus

---

## Arquivos Gerados

| Arquivo | Descrição |
|---------|-----------|
| `/tmp/rpo_key_hook_v17.so` | Hook para captura múltipla de seeds |
| `/tmp/rpo_seeds_v16_full.json` | Seed capturada (4096 bytes) |
| `/tmp/analyze_seed_pattern.py` | Análise do padrão |
| `/tmp/seed_generator.py` | Tentativas de reprodução |
| `docs/rpo-seeds/SEED-ANALYSIS-FULL.md` | Análise estrutural |
| `docs/rpo-seeds/SEED-BRUTEFORCE-VARIABLES.md` | Variáveis coletadas |
| `docs/rpo-seeds/INVESTIGATION-FINAL-REPORT.md` | Este relatório |

---

## Notas Técnicas

### Hook de Captura
- **v14**: Captura apenas `RAND_bytes` (keys)
- **v15**: Captura `RAND_seed` + `RAND_bytes` (64 bytes truncados)
- **v16**: Captura `RAND_seed` completo (4096 bytes)
- **v17**: Captura múltiplas seeds (até 50)

### Limitações Encontradas
1. Compilações falham com "Session [P12] not found"
2. Necessita de appserver rodando corretamente
3. Ambiente de teste pode não ter todas as dependências

### Recomendações
1. Iniciar appserver antes de capturar seeds
2. Usar container com configuração completa
3. Coletar pelo menos 5-10 seeds para análise estatística

---

**Próxima ação:** Coletar mais amostras de seeds e analisar correlações.
