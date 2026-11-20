# Auditoria de segurança — 2026-11-20 (v4.7.0)

Escopo: superfícies de rede locais (`advplc serve`, `--debug-port`, REST,
gRPC, P2P), parsers de arquivo não confiável (GGUF, `.bc`, RPO/APO) e o
pipeline preprocessor → lexer → parser. Baseline: `go vet -unsafeptr=false
./...` limpo e `go test ./...` verde antes das mudanças.

## Achados corrigidos

| # | Severidade | Componente | Problema | Correção | Teste |
|---|---|---|---|---|---|
| 1 | Alta | `pkg/webui/server.go` (`advplc serve`) | CSRF: qualquer página aberta no browser disparava `GET /events`, e cada requisição executa o programa. DNS rebinding (domínio do atacante resolvendo para 127.0.0.1) permitia ler os eventos e responder diálogos via `/reply`. | Middleware `sameOrigin`: `Host` precisa ser loopback/IP literal ou nome listado em `ADVPL_SERVE_HOST`/`ADVPP_WEBUI_ALLOWED_HOSTS`; `Origin`, se presente, igual ao `Host`. `/reply` só `POST`, corpo ≤ 8 MB; teto de 64 sessões; `ReadHeaderTimeout` 10 s. | `pkg/webui/origin_test.go` |
| 2 | Alta | `pkg/dap/dap.go` (`--debug-port`) | Ataque cross-protocol: o leitor de frames ignorava linhas desconhecidas, então um `fetch()` de página web para a porta TCP (request line + headers HTTP + corpo JSON) virava comando DAP, inclusive `launch`. `Content-Length` sem teto → `make()` gigante → OOM. | Só `Content-Length`/`Content-Type` aceitos (qualquer outra linha aborta); corpo ≤ 16 MB. | `TestReadMessageRejectsHTTP` |
| 3 | Média | `pkg/llm/gguf.go` | Contagens e tamanhos do header GGUF (nº de tensores/KV, tamanho de string/array, `nDims`) alocados sem limite → OOM/pânico com arquivo malformado; `general.alignment = 0` → divisão por zero; offset+tamanho de tensor não checado contra o arquivo. | Toda contagem limitada ao tamanho do arquivo, crescimento por `append` (memória proporcional ao lido), `nDims ≤ 4`, arrays aninhados ≤ 8, alinhamento 0 rejeitado, `checkBounds` antes de ler tensor. | `pkg/llm/gguf_malformed_test.go` |
| 4 | Média | `pkg/compiler/bytecode.go` (`advplc exec`) | `.bc` adulterado: `numGlobals`/`NumLocals` negativos → pânico; gigantes (2⁴⁰) → OOM fatal não recuperável; `mainOffset`/offset de função fora do código → pânico. | `validate()` em `LoadBytecode`: slots ≤ 65536, offsets dentro do código, operandos `|Arg| ≤ 2²⁴`. | `TestLoadBytecodeRejectsHostile` |
| 5 | Baixa | `pkg/preprocessor/commands.go` | `#command`/`#xcommand` com `[` sem `]` → pânico (slice out of range) no compilador. Achado por fuzzing. | Grupo opcional sem fechamento vai até o fim do padrão. | corpus `pkg/parser/testdata/fuzz/FuzzPipeline/` |

Bug funcional achado no mesmo processo (job de determinismo): literal
numérico que estoura `float64` (ex.: `1.8e308` → `+Inf`) fazia
`advplc compile` falhar com `json: unsupported value: +Inf`. `Constant`
ganhou `MarshalJSON`/`UnmarshalJSON` (±Inf/NaN como string; finitos
continuam número, `.bc` antigos carregam igual). Teste:
`TestBytecodeNonFiniteConstants`.

## Fuzzing

- `pkg/parser/fuzz_test.go` — `FuzzPipeline` (preprocessor → lexer →
  parser): 2,25 M execuções / 120 s sem falha após o fix #5.
- `pkg/rpo/fuzz_test.go` — `FuzzParse` (`Parse`, `ParseApoBlob`,
  `APOParser.ExtractAll`): 1,6 M execuções / 40 s sem falha.

Rodar: `go test ./pkg/parser -run '^$' -fuzz FuzzPipeline -fuzztime 60s`.

Entradas adversariais manuais (1000+ níveis de `(`/`{`, 50 000 `If`
aninhados, `#include` recursivo, `#define A A+A`, 200 KB aleatórios):
todas terminam com erro limpo ou sucesso, sem pânico nem travamento.

## Revisado e mantido (por design)

- `WaitRun`, I/O de arquivo, SQL e DynCall do AdvPL: capacidades da
  linguagem, executam com os privilégios de quem roda o programa.
- `WSRestServer`/`GRPCServer` escutando em todas as interfaces: é o
  servidor da aplicação do usuário (endereço definido no fonte). REST já
  tinha timeouts e teto de corpo.
- `InsecureSkipVerify` só com `ADVPP_HTTP_INSECURE` explícito.
- `docker exec bash -c` em `cmd/advplc/cmd_rpo.go`: argumentos vêm da
  linha de comando local do operador.

## Limitação conhecida

A validação do `.bc` é estrutural, não semântica por opcode: um operando
dentro do teto mas inválido para o opcode ainda vira pânico Go (seguro em
memória, sem OOM). Validação por opcode fica para quando `.bc` de terceiros
virar caso de uso real.
