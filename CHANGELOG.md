# Changelog

Todas as mudanças notáveis deste projeto são documentadas aqui.

## [1.8.4] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (94,4% → 95,8%)

- **Bug estrutural**: `Static cVar := valor` DENTRO de uma função era
  tratado como fronteira de função (por causa de `Static Function`),
  truncando o corpo silenciosamente e corrompendo o parse do resto do
  arquivo (fonte dos piores "drift bugs"). STATIC agora só é boundary
  seguido de FUNCTION.
- DSL XML legado: `ADDNODE <expr> NODE <expr> ON <expr>`, `DELETENODE
  <expr> ON <expr>`, `CREATE <var> XMLFILE <expr> [SETASARRAY <lista>]`.
- Alvo de `Count/Sum/Average ... To` pode ser expressão (`self:nTotReg`).
- Keyword como identificador comum em posição de operando quando o token
  seguinte só continua expressão (`{|Panel| f(Panel, ...)}`).
- `Private &("nome"+var) := x` — memvar com nome computado por macro.
- `HEADERS` (plural) como cláusula de LISTBOX; WSMETHOD REST sem nome
  próprio (`WSMETHOD GET WSRECEIVE ... WSSERVICE X`).

## [1.8.3] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (92,6% → 94,4%)

- `@ nLin++` sozinho (forma degenerada real) tolerado como expressão.
- `End If` / `End Do` fechando If e Do Case (variantes de duas palavras).
- QUALQUER keyword seguida de `(` na mesma linha em contexto de expressão é
  chamada de função (`alias->(Add())`, `Select()`, ...) — generalização da
  lista IF/ARRAY/DATE/OBJECT/BREAK.
- Nome de função pode colidir com keyword (`Static Function Add`).
- `SEND MAIL FROM ... TO ... SUBJECT ... BODY ... [ATTACHMENT] [RESULT]` e
  `GET MAIL ERROR <var>` (DSL de e-mail do workflow).
- `MENU <var> POPUP ... MENUITEM ... ACTION ... ENDMENU` (menu de contexto).
- `DEFINE DBTREE ... CARGO ; ON CHANGE <expr>`; `DBADDTREE ... PROMPT ...
  RESOURCE ... CARGO ... OPENED` (árvores legadas).
- Cláusulas de `@`: `FROM` (METER FROM 0 TO 100), `PICT` (abrev. de
  PICTURE), `OPTION` (FOLDER), flag `RIGHT`; `FIELDS` pode levar lista de
  valores própria (`LISTBOX ... FIELDS "" ; HEADER ...`).

## [1.8.2] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (89,4% → 92,6%)

- `&&` (comentário Clipper) após `;` de continuação tratado como fim de
  linha para a continuação (lexer), igual ao `//` já suportado.
- `Begin Report Query <expr>` — a seção pode ser expressão completa
  (`oReport:Section(2)`), não só um nome, nos dois lados do bloco.
- Atribuição encadeada como valor dentro de item de codeblock
  (`x[9] := x[10] := ... := 0`).
- `COLORS` como cláusula de `@` (FOLDER ... COLORS 0,167...).
- `Release Object <nome>` / `Release All [Like <máscara>]`.
- `Break(oErro)` como chamada de função em expressão (idioma de
  ErrorBlock), distinto do statement BREAK.
- `@ y,x To y2,x2 MultiLine Object oMulti` — flags/cláusulas do TMultiget
  legado no ramo de caixa do `@`.
- `Data <keyword>` — nome de membro de classe pode colidir com palavra
  reservada (`Data size`, `Data default`); `::Default()` idem em acesso.
- `DEFINE SCROLLBAR ::oVScroll VERTICAL OF Self RANGE a,b` — alvo `::prop`
  em DEFINE e ACTIVATE, flags VERTICAL/HORIZONTAL e cláusula RANGE.
- Bytes de controle soltos (\x01, corrupção comum em fontes legados)
  tolerados pelo lexer, como o backtick.
- `Count To <var>` / `Sum <exprs> To <vars>` / `Average ... To ...`
  (comandos Clipper de agregação; parseados e descartados).
- Atribuição em resultado de chamada (`ATail(arr) := v`, semântica de
  referência do Clipper) tolerada no codegen (avalia e descarta), mesma
  tolerância já dada a `&macro := v`.

## [1.8.1] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (87,6% → 89,4%)

- Resolução de #include: tenta subpastas convencionais (`ch/`, `include/`,
  `includes/`) e fallback case-insensitive por diretório (fontes CP-1252
  vindos de Windows quase nunca batem o case do disco em Linux).
- Junção de linha lógica antes do casamento de #command (`Store COLS ... ;`
  + `While ...`), preservando contagem de linhas; comentário `//` após o
  `;` de continuação não esconde mais a marca (definições e uso); CRLF
  (`;\r`) tratado em todos os joins.
- Operador de exponenciação `^` e sinônimo `**` (parsePower, associativo à
  direita, acima da multiplicação; novo OP_POW na VM via math.Pow).
- `ACTIVATE FWMBROWSE/MBROWSE/REPORT <var>`.
- DSL mobile FDA: `ADD FOLDER ... CAPTION ... [ON ACTIVATE f()] OF ...`,
  `ADD COLUMN ... TO ... ARRAY ELEMENT n HEADER ... WIDTH n`,
  `SET BROWSE <var> ARRAY <expr>`, cláusulas `CAPTION`/`SYMBOL`/`ITEM`/
  `VSCROLL` e flags de duas palavras `NO SCROLL`/`NO UNDERLINE` em `@`.

## [1.8.0] — 2026-07-10

### Motor de #command reescrito para semântica Clipper real + 2 features de lexer (86,4% → 87,6%)

Rodada dirigida pelos arquivos que exigiam o pré-processador de verdade:

**Motor #command/#xcommand (pkg/preprocessor):**
- Junção de definição multi-linha agora REMOVE o `;` de continuação de fim
  de linha (antes ficava dentro do padrão como literal impossível de casar,
  desativando silenciosamente toda regra multi-linha); um `;` no meio/começo
  de linha do resultado é conteúdo (separador de comandos gerados).
- Regra definida por ÚLTIMO vence (ordem reversa de tentativa), semântica
  Clipper que permite um .ch especializar comando já definido.
- Cláusulas opcionais consecutivas casam em QUALQUER ORDEM (ex.: padrão
  declara `[OF <oWnd>] [PIXEL]`, fonte usa `PIXEL OF oPanel`).
- Marcador restrito multi-literal `<nome: LIT1, LIT2, ...>` (captura qual
  literal casou; `<.nome.>` vira .T./.F.).
- Marcador guloso para em QUALQUER literal alcançável à frente (união dos
  abridores de todos os grupos opcionais + próximo literal obrigatório),
  não só no primeiro.
- Marcador dentro de grupo opcional herda o ponto de parada do padrão
  externo (lista, não um único literal).
- Grupos opcionais `[...]` do lado do RESULTADO: emitidos sem os colchetes
  só se algum marcador interno capturou algo (antes viravam `[, ]` literal).
- Marcador de resultado `<"var">` (stringify: nome capturado entre aspas).
- Expansão re-processada recursivamente (o resultado pode conter novos
  comandos, ex.: VTSAY+VTGET expande para dois comandos `@...` que expandem
  de novo), segmentada por `;` de topo.
- Comentário `//` de fim de linha removido antes do casamento (um marcador
  guloso o capturava para dentro da expansão, comentando o resto do código
  gerado).
- Flag `[<nome: LITERAL>]` com espaços ao redor de `:`/`,` normalizada.

**Lexer:**
- Literal de string entre colchetes do Clipper: `[texto]`/`[]` em posição
  de operando é string (heurística clássica: `[` após token que encerra
  operando é indexação; caso contrário, literal até o `]` da mesma linha).
- `BeginContent var <nome> ...conteúdo cru... EndContent` (bloco TLPP de
  JSON/XML embutido) — corpo não é AdvPL e não deve ser tokenizado;
  consumido no lexer e emitido como `<nome> := "<corpo>"`.

**Parser:**
- Parâmetro de codeblock pode colidir com palavra reservada
  (`{|Self| ...}`) — aceita keyword como nome.

## [1.7.11] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (86,0% → 86,4%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs reais adicionais de parser corrigidos:

- `WSMETHOD GET <nome> PATHPARAM <param> WSRECEIVE ...` — cláusulas REST
  `PATHPARAM`/`QUERYPARAM` (binding de parâmetro de rota) não reconhecidas
  na implementação do WSMETHOD.
- `Default Self:Prop := valor` — alvo de `Default` explicitamente escrito
  como `Self:Prop` em vez do atalho `::Prop` já suportado.

## [1.7.10] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real

- `@ y,x BMPBUTTON TYPE n ACTION expr` — cláusula `TYPE` (número de estilo
  do botão bitmap) não reconhecida no laço de cláusulas de `@`.

## [1.7.9] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (85,2% → 86,0%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).

- **Bug estrutural (lexer)**: literal numérico com ponto decimal sem
  dígito antes (`.5`, `.7` — comum em coordenadas `@ .5,.7 ...`) não era
  reconhecido; o lexer só entrava no tokenizador de número ao ver um
  dígito primeiro, então um `.` inicial caía sempre no caminho de
  dot-literal/operador (`.T.`, `.AND.`, `.`), nunca no de número. Corrigido:
  um `.` seguido de dígito agora entra no tokenizador de número.

## [1.7.8] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (84,4% → 85,2%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs reais adicionais de parser corrigidos:

- `DEFINE CELL ... BLOCK{||...} ...` — cláusula de bloco de valor da
  coluna do TReport não reconhecida.
- `@ x1,y1 TO x2,y2 DIALOG <var> TITLE "..." [PIXEL]` — sintaxe legada de
  criação de diálogo via `@ ... TO ...` (equivalente a `DEFINE MSDIALOG
  <var> FROM x1,y1 TO x2,y2 TITLE ...`), confundida com o desenho de caixa
  (`@ ... TO ... BOX`); nenhum dos dois é o outro, precisavam de ramos
  separados.

## [1.7.7] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (83,6% → 84,4%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs reais adicionais de parser corrigidos:

- `Private M->NOME_CAMPO := valor` — idioma Clipper de qualificar
  explicitamente uma memvar com o alias "M" (memory), redundante mas usado
  em fontes reais para dar nome de memvar igual a um campo; `Local`/
  `Private`/`Public`/`Static` só aceitavam um nome simples, não o padrão
  `M->nome`.
- `@ ... MSGET ... HASBUTTON F3 "..." ...`, `@ ... WORKTIME ... RESOLUTION
  <expr> VALUE <expr> ...` — cláusulas de `@` não reconhecidas
  (`HASBUTTON` do MSGET com botão de F3; `RESOLUTION`/`VALUE` do controle
  WORKTIME).

## [1.7.6] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (83,0% → 83,6%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).

- `alias->END` (e qualquer outro campo cujo nome colide com palavra
  reservada, ex. `alias->DELETE`) — nome de campo após `->` exigia
  `TOKEN_IDENT`; agora usa `expectName()` (aceita `TOKEN_KEYWORD`
  também), mesma classe de bug já corrigida em outros pontos do parser
  para identificadores que colidem com keywords.

## [1.7.5] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (82,0% → 83,0%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs reais adicionais de parser corrigidos:

- Elemento de array literal `{a, b := c, d}` (sem `||`, usado como
  sequência de expressões em cláusulas VALID/ACTION reais) não aceitava
  atribuição (`:=`) como elemento — usava `parseExpression` puro em vez de
  `parseCodeBlockItem`, mesma classe de bug já corrigida em outros pontos
  do parser para `:=` inline.
- `Return target := value` (atribuição usada inline como valor de retorno,
  ex. `Return self:oProp := {...}`) deixava o `:=` pendurado — corrigido
  nos DOIS parsers de RETURN existentes (`expressions.go` e o parser de
  corpo de método em `parser.go`, que tem sua própria cópia da lógica de
  RETURN) para usar `parseAssignRHS` em vez de `parseExpression`.

## [1.7.4] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (81,0% → 82,0%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Seis bugs reais adicionais de parser corrigidos:

- `@ ... RADIO/CHECKBOX ... 3D SIZE w,h ...` — flag de layout "3D"
  (tokeniza como NUMBER "3" + IDENT "D") não reconhecida no laço de
  cláusulas de `@` (só existia o caso equivalente em `DEFINE`).
- `LOCATE FOR <expr> [WHILE <expr>]` — comando Clipper de busca sequencial
  no alias atual, não suportado (nenhum dispatch existia).
- `Copy File <expr> To <expr>` — cópia de arquivo em disco (comando
  Clipper), distinto de `Copy To` (exportação de registros); confundia-se
  com este e quebrava o parsing.
- `Copy <alias-expr> To Memory <name> [Blank]` — copia a estrutura de
  campos de um alias para um array; forma de `COPY` não reconhecida.
- `@ ... GET ... MULTILINE ... HSCROLL ...` — cláusulas do GET multilinha
  (TGet memo) não reconhecidas.
- `DEFINE SBUTTON ... ONSTOP <expr> ...` — cláusula de tooltip do botão
  não reconhecida.

## [1.7.3] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (76,6% → 81,0%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Dois bugs estruturais de alto impacto e mais quatro bugs pontuais de parser
corrigidos:

- **Bug estrutural**: strings `"..."` sem aspa de fechamento até o fim da
  linha (typo comum em fontes reais, ex. query SQL multi-linha via `+=`)
  travavam o lexer, que consumia até a próxima aspa em QUALQUER linha
  seguinte, engolindo o resto do arquivo. Clipper/AdvPL fecha strings
  implicitamente no fim da linha; o lexer agora faz o mesmo.
- **Bug estrutural**: um identificador seguido de `(` seguinte, sem guarda
  de mesma linha, era sempre tratado como chamada de função — já que
  newlines são removidas antes do parsing, um `(alias)->campo` no início da
  PRÓXIMA linha grudava como argumento da chamada do identificador da
  linha anterior (`var := f() \n (alias)->campo := x` virava
  `f()(alias)->campo`). Corrigido em dois pontos: `parsePrimary` (chamada
  direta `ident(`) e `parsePostfix` (chamada após expressão composta),
  ambos agora exigem que o `(` esteja na mesma linha do token anterior.
- `alias->(expr1, expr2, ...)` — sequência separada por vírgula dentro do
  escopo de alias só aceitava uma única expressão; agora usa a mesma
  produção de `(a, b, c)` (avalia todas, retorna a última).
- `DEFINE SECTION ... TABLES "A","B",...`, `DEFINE CELL ... PICTURE "..."`,
  `DEFINE BREAK ... WHEN {||...}`, `DEFINE FUNCTION ... FUNCTION SUM BREAK
  oBreak TITLE "..." NO END SECTION` — cláusulas do DSL de TReport
  (`DEFINE SECTION/CELL/BREAK/FUNCTION`) não reconhecidas: `TABLES`,
  `PICTURE`, `WHEN`, `FUNCTION` (como nome de cláusula, colide com o nome
  do próprio DEFINE kind), `BREAK`, e o flag de três palavras `NO END
  SECTION`.

## [1.7.2] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (73,8% → 76,6%)

Continuação do sweep dirigido por corpus (ver [[advpp_corpus_locations]]).
Seis bugs reais adicionais de parser corrigidos:

- `If x := cond` / `While x := cond` — atribuição usada inline como
  condição (idioma comum em AdvPL: "avança e testa") deixava o `:=`
  pendurado; a condição agora usa `parseAssignRHS` como o resto do parser.
- Caminho de namespace TLPP totalmente qualificado
  (`totvs.framework.treports.date.stringToTimeStamp(...)`) quebrava
  sempre que um segmento colidia com palavra reservada (`date`, que lexa
  como `TOKEN_KEYWORD`); o loop de segmentos só aceitava `TOKEN_IDENT`.
  Mesmo problema corrigido em `NAMESPACE`/`USING NAMESPACE`, agora via
  `parseNamespacePath` compartilhado (segmento só aceito logo após um
  ponto, nunca solto — não avança para além da declaração).
- `WSRESTFUL/WSSERVICE <nome> ... FORMAT <expr>` — cláusula de cabeçalho
  não reconhecida, quebrando o corpo inteiro do bloco.
- **Bug de colisão de nome em `WSDATA`**: o bypass "nome de método é
  opcional" (`WSMETHOD POST DESCRIPTION "..." ...`) se aplicava também a
  `WSDATA`, então um campo literalmente chamado `Description`
  (`WSDATA Description As String`) era confundido com a cláusula
  `DESCRIPTION` e o parser pulava o nome do campo — struct inteira
  corrompida a partir daí. `WSDATA` agora sempre exige nome explícito.
- `ParamType <n> Var <nome> As <tipo> [Default <expr>]` — declaração de
  metadados de parâmetro não suportada.

## [1.7.1] — 2026-07-10

### Sweep de pass-rate no corpus Protheus real (70,8% → 73,8%)

Continuação do sweep dirigido por corpus contra os fontes reais 811R4 e
12.1.2510 (amostra de 500 arquivos, ver [[advpp_corpus_locations]]).
Onze bugs reais de parser corrigidos:

- `SET KEY <nKey> TO [<uBlock>]` — o keycode antes do `TO` não era
  reconhecido pelo dispatcher de `SET`.
- `DEFINE CELL ... AUTO SIZE` (TReport) — flag `AUTO` sem valor antes de
  `SIZE` não era reconhecida, quebrando o parsing da cláusula seguinte.
- Drift em `SET FILTER TO` — a heurística "tem valor?" não detectava que
  um `x += ...` na linha seguinte não era o valor do `SET`, engolindo a
  variável errada.
- `DELETE FILE <expr>` e `DELETE [FOR/WHILE/RECORD/REST/ALL]` — comandos
  Clipper de arquivo/registro não eram suportados.
- `WSMETHOD ... WSRECEIVE a,b WSSEND c` — só aceitava `WSSEND` antes de
  `WSRECEIVE` e com valor único; real Protheus usa qualquer ordem e listas
  separadas por vírgula em ambas.
- `PREPARE ENVIRONMENT EMPRESA/FILIAL/MODULO/TABLES` — comando batch de
  abertura de ambiente não suportado.
- `SET DELETE ON` — "DELETE" é palavra reservada (`TOKEN_KEYWORD`), não
  identificador; o dispatcher de `SET` exigia `TOKEN_IDENT` e falhava.
- **Bug estrutural**: como quebras de linha são descartadas antes do
  parsing, um `++x` prefixo iniciando uma nova instrução colava-se ao
  fim da expressão da instrução anterior (`y := f()` seguido de `++x`
  virava `(f())++`, erro de compilação "unsupported assignment target").
  Corrigido exigindo que o operador pós-fixo `++`/`--` esteja na mesma
  linha do token anterior.
- Atribuição encadeada (`a:=b:=c:=valor`) dentro de `Local`/`Private` não
  era suportada (só funcionava em atribuição solta).
- `For ... EndFor` (além de `Next`/`End`) não fechava o loop.
- `Default a:=1, b:=2, c:=3` (múltiplas variáveis separadas por vírgula)
  só suportava uma única variável.


### Motor real de `#xcommand`/`#command`/`#xtranslate`/`#translate`

Até aqui, o pré-processador **reconhecia** a sintaxe destas diretivas mas
**descartava** as definições — nenhuma expansão de verdade acontecia. Isso
quebrava qualquer arquivo que dependesse de comandos customizados definidos
em headers `.ch` reais (padrão comum em código Protheus legado: `STORE
HEADER <cA> TO <aH> [FOR <for>]`, `COPY <cAC> TO MEMORY [<bl:BLANK>]`,
etc.). Agora o AdvPP implementa o pattern-matching de verdade, no estilo
Clipper:

- **Padrão de casamento**: palavras literais (case-insensitive), `<nome>`
  (captura uma cláusula até o próximo literal esperado), `<nome,...>`
  (captura uma lista), `[...]` (grupo opcional — só tenta se o primeiro
  literal dele aparecer na posição atual), `[<nome:LITERAL>]` (marcador de
  flag booleana).
- **Molde de resultado**: `<nome>` (substitui pelo texto capturado, ou
  vazio se ausente), `<{nome}>` (vira `{|| texto}` se presente, `NIL` se
  ausente — usado para condições `FOR`/`WHILE` que viram codeblock),
  `<.nome.>` (`.T.`/`.F.` conforme presença), `\[`/`\]` (colchete literal).
- Definições multi-linha via continuação com `;` (convenção usual do
  Clipper) são unidas antes de compilar a regra.

Três bugs reais adicionais encontrados e corrigidos no caminho (achados ao
validar contra headers `.ch` reais de um fork ApSoft/Protheus):

1. **`#define` com múltiplos espaços** (`#define  NOME    valor`) —
   `parseDefine` usava `strings.SplitN(line, " ", 3)`, que quebra quando
   há mais de um espaço entre `#define` e o nome (comum em código real),
   armazenando a macro com nome vazio.
2. **`#define` multi-linha** (`#define NOME { "a","b",;\n "c","d" }`) —
   sem juntar as linhas de continuação, o resto do array vazava como
   código bruto (token solto no meio de uma statement).
3. **Tokenização por espaço simples** grudava identificador com pontuação
   colada (`TCSQLEXEC("select 1")` virava um token só), fazendo até um
   `#translate` sem parâmetros nunca casar; e quando um padrão casava só
   o início da linha, o resto era descartado em vez de reanexado.

Validado com testes automatizados (`pkg/preprocessor/commands_test.go`) e
contra arquivos `.prw`/`.ch` reais de um corpus de ~30 mil fontes Protheus
(legado 811R4 + versão 12.1.2510 atual) cedido pelo usuário para esta
investigação — usado só localmente para validação, não redistribuído.
Sem regressões: `make test` continua 30/30, `go vet` e os demais pacotes
seguem limpos, cross-compile OK em linux/windows/darwin (amd64+arm64).

## [1.6.0] — 2026-07-09

### `tests/real_protheus_test.prw` totalmente resolvido

O dump de 3785 linhas de código Protheus real usado como fixture de
estresse — que tinha uma falha de parser documentada como conhecida
desde antes desta série de correções — agora **compila e interpreta
sem nenhum erro** (`advplc check` e `advplc run`, ambos saem limpo).
Oito bugs reais e distintos encontrados e corrigidos por bisecção
binária (truncar o fonte progressivamente até isolar a menor entrada
que ainda reproduz o erro), além dos cinco já corrigidos na versão
anterior:

- `++nome` — incremento **prefixo** (só o pós-fixado `nome++` estava
  implementado).
- `@ ... LISTBOX ... FIELDS HEADER a,b,c ... ON DBLCLICK expr
  NOSCROLL OF window PIXEL` — cláusulas do LISTBOX (`FIELDS`,
  `HEADER`, `ON <evento> <expr>`, `NOSCROLL`) não reconhecidas.
- `@ y,x BUTTON var PROMPT "texto" ...` — cláusula `PROMPT` do BUTTON
  não reconhecida.
- `IF ( aArray[ i , j ] )` — o lookahead que desambigua bloco `If`
  de `IF(cond,then,else)` (adicionado na correção anterior) contava
  a vírgula de um índice multi-dimensional `[i,j]` como se fosse a
  vírgula de topo do `IF(...)`, tratando incorretamente todo `If`
  cuja condição usa um array 2D como a forma de chamada.
- `f(aArray[i] := valor, ...)` — atribuição como argumento de função
  quando o alvo não é um identificador simples (só `ident := valor`
  virava atribuição; `array[i] := valor` ficava com o `:=` sobrando).
- `@ y,x RADIO var VAR nVar ITEMS v1,v2,...` — cláusula `ITEMS` do
  RADIO não reconhecida.
- `Do Case ... End Case` — só `EndCase` (uma palavra) era aceito como
  fechamento; `End Case` (duas palavras, forma clássica do Clipper)
  não.
- `FindFunction("Nome")` — nativa ausente (usada no Protheus real para
  checar a existência de funções opcionais/AddOn antes de chamá-las).
  Implementada: verifica natives registradas e funções do bytecode
  (com/sem prefixo `U_`).

Sem regressões: `make test` agora dá **30/30** fixtures (antes eram
29/30, com esta sendo a única falha conhecida); `go vet ./...` e os
testes de `pkg/llm`/`pkg/mcp`/`cmd/advplc` continuam limpos;
cross-compile OK em linux/windows/darwin (amd64+arm64).

## [1.5.0] — 2026-07-09

### Servidor MCP nativo (classe `MCPServer`)

O AdvPP agora fala **MCP (Model Context Protocol)** de verdade — ao
contrário do suporte a REST (`WSRESTFUL`/`@Get`/`@Post`), que hoje é só
sintaxe reconhecida e descartada (sem servidor HTTP nem despacho real), a
classe `MCPServer` sobe um servidor **funcional**: JSON-RPC 2.0 sobre
stdio, expondo funções AdvPL/TLPP como "tools" que qualquer cliente MCP
(Claude, outros agentes) pode listar e chamar.

- **`pkg/mcp`**: núcleo do protocolo em Go puro (sem CGO, sem
  dependências externas) — `initialize`, `notifications/initialized`,
  `tools/list`, `tools/call`, `ping`; transporte stdio com uma mensagem
  JSON por linha.
- **Classe `MCPServer`** (`pkg/vm/mcp_native.go`):
  ```advpl
  oMCP := MCPServer():New("meu-servidor", "1.0.0")
  oMCP:AddTool("soma", "Soma dois números", ;
      '{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}', ;
      "ToolSoma")
  oMCP:Serve() // bloqueia lendo/escrevendo em stdin/stdout

  User Function ToolSoma(oArgs)
  Return cValToChar(oArgs:A + oArgs:B)
  ```
  Cada chamada de tool roda a função registrada numa VM isolada (mesmo
  mecanismo do `StartJob`) — necessário porque `Serve()` já está no meio
  da execução da VM principal quando uma `tools/call` chega; chamar a
  função direto na mesma VM corromperia a pilha de chamadas em andamento
  (bug real encontrado e corrigido durante o desenvolvimento).
  `Serve()` redireciona `ConOut`/console para stderr automaticamente,
  para não misturar saída de depuração com as mensagens JSON-RPC no
  stdout.
- Funciona com **`advplc run`** normal — não precisa de um comando novo.

**Validado com o SDK oficial em Python do MCP** (não só testes internos):
handshake `initialize`, `list_tools`, `call_tool` — ver
`cmd/advplc/mcp_integration_test.go`.

### Correções no parser (encontradas caçando um bug pré-existente)

Investigando uma falha antiga documentada em
`tests/real_protheus_test.prw` (um dump de 3785 linhas de código
Protheus real usado como fixture de estresse) via bisecção binária
(truncar o fonte progressivamente até isolar a menor entrada que ainda
reproduz o erro), foram encontrados e corrigidos cinco bugs reais e
distintos de parsing:

1. `&nome.` — o ponto final (terminador explícito clássico do
   Clipper/AdvPL para a substituição de macro) não era consumido.
2. `&nome.()` / `&(expr)()` — chamada de função cujo nome vem de uma
   macro; os parênteses da chamada não tinham dono no parser (mesma
   simplificação já usada para `alias->&macro`: sintaxe consumida, sem
   modelar a invocação dinâmica — o VM não resolve função por nome em
   runtime).
3. `@ y,x GROUP var TO y2,x2 OF window LABEL "..." PIXEL` — a cláusula
   GROUP do comando `@` de diálogo (caixa de agrupamento) usa `TO` e
   `LABEL` como cláusulas, não reconhecidas antes.
4. `ACTIVATE DIALOG oDlg ON INIT ... CENTERED` — variante clássica (sem
   o prefixo "MS") do já suportado `ACTIVATE MSDIALOG`.
5. `IF(cond, then, else)` usado como **statement isolado** (resultado
   descartado) — sempre caía no parser de bloco `If/EndIf`, que não
   trata `(...)` com vírgulas como chamada. Novo lookahead
   (`isInlineIfCall`) desambigua da forma bloco `If (cond) ... EndIf`.

`tests/real_protheus_test.prw` avança de ~503 para ~2414 das 3785
linhas antes de esbarrar no próximo gap (não mais um bug de parsing,
uma feature genuinamente não implementada) — mantido como falha
conhecida documentada no Makefile.

## [1.4.0] — 2026-07-09

### Motor de inferência LLM embutido (`pkg/llm` + classe `LLM`)

Novo: um motor de inferência para modelos de linguagem quantizados em
**I2_S** (ternário, formato BitNet), escrito 100% em Go — sem CGO, sem
`llama.cpp`, sem dependências de terceiros — compilando e rodando
identicamente em Linux, Windows e macOS (amd64 e arm64). Validado
**token a token** contra o `llama.cpp` de referência (fork BitNet do
projeto) usando o modelo `Falcon3-3B-Instruct-1.58bit`.

- **Parser GGUF** (`pkg/llm/gguf.go`): header, metadados e tensores lidos
  sob demanda (não carrega o arquivo inteiro em memória de uma vez).
- **Kernel ternário I2_S** (`pkg/llm/i2s.go`): dequantização e matmul
  contra ativações int8, replicando byte a byte o algoritmo de
  `ggml-quants.c`.
- **SIMD AVX2** (`pkg/llm/simd_amd64.s`, amd64): o dot-product ternário
  em assembly Go (VPMADDUBSW/VPSRLW), com detecção de CPU em runtime via
  CPUID e fallback automático para o caminho escalar em CPUs sem AVX2 —
  ou em qualquer arquitetura fora de amd64 (arm64 usa o escalar puro já
  validado; sem assembly não testável nesta arquitetura).
- **Forward pass completo** (`pkg/llm/model.go`): transformer arquitetura
  "llama" (GQA, RoPE, RMSNorm, FFN SwiGLU) com KV cache incremental.
- **Tokenizer BPE** (`pkg/llm/tokenizer.go`): byte-level estilo GPT-2,
  usando o vocabulário/merges já embutidos no próprio GGUF.
- **Amostragem** (`pkg/llm/sampling.go`): greedy, temperatura, top-k, top-p.
- **Classe AdvPL/TLPP `LLM`** (`pkg/vm/llm_native.go`): expõe o motor
  como native, no mesmo padrão de `FWMBrowse`/`MsDialog`:
  ```advpl
  oLLM := LLM():New("/caminho/modelo-i2_s.gguf")
  cTexto := oLLM:Generate("The capital of France is", 6, 0)  // prompt, nMaxTokens, nTemperatura
  aTokens := oLLM:Tokenize("algum texto")
  cTexto := oLLM:Decode(aTokens)
  oLLM:Close()
  ```

**Desempenho** (Falcon3-3B-1.58bit, 8 núcleos): ~5s/token com
paralelização por goroutines (matmul e atenção por faixa de
linhas/cabeças) + caminho rápido sem checagem de limite para blocos
ternários completos; AVX2 reduz mais ~1.6x sobre isso em amd64.

**Limitações conhecidas**: só arquitetura GGUF `"llama"` com pesos I2_S
(não `bitnet-b1.58` com as normas extras "SubLN"); pré-tokenizador
simplificado (não replica o split dígito-a-dígito específico da
Falcon3 — só afeta números com mais de um dígito); sem streaming
token-a-token na classe `LLM` (bloqueia até `Generate()` terminar); sem
suporte a outras quantizações (Q4_K, Q6_K etc.) nem outras arquiteturas.

## [1.3.0] — 2026-07-09

### Renderer web (`advplc serve`) — fases 1 a 4

Novo modo de execução: o programa AdvPL/TLPP roda no servidor (mesma VM,
mesmo `ADVPP.db`) e a interface é renderizada no browser. Basta o binário
`advplc` e um navegador — sem SmartClient, sem executável gráfico.

- **Fase 1 — console e diálogos**: `advplc serve <fonte> [--port N]`.
  `ConOut` é transmitido em tempo real; `MsgInfo`/`MsgStop`/`MsgAlert`/
  `MsgYesNo` bloqueiam a execução até a resposta do usuário no browser.
  Protocolo SSE + POST (stdlib pura, sem WebSocket). Cada aba/recarga é
  uma sessão com VM isolada e conexão própria ao banco.
- **Fase 2 — MVC → PO-UI**: frontend **PO-UI/Angular** (TOTVS) embutido
  no binário via `embed.FS`. `FWMBrowse():New()` + `SetAlias("SA1")` +
  `Activate()` renderiza um **`po-table`** com colunas e títulos vindos
  do dicionário **SX3** do `ADVPP.db`; Incluir/Editar abrem um
  **`po-dynamic-form`** gerado do dicionário; exclusão é soft-delete
  padrão Protheus (`D_E_L_E_T_='*'`). CRUD persistido no SQLite.
- **Fase 3 — hot reload**: `advplc serve <fonte> --watch` recompila a
  cada alteração do fonte e recarrega as sessões do browser
  automaticamente; erro de compilação aparece no console do browser.
- **Fase 4 — MSDIALOG legado**: `DEFINE MSDIALOG` + `@ linha,coluna
  SAY/GET/BUTTON` + `ACTIVATE MSDIALOG` viram um modal PO-UI por
  heurística de grade (controles agrupados em linhas por proximidade de
  `y`). O valor digitado nos `GET`s **escreve de volta nas variáveis**
  do programa (novo `FunctionInfo.LocalNames` no bytecode). `ACTION` de
  botão executa em VM isolada; `VALID`/`WHEN`/`ACTION` agora são lazy
  (embrulhados em codeblock, como o `#xcommand` real do Protheus).

### Infra

- `webui_port` na configuração compartilhada (`~/.advpp/advpp_config.json`);
  precedência: `--port` → config → 8080. Diretiva do projeto: toda nova
  configuração entra na Config compartilhada para futura edição via AdvCfg.
- Novo alvo `make web`: recompila o frontend PO-UI e embute em
  `pkg/webui/dist` (o dist é versionado — `go build` funciona sem Node).
- `SQLiteEngine` ganhou `QueryRows`/`Exec` (interface `vm.SQLEngine`).
- Fixtures novos: `tests/webui_test.prw`, `tests/mvc_browse_test.prw`,
  `tests/msdialog_test.prw`.

### Limitações conhecidas (fase 4)

- Codeblocks deste runtime não capturam variáveis locais: `ACTION
  {|| oDlg:End()}` não fecha o diálogo — por isso, qualquer clique de
  botão fecha o diálogo após executar o `ACTION`.
- `VALID` ainda não dispara round-trip por campo (planejado).

## [1.2.0] — 2026-07-08

### Multi-thread

- **`StartJob(cFunc, cEnv, lWait, params...)`** implementado no runtime:
  executa a função em uma VM isolada (semântica de work process do
  Protheus). Com `lWait=.F.` roda em goroutine e o processo aguarda os
  jobs pendentes antes de encerrar; cada job abre a própria conexão ao
  banco SQLite (WAL).
- **`FWGridProcess`** implementada conforme a documentação TDN:
  `New`, `SetThreadGrid`/`SetMaxThreadGrid` (pool de threads com
  backpressure), `CallExecute` (cada unidade em VM isolada com conexão
  própria), `Activate`/`Execute`, `StopExecute`, `IsFinished`,
  `SetAbort`, `SetAfterExecute`, meters (`SetMeters`/`SetMaxMeter`/
  `SetIncMeter`) e `SaveLog`/`GetLastLog`. Sem a interface gráfica de
  configuração (runtime headless).
- **`advplc check` paralelo**: aceita múltiplos arquivos (antes das
  flags) e verifica com 1 worker por CPU, com resumo `ok/failed`.

### Performance

- **Lexer ~95× mais rápido em arquivos grandes**: `tryDotLiteral` fazia
  `ToUpper` de todo o fonte restante a cada caractere `.` (O(n²)).
  Fonte real de 574KB: 9,1s → 0,095s. Corpus de 300 fontes reais do
  Protheus 12.1.2510 verificado em ~1,2s.

### Compatibilidade de linguagem

- Lexer tolera backtick solto fora de strings (typo presente em fontes
  reais da TOTVS aceito pelo compilador Protheus).

## [1.1.x] — 2026-07-08

### Banco de dados unificado

- **Banco padrão renomeado para `~/.advpp/ADVPP.db`** (era
  `./data/advpl_dictionary.db`, caminho relativo que quebrava fora do
  diretório do projeto).
- **Resolver único de caminho** (`shared.ResolveDatabasePath`) usado por
  todas as ferramentas: flag explícita → variável `ADVPP_DB` → config
  `~/.advpp/advpp_config.json` → legado `./data/` → padrão absoluto.
- **Ponto único de abertura** (`shared.OpenSQLite`) com pragmas WAL,
  `busy_timeout` e `foreign_keys` para todas as ferramentas.
- **VM conectado ao banco compartilhado**: `--db-path`/`ADVPP_DB` agora
  funcionam de fato no `advplc run`/`exec` (antes eram parseados e
  ignorados); a IDE também conecta o VM ao mesmo banco.
- Corrigido schema do dicionário: criação do zero falhava por colunas
  ausentes em SX2 (`X2_NOMEUSR`/`X2_MODULO`/`X2_TIPO`/`X2_DESCRIC`) e
  SX5 (`X5_TIPO`/`X5_TAMANHO`/`X5_DECIMAL`).
- Corrigida a heurística `banco.db/tabela` do driver SQLite, que
  quebrava qualquer caminho absoluto (agora só ativa quando o caminho
  não existe em disco; aceita `/` e `\`).

### Portabilidade (Linux / Windows 64 / macOS)

- **Driver SQLite trocado para `modernc.org/sqlite` (100% Go, sem
  CGO)**: o CLI cross-compila estaticamente para linux/windows/darwin,
  amd64 e arm64.
- **Removida a dependência do `iconv` externo**: conversão CP-1252 →
  UTF-8 é feita por conversor interno 100% Go, idêntico nas 3
  plataformas.
- `go.sum` versionado (estava incorretamente no `.gitignore`).

### Build, empacotamento e release

- **`Makefile`**: `make build` (4 ferramentas), `make test` (fixtures),
  `make cross` (CLI para 5 alvos), `make package VERSION=x.y.z`
  (pacotes em `dist/`), `make release VERSION=x.y.z` (tag + CI).
- **GitHub Actions** (`.github/workflows/release.yml`): a cada tag
  `v*`, builds nativos em Linux, Windows e macOS (incluindo as GUIs
  Fyne) e publicação automática dos pacotes `.tar.gz`/`.zip`/`.deb` na
  Release.
- `advplc version` mostra a versão embutida no build.
- Corrigido `.gitignore` que ignorava o diretório `cmd/advpp-ide`
  (o fonte da IDE não estava no repositório).

## [1.0.0]

- Versão inicial: compilador (lexer, preprocessador, parser, codegen),
  VM com natives, MVC, UI Fyne, ferramentas advcfg/adveditor/advpp-ide.
