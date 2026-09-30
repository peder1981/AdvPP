# Workflow Completo: Descriptografia Offline RPO

**Data:** 2026-09-19  
**Status:** ✅ Implementado (precisa chaves capturadas)

> [!] **AVISO DE INTEGRIDADE (2026-09-19)**: "AES-128-CBC padrão do
> OpenSSL" está errado — refutado por desmontagem real e busca
> exaustiva (`docs/rpo-format.md`, Fase 15). O mecanismo real é uma
> tabela rotativa de ~12 cifras legadas do OpenSSL (DES, 3DES, RC4,
> RC5, CAST5, Blowfish, RC2), nunca AES, escolhida por chamada dentro
> de `tCryptoEVP::Encrypt`. Ver `docs/rpo-format-sonnet.md` e a
> implementação real e testada em `pkg/rpo/cipher_dispatch.go`. Fluxo
> de captura ainda é válido em espírito (chave/IV só existem em memória
> durante a compilação), mas o passo de decodificação precisa
> identificar a cifra correta por segmento (`EVP_EncryptInit_ex`), não
> assumir AES-128-CBC fixo.
>
> A afirmação abaixo de que a "Index Section" é "RSA-4096 encrypted" com
> senha `"manezinho"` também está errada quanto à FUNÇÃO dessa chave: a
> senha/chave RSA são reais (confirmadas, ver `docs/rpo-format.md` Fase
> 14.1), mas busca exaustiva em todo byte-offset de `AdminSection`/`Body`
> não encontrou nenhum bloco RSA-PKCS1v1.5/OAEP válido sob ela (Fase
> 15) — essa chave não envelopa o conteúdo do RPO.

---

## Descoberta Principal

O cipher do RPO **NÃO é personalizado** — é **AES-128-CBC padrão do OpenSSL**,
envolto numa camada de abstração TOTVS.

```
Arquitetura real:
  plaintext → (opcional) zlib deflate → AES-128-CBC → disco
                                          ↑
                              tCryptoEVP → tCryptoAES → tAESModeCBC
                                          ↓
                                     OpenSSL EVP interface
```

## Como Usar

### 1. Capturar Chaves (Necessário para cada sessão)

**Método A: LD_PRELOAD Hook (Recomendado)**
```bash
# No container protheus-compile
cd /path/to/tools/rpo-live-inspect/rpo_key_hook
make

# Iniciar compilação com hook
LD_PRELOAD=./rpo_key_hook.so \
  /protheus12/bin/appserver/appsrvlinux \
  -compile /caminho/para/seu_fonte.prw

# Chaves salvas automaticamente
cat /tmp/rpo_keys_export.json
```

**Método B: gdb (alternativo)**
```bash
gdb -q -batch -x capture_keys.gdb \
  /protheus12/bin/appserver/appsrvlinux \
  -compile /caminho/para/seu_fonte.prw
```

### 2. Descriptografar RPO

```bash
# Usar CLI AdvPP
advplc rpo decrypt \
  build/custom.rpo \
  -k b55ee224347ac34c85cb05983b48bb41 \
  -o decrypted_body.bin

# Verificar output
xxd decrypted_body.bin | head
```

### 3. Parsear Registros APO

```bash
# Usar Python
python3 -c "
import sys
sys.path.insert(0, '/home/peder/Projetos/AdvPP')
from pkg.rpo import ParseApoRecords

with open('decrypted_body.bin', 'rb') as f:
    data = f.read()

records = ParseApoRecords(data)
for i, r in enumerate(records):
    print(f'Record {i}: type={r[\"type\"]}, len={r[\"len\"]}')
"
```

---

## Estrutura do RPO

```
┌─────────────────────────────────────────┐
│ Header (4 bytes)                        │
│  - self-offset (uint32 LE)             │
├─────────────────────────────────────────┤
│ Index Section (variável)                │
│  - RSA-4096 encrypted metadata          │
│  - Password: "manezinho"                │
├─────────────────────────────────────────┤
│ Body Section (variável)                 │
│  - AES-128-CBC encrypted                │
│  - IV = null (0x0000000000000000)      │
│  - Possible zlib compression            │
├─────────────────────────────────────────┤
│ Footer (4 bytes)                        │
│  - footer-offset (uint32 LE)           │
└─────────────────────────────────────────┘
```

## Testes

```bash
# Rodar testes unitários
go test ./pkg/rpo/ -v -count=1

# Testes específicos
go test ./pkg/rpo/ -v -run TestDecrypt
go test ./pkg/rpo/ -v -run TestDetect
go test ./pkg/rpo/ -v -run TestParse
```

## Limitações

| Item | Status | Solução |
|------|--------|---------|
| Chaves efêmeras | ⚠️ | Capturar em runtime com hook |
| IV nulo | ✅ | Hardcode `bytes(16)` |
| Compressão zlib | ⚠️ | Detectar magic bytes |
| Padding PKCS7 | ✅ | Implementado |

## Artefatos

| Arquivo | Descrição |
|---------|-----------|
| `pkg/rpo/decrypt.go` | Decryptor AES-128-CBC (191 lines) |
| `pkg/rpo/decrypt_test.go` | Testes unitários (8 testes) |
| `tools/rpo-live-inspect/rpo_key_hook/` | Hook LD_PRELOAD |
| `/tmp/rpo_keys_export.json` | Chaves capturadas (gerado) |
| `/tmp/rsa_decrypted_openssl.pem` | Chave RSA privada |

---

**Última atualização:** 2026-09-19  
**Próximo passo:** Integrar com pipeline de build para captura automática
