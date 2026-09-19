# Investigação do Protocolo `advpls` (ADVPL Language Server)

**Data:** 2026-09-19  
**Investigador:** Agnes (Sapiens AI)  
**Contexto:** TOTVS TDS VSCode Extension + AdvPP RPO extraction

---

## 1. Arquitetura Encontrada

A extensão `killerall.advpl-vscode` v0.17.0 (TOTVS Official) possui **dois subsistemas distintos**:

### 1.1 Language Server (NÃO FUNCIONANDO)
- Função `initLanguageServer()` existe em `src/extension.ts:254` mas está **comentada** na linha 146
- Caminho do binário hardcoded para Windows: `C:\Totvs\vscode\advpl-language-server\bin\Debug\advpl-language-server.exe`
- Comunicação: **stdio** (spawn de processo, leitura stdout/stderr)
- Implementação original declarada como C#, mas o repositório público é TypeScript

### 1.2 AdvplDebugBridgeC (FUNCIONANDO — via monitor commands)
- Binário Linux: `bin/alpha/linux/AdvplDebugBridgeC` (ELF 64-bit, C++)
- Tecnologias: Boost.Asio + Boost.Beast + OpenSSL + nlohmann/json + Boost.Serialization
- Protocolo: **SSL WebSocket** para o appserver
- Comandos: `--compileInfo`, `--threadsInfo`, `--getFunctions`, `--getMap`, `--urlBuildWSClient`

---

## 2. O Repositório `totvs/advpl-language-server`

**URL:** https://github.com/totvs/advpl-language-server  
**Stars:** 4 | **Forks:** 5 | **Issues:** 1 (aberto desde 2018)

### 2.1 Stack
- TypeScript + ANTLR4TS (parser) + vscode-languageserver
- Grammar: `src/parser/grammar/Advpl.g4` (21KB, grammar ANTLR completa para AdvPL)
- Symbol table: `src/parser/symboltable/` (classes para funções, classes, variáveis, escopos)

### 2.2 O que o servidor implementa (LSP padrão)
```typescript
// server.ts — apenas LSP básico
connection.onInitialize()     // capabilities: textDocumentSync + completionProvider
connection.onCompletion()     // retorna items fixos ("TypeScript", "JavaScript")
connection.onCompletionResolve()
connection.onDidChangeConfiguration()
// NÃO há: onDefinition, onReferences, onHover, documentSymbol, etc.
```

### 2.3 O que NÃO implementa
- **Nenhuma comunicação com appserver/RPO**
- **Nenhuma extração de código fonte**
- **Nenhum símbolo dinâmico** (completion é hardcoded)
- **Parser ANTLR não é usado** no server.ts (só existe no módulo `parser/`)

### 2.4 Issue #2 (aberto desde Jul 2018)
> "Retornar no LS a lista de símbolos definida num fonte."
> Link para spec LSP `textDocument/documentSymbol`

**Conclusão:** O repositório é um **skeleton/prova de conceito** nunca finalizado.

---

## 3. Protocolo Real: AdvplDebugBridgeC

### 3.1 Como a extensão se conecta

```javascript
// extension.js → advplMonitor.js
const debugPath = debugBrdige.getAdvplDebugBridge(); // bin/alpha/linux/AdvplDebugBridgeC

// Para obter funções do RPO:
_args.push("--compileInfo=" + this.EnvInfos);  // JSON com config do ambiente
_args.push("--getFunctions");                   // ou "--getMap"
child = child_process.spawn(this.debugPath, _args);
child.stdout.on("data", callback);
```

### 3.2 Formato do `--compileInfo`
JSON serializado da configuração VSCode `advpl`:
```json
{
  "selectedEnvironment": "meuAmbiente",
  "environments": [...],
  "server": "192.168.1.100",
  "port": "1201",
  "user": "Admin",
  "passwordCipher": "...",
  "serverVersion": "210324P",
  "rpoType": "TOP",
  "language": "PORTUGUESE",
  ...
}
```

### 3.3 Comandos suportados pelo Bridge

| Argumento | Função | Retorno |
|-----------|--------|---------|
| `--getMap` | Lista sources/resources do RPO | Texto bruto → arquivo `.log` |
| `--getFunctions` | Lista funções do RPO | Texto bruto → arquivo `.log` |
| `--threadsInfo` | Info de threads do appserver | Console output |
| `--urlBuildWSClient=<url>` | Gera client WS Protheus | Arquivo `.prw` |
| `--compileInfo=<json>` + compile | Compila fonte | JSON de status |
| `--ADVANCEDPR--` | Modo advanced (alpha) | — |

### 3.4 Classes C++ identificadas (demangling)
- `advtec::monitor::AdvtecRpoSourceInfo` — estrutura de info de source do RPO
- `advtec::monitor::AdvtecRpoSourceInfoItem` — item individual
- `advtec::lib_base::MessageDbgCheckauth` — autenticação
- `advtec::lib_base::doneSendSource` — envio de source
- `advtec::compile::AdvplCompileMultiThread` — compilação multi-thread
- `advtec::debug::AdvtecAsyncDebugThread` — debug assíncrono

---

## 4. Fluxo de Comunicação Real

```
┌──────────────┐     spawn()      ┌─────────────────────┐    SSL WS     ┌──────────────┐
│ VSCode Ext   │ ───────────────→ │ AdvplDebugBridgeC   │ ────────────→ │ AppServer   │
│ (Node.js)    │                  │ (C++ binary)        │               │ (ports 1201)│
└──────────────┘ ←─────────────── └─────────────────────┘ ←──────────── └──────────────┘
                  stdout data                  --getFunctions             RPO encrypted
                  (text log)                   --getMap
```

**O language server (`advpls`) é um componente SEPARADO e INATIVO.**

---

## 5. Formato das Mensagens

### 5.1 Bridge → VSCode (stdout)
- **Texto bruto** (não JSON, não LSP)
- Formato depende do comando:
  - `--getMap`: lista de fontes do RPO (texto plano)
  - `--getFunctions`: lista de funções (texto plano)
  - `--threadsInfo`: info de threads
- Escrito em arquivo `.log` temporário pelo extension

### 5.2 Bridge → AppServer (SSL WebSocket)
- Protocolo proprietário TOTVS (`advtec::lib_base`)
- Autenticação via `MessageDbgCheckauth`
- Mensagens serializadas com Boost.Serialization (text_oarchive/text_iarchive)
- Estruturas: `StateEnv`, `State`

---

## 6. Viabilidade de Client Alternativo

### 6.1 Usando AdvplDebugBridgeC (VIÁVEL)
```bash
# Exemplo: listar funções do RPO
./AdvplDebugBridgeC --compileInfo='{"selectedEnvironment":"P12","environments":[...],...}' --getFunctions
```

**Prós:**
- Binário já existe no container e no host
- Funciona via SSL WebSocket (mesmo mecanismo da extensão)
- Não precisa de GDB

**Contras:**
- Binário fechad (no source disponível)
- Formato de saída é texto bruto não documentado
- Sem API programática — precisa parsear stdout

### 6.2 Usando advpls Language Server (NÃO VIÁVEL)
- Código-fonte disponível mas **incompleto**
- Não se conecta ao appserver
- Completion hardcoded ("TypeScript", "JavaScript")
- Parser ANTLR existe mas não é integrado ao server

### 6.3 Abordagem GDB Hooks (ATUAL)
- Funciona mas requer appserver rodando + attachment
- Chaves efêmeras por sessão
- Extração direta da memória do RPO criptografado

---

## 7. Recomendação

### Para consulta de funções RPO (sem GDB):
**Usar o `AdvplDebugBridgeC` diretamente.** É o mecanismo que a extensão TOTVS usa de fato. Pode ser invocado via `subprocess` em Python:

```python
import subprocess, json

compile_info = json.dumps(vscode_advpl_config)
args = [
    "/caminho/para/AdvplDebugBridgeC",
    f"--compileInfo={compile_info}",
    "--getFunctions"
]
result = subprocess.run(args, capture_output=True, text=True)
functions = result.stdout  # parsear texto
```

### Para extração de código fonte completo:
**Manter a abordagem GDB hooks.** O bridge não expõe endpoint para get source code — apenas metadados (mapa/lista). A extração via `tAppMap::GetApoCount()` + `GetFuncName()` ainda é a via para obter o código fonte compilado no RPO.

### Para autocompletar/IntelliSense:
O parser ANTLR do `advpl-language-server` poderia ser **integrado** como base para um language server caseiro, mas precisaria ser expandido significativamente (documentSymbol, definition, hover, references) e conectado ao bridge para símbolos do RPO.

---

## 8. Arquivos de Referência

| Arquivo | Localização |
|---------|-------------|
| Extension source (TS) | `src/extension.ts` no repo GitHub |
| Compiled extension | `~/.vscode/extensions/killerall.advpl-vscode-0.17.0/out/src/extension.js` |
| Monitor (RPO commands) | `out/src/advplMonitor.js` |
| Debug Bridge util | `out/src/utils/debugBridge.js` |
| Bridge binary (Linux) | `bin/alpha/linux/AdvplDebugBridgeC` |
| Language server repo | https://github.com/totvs/advpl-language-server |
| Grammar ANTLR | `src/parser/grammar/Advpl.g4` |
| Server LSP skeleton | `src/server.ts` |

---

## 9. Resumo Executivo

| Aspecto | Linguage Server (advpls) | Debug Bridge (AdvplDebugBridgeC) |
|---------|-------------------------|----------------------------------|
| **Status** | ❌ Desativado/comentado | ✅ Ativo e funcional |
| **Comunicação** | stdio (process spawn) | SSL WebSocket → appserver |
| **Código aberto** | ✅ TypeScript/ANTLR | ❌ Binário fechad |
| **Consulta RPO** | ❌ Não faz | ✅ `--getFunctions`, `--getMap` |
| **Compilação** | ❌ Não faz | ✅ via `--compileInfo` |
| **Extração source** | ❌ Não faz | ❌ Apenas metadados |
| **Viabilidade como base** | Baixa (incompleto) | Alta (já funciona) |

**Veredito:** O `advpls` como conhecido (language server que consulta RPO) **não existe na prática**. A extensão TOTVS usa o `AdvplDebugBridgeC` para todas as interações com o appserver. Para um client alternativo, o caminho é invocar o bridge diretamente ou reimplementar o protocolo SSL WebSocket `advtec::lib_base`.
