# Abordagens Alternativas para Extração RPO

**Data:** 2026-09-21  
**Autor:** Agnes (Sapiens AI)

---

## Resumo

Quando não se pode entrar pela porta principal (descriptografia direta), 
tenta-se a janela (exploitar falhas de implementação) ou a porta dos fundos 
(abordagens indiretas).

---

## Abordagem 1: Binary Patching

### 1.1 Remover Check de Versão

**Localização:** Offset 0x37c7153 no libaplinux.so

**String identificada:**
```
"...Decrypt evpCrp is null\x0020.3.3.0\x00production\x00..."
```

**Patch proposto:**
```python
# patch_rpo_version.py
with open('libaplinux.so', 'rb') as f:
    data = bytearray(f.read())

# Encontrar e substituir
pos = data.find(b'20.3.3.0')
if pos >= 0:
    data[pos:pos+8] = b'24.3.1.1'
    with open('libaplinux.so.patched', 'wb') as f:
        f.write(data)
```

**Resultado:** Novo erro "load default map DEFAULT" aparece.

**Próximo passo:** Investigar causa do novo erro.

### 1.2 Exportar Chaves

Modificar o binário para salvar chaves em arquivo após uso:

```cpp
// Inserir no final de tCryptoEVP::SetKey
extern "C" void hooked_SetKey(...) {
    // ... código original ...
    
    // Salvar chaves
    FILE* f = fopen("/tmp/rpo_debug_keys.json", "a");
    fprintf(f, "{\"key\":\"%s\",\"iv\":\"%s\"}\n", key_hex, iv_hex);
    fclose(f);
}
```

**Problema:** Require recompilação ou injeção de código.

---

## Abordagem 2: Memory Scraping

### 2.1 Ler Memória do Processo

```bash
# Encontrar PID do appserver
PID=$(pgrep appsrvlinux)

# Ler/maps para ver segmentos de memória
cat /proc/$PID/maps | grep -E "(rpo|apo|libap)"

# Tentar ler memória diretamente
dd if=/proc/$PID/mem bs=1 skip=0x<address> count=1024 2>/dev/null | strings
```

### 2.2 Procurar por Padrões

```python
import re

# Padrões de chave AES (16 bytes = 32 hex chars)
key_pattern = re.compile(rb'[0-9a-f]{32}')

# Padrões de IV (8 bytes = 16 hex chars)
iv_pattern = re.compile(rb'[0-9a-f]{16}')

# Buscar no dump de memória
for match in key_pattern.finditer(memory_dump):
    print(f"Possible key: {match.group().decode()}")
```

**Problema:** Memória pode estar em segmentos protegidos ou já limpa.

---

## Abordagem 3: Instrumentação

### 3.1 LD_PRELOAD Avançado

Criar hook que intercepta em nível mais baixo:

```cpp
// Hook para RAND_bytes
extern "C" int RAND_bytes(unsigned char* buf, int num) {
    // Chamar original
    int ret = orig_rand_bytes(buf, num);
    
    // Salvar se for 16 bytes (chave AES)
    if (num == 16 && ret == 1) {
        log_to_file("KEY", buf, 16);
    }
    
    return ret;
}
```

**Desafio:** RAND_bytes pode ser inline ou em região não hookable.

### 3.2 eBPF Tracing

Usar eBPF para tracear chamadas de função sem modificar binário:

```bash
# Trace EVP_EncryptInit_ex
sudo bpftrace -e '
    proto E{
        printf("EVP_EncryptInit_ex called\n")
        printf("cipher: %s\n", str(arg2))
        printf("key: %s\n", binstr(arg3, 16))
        printf("iv: %s\n", binstr(arg4, 8))
    }
' -c './appsrvlinux -compile ...'
```

**Requisito:** Kernel 4.9+ com eBPF habilitado.

---

## Abordagem 4: Análise Estática

### 4.1 Extrair Strings do RPO

```python
import re

with open('tttm120.rpo', 'rb') as f:
    data = f.read()

# Procurar por strings ASCII
strings = re.findall(rb'[\x20-\x7e]{6,}', data)

# Filtrar padrões de funções AdvPL
func_pattern = re.compile(rb'^[A-Z][A-Z0-9_]{3,10}$')
for s in strings:
    if func_pattern.match(s):
        print(s.decode())
```

### 4.2 Analisar Padrões de Bytes

```python
from collections import Counter

# Dividir em chunks de 4 bytes
chunks = [data[i:i+4] for i in range(0, len(data), 4)]
counter = Counter(chunks)

# chunks mais frequentes podem ser estruturas APO
for chunk, count in counter.most_common(10):
    if chunk != b'\x00\x00\x00\x00':
        print(f"{chunk.hex()}: {count}")
```

### 4.3 Mapear Estruturas APO

Registros APO têm estrutura conhecida:
- 4 bytes: tamanho
- 1 byte: tipo (0x01=function, 0x02=class, etc)
- N bytes: dados

Procurar por padrões de tamanho+tipo:

```python
# Procurar por headers APO
for i in range(len(data) - 5):
    size = struct.unpack('<I', data[i:i+4])[0]
    type_byte = data[i+4]
    
    # Tamanho razoável (100-1000000) e tipo válido
    if 100 < size < 1000000 and type_byte in [0x01, 0x02, 0x03]:
        print(f"Possible APO at offset {i}: size={size}, type={type_byte}")
```

---

## Abordagem 5: Reimplementação

### 5.1 GetSx3Cache

Baseado em padrões observados:

```advpl
/*/{Protheus.doc} GetSx3Cache
    Busca propriedade de campo no dicionário SX3 com cache interno
    @type Function
    @author Peder Munksgaard
    @since 21/09/2026
/*/
User Function GetSx3Cache(cCampo, cProperty)
    Local cAlias := "SX3"
    Local cValor := ""
    Local lOk    := .F.
    
    // Verificar se alias está aberto
    If FwAliasInDic(cAlias)
        Select(cAlias)
        
        // Preparar chave de busca
        Local cChave := xFilial(cAlias) + AllTrim(cCampo)
        
        // Posicionar no registro
        DbSeek(cChave)
        
        // Verificar se encontrou
        If !Eof() And !Bofer()
            // Obter posição do campo
            Local nPos := FieldPos(cProperty)
            If nPos > 0
                cValor := FieldGet(nPos, cAlias)
                lOk := .T.
            EndIf
        EndIf
    EndIf
    
Return IIF(lOk, cValor, "")
```

**Nota:** Esta é uma reimplementação aproximada baseada em padrões.

### 5.2 Funções SX3 Comuns

```advpl
// Funções típicas em SX3
User Function GetSx3Field(cTable, cField)
    // Retorna nome físico do campo
    ...
EndFunction

User Function GetSx3Index(cTable, nOrder)
    // Retorna informações do índice
    ...
EndFunction

User Function GetSx3Trigger(cTable, cField)
    // Retorna trigger do campo
    ...
EndFunction
```

---

## Abordagem 6: Hook tCryptoEVP::Encrypt

### 6.1 Assinatura Confirmada

```cpp
// Nome mangled: _ZN10tCryptoEVP7EncryptEiiiPciR9tAutoCharRi
// Decoded: tCryptoEVP::Encrypt(int, int, int, char*, int, tAutoChar&, int&)
typedef void* (*EncryptFn)(void*, int, int, int, char*, int, void*, int&);
```

### 6.2 Hook Implementation

```cpp
extern "C" void* tCryptoEVP_Encrypt(
    void* self, int a, int b, int c, 
    char* out, int outl, void* src, int& src_len)
    asm("_ZN10tCryptoEVP7EncryptEiiiPciR9tAutoCharRi");

void* tCryptoEVP_Encrypt(
    void* self, int a, int b, int c, 
    char* out, int outl, void* src, int& src_len) {
    
    // Log antes
    fprintf(stderr, "[CRYPTO] Encrypt: a=%d b=%d c=%d outl=%d src_len=%d\n",
            a, b, c, outl, src_len);
    
    // Capturar source
    if (src && src_len > 0) {
        hexdump("SRC", (unsigned char*)src, std::min(src_len, 64));
    }
    
    // Chamar original
    auto orig = (EncryptFn)dlsym(RTLD_NEXT, 
        "_ZN10tCryptoEVP7EncryptEiiiPciR9tAutoCharRi");
    void* ret = orig(self, a, b, c, out, outl, src, src_len);
    
    // Capturar output
    if (out && outl > 0) {
        hexdump("OUT", (unsigned char*)out, std::min(outl, 64));
        fprintf(stderr, "[CRYPTO] src_len after: %d\n", src_len);
    }
    
    return ret;
}
```

### 6.3 Calling Convention

Baseado na desmontagem:

```assembly
; Parâmetros (SysV AMD64 ABI):
; rdi = this
; rsi = a (int)
; rdx = b (int)
; rcx = c (int)
; r8  = out (char*)
; r9  = outl (int)
; [stack] = src (tAutoChar&)
; [stack+8] = src_len (int&)
```

---

## Abordagem 7: GDB Integration

### 7.1 Breakpoints Automáticos

```gdb
# script.gdb
break *_ZN10tCryptoEVP7EncryptEiiiPciR9tAutoCharRi
commands
  printf "Encrypt called\n"
  printf "  self=0x%x\n", $rdi
  printf "  a=%d b=%d c=%d\n", $rsi, $rdx, $rcx
  printf "  out=%p outl=%d\n", $r8, $r9
  continue
end

break *_ZN10tCryptoEVP6SetKeyEPKciS1_iS1_
commands
  printf "SetKey called\n"
  print (char*)$rsi
  continue
end

run --args ./appsrvlinux -compile -files=test.prw -env=environment
```

### 7.2 Dump de Memória

```gdb
# Dump registers e stack
info registers
x/32wx $rsp
```

---

## Abordagem 8: Strace/Ltrace

### 8.1 System Calls

```bash
strace -f -e trace=write ./appsrvlinux -compile ... 2>&1 | grep -E "(rpo|key|encrypt)"
```

### 8.2 Library Calls

```bash
ltrace -f ./appsrvlinux -compile ... 2>&1 | grep -E "(RAND|Encrypt|SetKey)"
```

---

## Conclusão

Nenhuma das abordagens alternativas é trivial, mas todas são viáveis:

1. **Binary patching** - Mais direto, mas require modify binário
2. **Memory scraping** - Require processo rodando
3. **Instrumentation** - Require eBPF ou recompilação
4. **Static analysis** - Limitado sem descriptografia
5. **Reimplementation** - Mais seguro, mas pode não ser exato
6. **Hook wrapper** - Promissor, require engenharia reversa da calling convention
7. **GDB** - Require processo interrompido
8. **Strace/Ltrace** - Limitado em performances

**Recomendação:** Combinar Abordagem 6 (hook wrapper) com Abordagem 5 (reimplementação) para obter resultados práticos.

---

*Documento gerado por Agnes (Sapiens AI) - 2026-09-21*
