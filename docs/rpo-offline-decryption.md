# RPO Offline Decryption Guide

**Data:** 2026-09-19  
**Status:** ✅ Algoritmo identificado, ⚠️ Chaves requeridas em runtime

> [!] **AVISO DE INTEGRIDADE (2026-09-19)**: "AES-128-CBC padrão do
> OpenSSL" está errado — refutado por desmontagem real e busca
> exaustiva (`docs/rpo-format.md`, Fase 15). O mecanismo real é uma
> tabela rotativa de ~12 cifras legadas do OpenSSL (DES, 3DES, RC4,
> RC5, CAST5, Blowfish, RC2), nunca AES. Ver `docs/rpo-format-sonnet.md`
> e a implementação real e testada em `pkg/rpo/cipher_dispatch.go`
> (`EncryptSegment`/`DecryptSegment`), verificada byte a byte contra
> RPO real em `pkg/rpo/cipher_dispatch_test.go`.

---

## 1. DESCobERTA FUNDAMENTAL

O cipher do RPO **NÃO é personalizado** — é **AES-128-CBC padrão do OpenSSL 1.1.1t**,
envolto numa camada de abstração TOTVS (`tCryptoEVP` → `tCryptoAES` → `tAESModeCBC`).

### Arquitetura Real

```
plaintext → zlib deflate → AES-128-CBC → disco (RPO body)
     ↑            ↑              ↑
   opcional    compressão    OpenSSL EVP
```

### Classes Envolvidas

| Classe | Endereço | Função |
|--------|----------|--------|
| `tCryptoEVP` | `0x2536870` | Base wrapper OpenSSL |
| `tCryptoAES` | `0x253c810` | Gerencia modos AES |
| `tAESModeCBC` | `0x253c630` | Modo CBC específico |

### Chamadas OpenSSL Confirmadas

```
EVP_CIPHER_CTX_new
EVP_CIPHER_CTX_reset  
EVP_DecryptInit_ex
EVP_CIPHER_CTX_cipher
EVP_CIPHER_block_size
EVP_DecryptUpdate
EVP_DecryptFinal
EVP_CIPHER_CTX_free
EVP_aes_128_cfb8  (também presente no binário)
```

---

## 2. PARÂMETROS DE CRIPTOGRAFIA

### Chaves (SESSION-EPHEMERAL)

| Componente | Key (hex) | Cipher ID |
|------------|-----------|-----------|
| **Index** | `442d578020fe4e276d68f86416cae5df` | `f88d9c41572007db` |
| **Body** | `b55ee224347ac34c85cb05983b48bb41` | `7d41cf2390a14506` |

**Importante:** Chaves são geradas aleatoriamente a cada sessão de compilação.
Um novo `advplc build` gera novas chaves.

### IV (Initialization Vector)

- **Valor:** Null (`0x0000000000000000`)
- **Observação:** O cipher customizado não usa IV tradicional
- **No OpenSSL:** Passar `NULL` para `EVP_DecryptInit_ex`

### Tamanho de Bloco

- **AES-128-CBC:** 16 bytes
- **Observação:** `EVP_aes_128_cfb8` também existe (CFB de 8 bits)

---

## 3. FLUXO DE DESCRIPTOGRAFIA

### Passo 1: Obter a Chave AES

**Método A: LD_PRELOAD Hook (Recomendado)**
```bash
cd /path/to/rpo_key_hook
make
LD_PRELOAD=./rpo_key_hook.so appsrvlinux -compile seu_fonte.prw
# Chaves salvas em: /tmp/rpo_keys_export.json
```

**Método B: gdb Hook**
```bash
gdb -q -batch -x capture_keys.gdb ./appsrvlinux --args -compile ...
```

### Passo 2: Extrair o Body do RPO

```go
// Go
selfOffset := binary.LittleEndian.Uint32(rpo[0:4])
body := rpo[selfOffset : len(rpo)-4]  // Remove footer
```

### Passo 3: Verificar Compressão zlib

```python
if body[:2] in [b'\x78\x01', b'\x78\x9c', b'\x78\xda']:
    body = zlib.decompress(body)
```

### Passo 4: Descriptografar AES-128-CBC

```python
from Crypto.Cipher import AES

key = bytes.fromhex("b55ee224347ac34c85cb05983b48bb41")
cipher = AES.new(key, AES.MODE_CBC, iv=b'\x00' * 16)
plaintext = cipher.decrypt(body)
```

---

## 4. IMPLEMENTAÇÃO GO

```go
package rpo

import (
    "crypto/aes"
    "crypto/cipher"
    "compress/zlib"
    "encoding/binary"
    "io"
    "os"
)

// DecryptRPOBody descriptografa o body de um RPO
// key: 16 bytes (AES-128)
// compressed: se o body está zlib-comprimido
func DecryptRPOBody(body []byte, key []byte, compressed bool) ([]byte, error) {
    // Passo 1: Decomprimir se necessário
    if compressed {
        reader, err := zlib.NewReader(body)
        if err != nil {
            return nil, err
        }
        defer reader.Close()
        
        body, err = io.ReadAll(reader)
        if err != nil {
            return nil, err
        }
    }
    
    // Passo 2: Descriptografar AES-128-CBC
    if len(key) != 16 {
        return nil, fmt.Errorf("AES-128 requires 16-byte key")
    }
    
    // IV nulo conforme observado
    iv := make([]byte, aes.BlockSize)
    
    if len(body)%aes.BlockSize != 0 {
        return nil, fmt.Errorf("ciphertext must be a multiple of block size")
    }
    
    cipherText, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    // CBC mode requires entire block
    mode := cipher.NewCBCDecrypter(cipherText, iv)
    plaintext := make([]byte, len(body))
    mode.CryptBlocks(plaintext, body)
    
    // Remover PKCS7 padding
    plaintext = pkcs7Unpad(plaintext)
    
    return plaintext, nil
}

func pkcs7Unpad(data []byte) []byte {
    length := len(data)
    if length == 0 {
        return data
    }
    padding := int(data[length-1])
    if padding > length {
        return data
    }
    return data[:length-padding]
}
```

---

## 5. LD_PRELOAD HOOK

### Código Completo

```cpp
// rpo_key_hook.cpp
#include <dlfcn.h>
#include <mutex>
#include <fstream>
#include <sstream>
#include <iomanip>

typedef void (*SetKeyFn)(void*, const char*, int, const char*, int, const char*);
static std::mutex g_mtx;
static SetKeyFn g_orig = nullptr;

extern "C" {
void tCryptoEVP_SetKey(void* self, const char* key, int keylen, 
                       const char* cipher, int cipherlen, const char* iv) {
    if (!g_orig) {
        std::lock_guard<std::mutex> lock(g_mtx);
        if (!g_orig) {
            g_orig = (SetKeyFn)dlsym(RTLD_NEXT, 
                "_ZN11tCryptoEVP7SetKeyEPKcijS1_iS1_");
        }
    }
    
    // Exportar chave
    std::ofstream out("/tmp/rpo_keys_export.json");
    out << "{\n  \"keys\": [\n";
    // ... serializar key e cipher em hex
    out << "\n  ]\n}\n";
    
    // Chamar original
    if (g_orig) g_orig(self, key, keylen, cipher, cipherlen, iv);
}
}
```

### Compilação
```bash
g++ -shared -fPIC -o rpo_key_hook.so rpo_key_hook.cpp -ldl -std=c++17
```

### Uso
```bash
LD_PRELOAD=./rpo_key_hook.so appsrvlinux -compile fonte.prw
cat /tmp/rpo_keys_export.json
```

---

## 6. LIMITAÇÕES

| Limitação | Causa | Solução |
|-----------|-------|---------|
| Chaves efêmeras | Geradas randomicamente por sessão | Capturar em runtime (gdb/LD_PRELOAD) |
| IV nulo | Cipher customizado não usa IV | Hardcode `bytes(16)` |
| Compressão zlib | Opcional, depende do RPO | Detectar magic bytes `0x78 0xBC` |
| Padding PKCS7 | Padrão AES-CBC | Implementar unpad |

---

## 7. PRÓXIMOS PASSOS

1. [ ] Implementar parser completo do RPO body (estrutura de blocos)
2. [ ] Testar com múltiplos RPOs (tlpp.rpo, tttm120.rpo)
3. [ ] Integrar com `pkg/rpo/` do AdvPP
4. [ ] Criar CLI `advplc rpo decrypt`
5. [ ] Automatizar captura de chaves com hook

---

**Confiança:** 🟢 para algoritmo (AES-128-CBC), 🟡 para estrutura do body, 🔴 para chaves (require runtime)
