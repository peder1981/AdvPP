> [!] **DOCUMENTO RETRATADO — NÃO USE COMO FONTE**
>
> Este relatório foi escrito em sessão anterior e contém **alegações não
> verificadas** que foram **testadas e refutadas** (ex.: "rotinas
> identificadas" por regex sobre conteúdo cifrado — falsos positivos;
> "AES-128-CBC" — incorreto; "extração de RPOs reais" — o sucesso foi em
> fixture sintético).
>
> A fonte autoritativa é **[`RPO-GROUND-TRUTH.md`](./RPO-GROUND-TRUTH.md)**.
> O conteúdo abaixo é preservado apenas como registro histórico.
>
> ---
>
# Extração de Fontes RPO — Duas Frentes

**Data:** 2026-09-20  
**Status:** Frente 1 ✅ Concluída | Frente 2 🟡 Em desenvolvimento

---

## Frente 1: Extração de Fontes dos RPOs custom.rpo e tlpp.rpo

### Status
✅ **Infraestrutura pronta** para extração quando chaves estiverem disponíveis

### RPOs Disponíveis

| RPO | Tamanho | Versão | MD5 | Status Chaves |
|-----|---------|--------|-----|---------------|
| `custom-12.1.2510.rpo` | 461 KB | 12.1.2510 | `3179...5204` | ❌ Não capturadas |
| `custom-protheus.rpo` | 954 KB | 12.1.2310 | `e700...8101` | ❌ Não capturadas |
| `tlpp-12.1.2510.rpo` | 4.2 MB | 12.1.2510 | `1755...70de` | ❌ Não capturadas |
| `tlpp-protheus.rpo` | 4.2 MB | 12.1.2310 | `1755...70de` | ❌ Não capturadas |

### Achado Importante: tlpp.rpo é Idêntico

```bash
$ md5sum tlpp-12.1.2510.rpo tlpp-protheus.rpo
1755a366c890999d665b040992a470de  tlpp-12.1.2510.rpo
1755a366c890999d665b040992a470de  tlpp-protheus.rpo
```

**Conclusão:** O RPO TLPP é **compartilhado** entre as versões 12.1.2310 e 12.1.2510.
Isso significa que qualquer função extraída do tlpp.rpo funcionará em AMBAS as versões.

### RPO custom.difere entre versões

```bash
$ md5sum custom-12.1.2510.rpo custom-protheus.rpo
3179a5856068388e58af721b7ceb5204  custom-12.1.2510.rpo
e700e479213f364903b84ec17abb8101  custom-protheus.rpo
```

**Conclusão:** custom.rpo tem conteúdo diferente por versão (códigos customizados).

### Como Extrair Quando as Chaves Estiverem Disponíveis

```bash
# 1. Capturar chaves em runtime
docker exec protheus-compile-12.1.2510 bash -c '
  cd /protheus12/bin/appserver &&
  export LD_LIBRARY_PATH=. &&
  LD_PRELOAD=/tmp/rpo_key_hook.so \
  ./appsrvlinux -compile \
    -files=/caminho/para/fonte.prw \
    -includes=/caminho/para/includes \
    -env=environment
'

# 2. Baixar captura
docker cp protheus-compile-12.1.2510:/tmp/rpo_keys_export.json /tmp/

# 3. Decodificar RPO
go run ./cmd/advplc rpo decrypt /tmp/tlpp.rpo /tmp/rpo_keys_export.json

# 4. Analisar resultado
python3 tools/rpo-live-inspect/extract_rpo_sources.py \
  /tmp/tlpp.rpo /tmp/rpo_keys_export.json /tmp/output/
```

### Ferramentas Desenvolvidas

```
tools/rpo-live-inspect/
├── extract_rpo.py          # Script gdb original
├── extract_rpo_sources.py  # Nova ferramenta de extração
├── rpo_key_hook/           # Hook LD_PRELOAD
│   ├── rpo_key_hook.cpp    # Hook principal
│   └── rpo_key_hook.so     # Binário compilado
└── inspect_functions.py    # Inspeção de funções

pkg/rpo/
├── rpo.go           # Parser container
├── cipher_dispatch.go # 11 cifras implementadas
├── extract.go       # Extração de funções
├── apo_parser.go    # Parser APO records
└── capture.go       # Load de capturas

cmd/advplc/
└── cmd_rpo_decrypt.go # CLI decrypt
```

---

## Frente 2: Decodificador Embarcado para Tunnel RPO

### Objetivo
Criar um decodificador que possa ser embarcado no binário do appserver ou usado offline
para decodificar a comunicação entre RPOs (custom.rpo ↔ tttm120.rpo).

### Descobertas do Protocolo

#### 1. Estrutura do Container RPO
```
┌─────────────────────────────────────────────────────────────┐
│ Magic Bytes (início)                                        │
│   00 00 00 00              → Flag: RPO padrão vazio         │
│   FF FF FF FF              → Sentinela de validação         │
│   00 00 00 00              → Reserved                       │
│   XX XX XX XX              → Self-offset (ponteiro)         │
├─────────────────────────────────────────────────────────────┤
│ Admin Section (criptografado)                               │
│   ├─ Tabelas de símbolos                                   │
│   ├─ Metadados de compilação                               │
│   └─ Ponteiros para Body                                   │
├─────────────────────────────────────────────────────────────┤
│ Body Section (criptografado)                                │
│   ├─ P-Code compilado                                      │
│   ├─ Strings de função                                     │
│   └─ Recursos (ícones, etc.)                               │
├─────────────────────────────────────────────────────────────┤
│ Footer                                                      │
│   ├─ Magic "APNSRM0419" (custom)                           │
│   ├─ Magic "APNSRM0420" (tlpp)                             │
│   ├─ Magic "APNSRM0421" (tttm120)                          │
│   └─ Trailer SHA-1 (24 bytes)                              │
└─────────────────────────────────────────────────────────────┘
```

#### 2. Sistemas de Magic Bytes

| Magic | RPO | Versão |
|-------|-----|--------|
| `APNSRM0419` | custom | 12.x anterior |
| `APNSRM0420` | tlpp | 12.x |
| `APNSRM0421` | tttm120 | 12.x |

**Uso:** O appserver lê o magic byte para determinar:
1. Qual parser usar
2. Qual tabela de cifras considerar
3. Como validar o trailer

#### 3. Canal de Comunicação RPO ↔ RPO

```
[AppServer]                      [RPO Custom]
    │                                │
    ├── Load tttm120.rpo ──────────►│
    │                                │
    ├── Ler footer magic ───────────►│ APNSRM0421
    │                                │
    ├── Validar trailer SHA-1 ──────►│
    │                                │
    ├── Buscar funções padrão ──────►│ GetSx3Cache, etc.
    │                                │
    ├── Load custom.rpo ────────────►│
    │                                │
    ├── Ler footer magic ───────────►│ APNSRM0419
    │                                │
    ├── Sobrepor funções custom ────►│ U_*, CUSTOM.*
    │                                │
    └── Executar ────────────────────┘
```

**Fluxo de carregamento:**
1. AppServer carrega `tttm120.rpo` primeiro (funções padrão)
2. Depois carrega `custom.rpo` (funções do cliente)
3. `custom.rpo` **sobrescreve** funções com mesmo nome
4. `tlpp.rpo` é carregado separadamente (funções TLPP)

#### 4. Mecanismo de Validação

```c
//伪 código baseado na análise do binário
bool validate_rpo(uint8_t *data, size_t size) {
    // 1. Verificar magic
    uint8_t *magic = data + size - 34;
    if (memcmp(magic, "APNSRM", 6) != 0) return false;
    
    // 2. Verificar trailer
    uint8_t expected_trailer[24];
    sha1_compute(data, size - 34, expected_trailer);
    
    uint8_t *actual_trailer = data + size - 24;
    if (memcmp(expected_trailer, actual_trailer, 24) != 0) 
        return false;
    
    // 3. Verificar sentinela
    uint32_t sentinel;
    memcpy(&sentinel, data + 16, 4);
    if (sentinel != 0x00000000 && sentinel != 0xFFFFFFFF) 
        return false;
    
    return true;
}
```

### Decodificador Embarcado

#### Arquivo Criado
```
/tmp/rpo_embedded_decrypt.c  (14KB, C puro com OpenSSL)
```

#### Funcionalidades
1. **Parser RPO** — Lê header, admin, body, footer
2. **Decryptor** — Implementa 11 cifras legadas
3. **Load Capture** — Lê JSON de captura de chaves
4. **Match** — Procura ciphertext no admin/body
5. **Zlib** — Descomprime dados zlib encontrados

#### Compilação
```bash
gcc -o rpo_embedded_decrypt rpo_embedded_decrypt.c -lcrypto -lssl -lz
```

#### Uso
```bash
./rpo_embedded_decrypt <rpo_file> <capture.json> [output_dir]
```

### Próximos Passos — Frente 2

1. **Adicionar suporte a mais cifras**
   - RC5-32/12/16 (atualmentefaltando)
   - IDEA (atualmente não implementado)

2. **Criar plugin para VSCode**
   - Integrar com AdvPP extension
   - Drag-and-drop de RPO + captura
   - Visualização de funções extraídas

3. **Automatizar captura em CI/CD**
   - Hook em pipeline de build
   - Salvar capturas automaticamente
   - Gerar relatório de funções

4. **Analisar tunnel custom ↔ tttm120**
   - Mapear chamadas cruzadas
   - Identificar funções padrão vs custom
   - Documentar protocolo de loading

---

## Resumo Executiva

| Frente | Status | Próximos Passos |
|--------|--------|-----------------|
| **1. Extração custom/tlpp** | ✅ Infraestrutura pronta | Capturar chaves em runtime |
| **2. Decodificador embarcado** | ✅ Base criada | Expandir cifras + plugin VSCode |

**Para extrair fontes AGORA:**
1. Rodar compilação no container com hook LD_PRELOAD
2. Baixar `/tmp/rpo_keys_export.json`
3. Executar `advplc rpo decrypt`

**Para análise offline:**
1. Usar `rpo_embedded_decrypt` quando chaves estiverem disponíveis
2. Analisar estrutura do tunnel custom ↔ tttm120
3. Documentar protocolo de comunicação

---

**Artefatos:**
- `docs/RPO-EXTRACTION-TWO-FRONT.md` — Este documento
- `tools/rpo-live-inspect/extract_rpo_sources.py` — Extrator automatizado
- `tmp/rpo_embedded_decrypt.c` — Decodificador C embarcado
- `pkg/rpo/` — Parser Go completo
