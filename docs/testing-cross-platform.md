# Padrão de testes cross-platform (Linux/Windows/macOS)

Metodologia extraída da implementação do DynCall (`pkg/vm/dyncall_native.go`,
v3.0.1) — a primeira feature do AdvPP a exigir código realmente
plataforma-específico (FFI: `dlopen`/`LoadLibrary`, símbolos C++ mangled,
`unsafe.Pointer`). Três rodadas de bugs reais só apareceram na matriz de CI
(`ubuntu-latest`/`windows-latest`/`macos-latest`, `.github/workflows/test.yml`)
e nunca localmente — documentado aqui para não repetir a mesma investigação
do zero na próxima feature nessa categoria (drivers nativos, threads de SO,
qualquer coisa que toque API de plataforma).

## Regra central

**Build e teste locais (Linux, aqui) nunca são suficientes para código
platform-specific.** `go build`/`go test` local só prova que compila e
funciona no SEU sistema operacional. Qualquer coisa que toque:

- carregamento dinâmico de biblioteca (`dlopen`/`LoadLibrary`, `purego`, cgo)
- `unsafe.Pointer`/manipulação de memória crua
- symbol mangling C++/ABI de outro compilador
- arquivos temporários que outro processo pode ter aberto (`t.TempDir()`)
- qualquer `//go:build` ou `runtime.GOOS` no meio do código

... só está de fato validado depois que a CI rodar nas 3 plataformas. Não
declare uma feature dessas prontas com base em teste local — abra o PR/push,
espere a matriz completa (`gh run view <id> --json jobs`) e trate qualquer
falha como um bug real a investigar, nunca como "flake da CI" até prova em
contrário.

## Checklist ao escrever uma feature com código platform-specific

1. **Isole a parte platform-specific em arquivos com `//go:build`**, nunca
   `if runtime.GOOS ==` misturado em lógica geral quando a API muda de nome
   entre SOs (ex.: `pkg/vm/dyncall_dlopen_unix.go` vs `dyncall_dlopen_windows.go`,
   mesma assinatura de função, implementação trocada por build tag). Isso
   também vale para os arquivos de TESTE que chamam essas funções — se o
   teste chama a API específica de um SO diretamente (não através do
   wrapper), ele quebra a **compilação** (não só a execução) nos outros SOs.
2. **Nunca assuma que uma dependência não suporta todos os SOs alvo** sem
   checar a documentação dela por SO. `purego.Dlopen`/`Dlsym`/`Dlclose`
   simplesmente não existem em Windows (a doc da própria lib diz isso) —
   `go build` só acusa isso ao compilar PARA windows, nunca localmente em
   Linux. Rode `GOOS=windows GOARCH=amd64 go vet ./...` (não precisa emular,
   `go vet`/`go build` cross-compilam de graça) antes de confiar que uma
   dependência nova cobre as 3 plataformas.
3. **Verifique o resultado real do `go vet` com a flag exata usada pela
   CI**, não só o comando "puro". Este projeto roda
   `go vet -unsafeptr=false ./...` (`Makefile`, `test.yml`) — qualquer
   arquivo novo com `unsafe.Pointer` deve ser checado com essa MESMA flag
   antes do push, não com `go vet ./...` sem flag (que falharia sozinho,
   dando um falso alarme desnecessário, OU o oposto: passando localmente
   por acaso enquanto uma flag diferente na CI pega algo que você não viu).
4. **Testes que abrem um recurso do SO (arquivo, DLL, handle) devem
   fechá-lo antes do fim do teste**, via `t.Cleanup`, registrado depois de
   qualquer `t.TempDir()`/criação de arquivo que dependa disso (cleanups
   rodam em ordem LIFO — registre o fechamento do recurso DEPOIS de pedir o
   TempDir, para que feche antes da tentativa de apagar o diretório). Em
   Linux isso quase nunca importa (o SO deixa apagar arquivo aberto); em
   Windows um handle aberto (`LoadLibrary`) TRAVA o arquivo — a falha
   aparece como erro de limpeza do `t.TempDir()`, não como falha da lógica
   testada, o que engana quem só olha a primeira linha do erro.
5. **Fixtures C/C++ de teste (bibliotecas compiladas em tempo de teste)
   precisam ser escritas pensando em pelo meno 2 toolchains de compilador**
   (GCC/Clang no mínimo — Linux normalmente usa GCC, macOS usa Clang por
   trás de `gcc`/`g++`, mesmo comando). Métodos/funções definidos **dentro**
   do corpo da classe (`inline` implícito) são candidatos a COMDAT/merge
   entre unidades de tradução — cada compilador decide de forma diferente
   se e como exportar esses símbolos quando só há uso implícito (ex.:
   `new Classe()` chamando o construtor por baixo dos panos). Regra prática:
   **se o teste precisa resolver um símbolo por nome via `dlsym`/`Dlsym`
   (não só chamar de dentro da própria lib), defina esse método FORA do
   corpo da classe** (`Classe::Metodo() {...}` na `.cpp`, declaração
   separada no `class {...}`) — isso dá linkage externa comum, sem
   ambiguidade de merge, funcionando igual nas 3 toolchains. Não tente
   resolver isso só com `__attribute__((visibility("default")))` ou
   `__attribute__((used))` — essas anotações não bastam quando o problema é
   o método ser implicitamente `inline` (visto em CI real: o símbolo existia
   no binário, mas como símbolo LOCAL, não exportado — nem `used` nem
   `visibility` mudaram isso; só sair de dentro da classe resolveu).
6. **Quando uma correção quebra outra plataforma, não é "azar" — é sinal de
   que a correção mexeu em algo mais amplo do que a intenção.** Ex. real:
   adicionar `__declspec(dllexport)` numa classe para resolver macOS
   quebrou Windows/MinGW, porque QUALQUER uso de `dllexport` no módulo muda
   o linker do MinGW de "exporta tudo por padrão" para "exporta só o
   marcado explicitamente" — uma mudança de escopo de módulo inteiro, não
   just daquele símbolo. Prefira a correção mais estreita possível por SO
   (via `#ifdef`/build tag), e sempre reteste TODAS as plataformas depois
   de qualquer fix, não só a que estava falhando.

## Como diagnosticar sem adivinhar

Quando uma falha só acontece numa plataforma que você não tem acesso local
(este ambiente é Linux; não há runner Windows/macOS disponível para debug
interativo), **pare de tentar corrigir às cegas depois da 1ª ou 2ª tentativa
falhar de novo**. Em vez disso, transforme o próprio teste num instrumento
de diagnóstico e rode de novo:

- Imprima o erro completo (`GetErrorMsg()`/`err.Error()`), não só um booleano
  de sucesso/falha, na mensagem de falha do teste.
- Se a dúvida é "o símbolo existe no binário?", rode a ferramenta real do
  SO (`nm`/`nm -D`/`otool`/`dumpbin`, via `exec.Command`, dentro do próprio
  teste) e imprima a saída na falha — isso dá a resposta definitiva em vez
  de mais uma hipótese. Foi exatamente isso que resolveu o caso do macOS:
  duas tentativas de correção "razoáveis" (visibility, dllexport) não
  bateram na causa real; o dump de `nm` mostrou en poucos segundos que o
  símbolo existia mas como local (`t`, minúsculo) — informação que nenhuma
  suposição teria entregue tão rápido.
- Cada iteração desse ciclo é: push → aguardar a matriz da CI completar
  (`gh run view <id> --json jobs --jq '.jobs[] | "\(.name): \(.conclusion)"'`)
  → ler o log de falha real (`gh run view <id> --log-failed`) → só então
  decidir a próxima mudança. Não empilhe suposições sem confirmar a anterior.

## Caso de referência completo

`pkg/vm/dyncall_native.go` + `pkg/vm/dyncall_dlopen_unix.go` +
`pkg/vm/dyncall_dlopen_windows.go` + `pkg/vm/testdata/dyncall/` — histórico
de commits de `feat(vm): implementa DynCall` até
`fix(vm): construtor out-of-line no fixture C++` (branch `master`,
2026-08-12/13) mostra a sequência real: dependência sem suporte Windows →
`go vet` quebrando a CI → handle de DLL travando limpeza no Windows →
três hipóteses erradas sobre visibilidade de símbolo no macOS → diagnóstico
real via `nm` → causa raiz (`inline` implícito) → fix definitivo. Vale como
exemplo ponta a ponta antes de repetir o padrão numa feature nova.
