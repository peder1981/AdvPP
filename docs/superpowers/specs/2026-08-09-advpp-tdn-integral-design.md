# AdvPP — Cobertura Íntegra do TDN AdvPL (Design)

## Objetivo

Implementar no compilador/VM AdvPP (Go) todos os recursos documentados no
espelho local do TDN AdvPL (`~/tdn-advpl-mirror/`, 991 páginas, TLPP fora de
escopo), como uma nova versão "grande" do compilador. Cobertura íntegra
significa: toda função, classe de framework e comando com especificação real
no TDN passa a existir no AdvPP, testada.

## Estado atual (levantado em 2026-08-09)

- **Natives implementadas hoje:** ~258 (`pkg/vm/natives.go` + arquivos por
  domínio: `httpclient_native.go`, `mail_native.go`, `asyncjob_native.go`,
  `mathstat_native.go`, `tensor_native.go`, `p2p_native.go`, `mcp_native.go`,
  `rest_native.go`, `llm_native.go`, `autograd_native.go`,
  `geometry_native.go`).
- **Suporte de linguagem a classes:** completo (`Class()...EndClass`,
  herança via `from`, métodos estáticos, despacho de método) — falta apenas
  o *catálogo* de classes de framework que o TOTVS entrega prontas.
- **Engine de comandos** (`#command`/`#xcommand`, v1.8.0): completo.
- **Compiler directives:** as 9 documentadas no TDN já existem
  (`#command`, `#define`, `#ifdef`, `#ifndef`, `#include`, `#translate`,
  `#xcommand`, `#xtranslate`).

## Gap levantado (contagens aproximadas — refinadas por categoria em cada plano de fase)

| Área | Documentado no TDN | Já implementado | Gap aproximado |
|---|---|---|---|
| Functions (36 categorias) | ~750 páginas (~588 com conteúdo real, ~162 stubs genuínos no próprio TDN) | ~258 | ~400-550 |
| Classes de framework | 97 (Visual=59, Não Visual=22, Mobile=6, Diálogo=2, + índices) | 0 (só suporte de linguagem) | ~89 |
| Comandos | 23 | parcial (subset via engine de comandos) | a apurar por fase |
| Compiler directives | 9 | 9 | 0 (nenhum trabalho) |
| Deprecated (functions+classes) | listado | — | fora de escopo (excluir explicitamente) |
| Mensagens de erro/advertência | 78+10 | — | fora de escopo (strings de runtime, não funções) |

Deprecated e Mensagens de erro/advertência **não geram fases de
implementação** — são apenas listas de referência para não reimplementar
algo que a própria TOTVS descontinuou.

## Arquitetura

### Functions
Aditivo ao padrão já existente: um arquivo Go novo por categoria/família
(`<categoria>_native.go`), registrado em `registerNatives()`, mantendo
`natives.go` de crescer indefinidamente. Cada native seguindo exatamente o
padrão de assinatura/erro já usado pelas ~258 existentes.

### Classes de framework
Novo subsistema, paralelo ao de natives: **registro de classes
intrínsecas**. Cada classe de framework (TWindow, TGet, TBrowse, etc.) é
implementada como struct Go registrada num catálogo intrínseco da VM
(análogo a `v.natives[...]`, mas para tipos/métodos), despachada pelo mesmo
mecanismo de método já usado para objetos definidos pelo usuário via
`Class()...EndClass`. Classes visuais (família "Visual") chamam o renderer
PO-UI/MSDIALOG já existente por baixo — sem reimplementar rendering do
zero. Um arquivo Go por família: `classes_visual.go`,
`classes_naovisual.go`, `classes_mobile.go`, `classes_dialogo.go`.

Isso é 100% Go nativo: o código do usuário continua usando
`Class()...EndClass` normalmente (já funciona); é só o *catálogo* de
classes prontas do framework que ganha implementações Go, não uma nova via
de bootstrap AdvPL.

### Comandos
Os 23 comandos documentados que ainda não têm cobertura via engine de
`#command`/`#xcommand` existente ganham suas definições, seguindo o padrão
já usado pelos comandos hoje implementados.

## Rastreamento de gaps sem spec

Funções/classes/comandos cujo TDN é um stub genuíno (sem assinatura, sem
parâmetros, sem exemplo) **não são implementados às cegas**. Vão para um
ledger vivo em `docs/tdn-gap-stubs.md` — nome, categoria, URL da página
TDN, data do levantamento — para retomada futura se/quando surgir spec real
(corpus real de uso, atualização do TDN, ou pedido explícito do usuário).

## Fases (execução sequencial, uma por categoria/família)

Cada fase = 1 ciclo plano→implementação (via `subagent-driven-development`),
1 task no plano mestre = 1 categoria/família inteira. Ordem: categorias
pequenas de Functions primeiro (para validar o padrão de arquivo/teste
antes de escalar), depois as grandes, depois Classes por família (menor
risco primeiro), depois Comandos.

1. **Functions, pequenas → grandes** (ordem aproximada por contagem real,
   ajustada por dependência técnica — ex. Conversão de tipos antes de
   categorias que a usam em exemplos): Validação, Sincronismo de dados, Web
   Services, Execução entre processos, Integração Excel, Manipulação do
   bloco de código, Tratamento de e-mail, Interface-SFTP, Manipulação de
   memória, Controle de acesso, Matemática, Manipulação de variáveis
   globais, Verificação dos tipos de variáveis, SAML, Manipulação de
   classe, Manipulação de RPO, Manipulação de variáveis numéricas,
   Manipulação de matriz HashMap, Manipulação do arquivo INI, Decimais de
   Ponto Fixo, Manipulação de matriz, Tratamento de XML, Manipulação de
   Data/Hora, Controle de processamento, Controle de impressão, Conversão
   entre tipos e dados, Manipulação de variáveis globais HashMap, Interface
   HTTP, Manipulação de string, Manipulação de arquivos/discos/IO,
   Ambiente, Banco de Dados, Componentes de interface visual, Segurança.
2. **Classes**: Diálogo(2) → Mobile(6) → Não Visual(22) → Visual(59).
3. **Comandos**: os 23 documentados, descontando o já coberto.

A ordem exata dentro de cada grupo grande é refinada no plano de
implementação de cada fase (a spec fixa só a sequência de fases, não a
ordem interna de cada uma).

## Testes

Um `*_test.go` por arquivo de categoria/família, um `Test<Nome>` por
função/método/classe, cobrindo: caso feliz do exemplo documentado no TDN +
edge cases óbvios (args ausentes, tipos errados quando relevante). Segue o
padrão de teste já existente no repositório.

## Versionamento / release

Nenhuma release intermediária pública durante a série de fases — todas
acumulam num branch/série de commits até a última fase fechar. Ao final,
um único bump de versão "grande" (minor ou major, a decidir no momento —
ex. `v2.1.0` ou `v3.0.0`) com CHANGELOG consolidado de tudo que entrou,
seguindo o mesmo processo de release já usado (compilar, empacotar, vsix,
deploy laptop-peder + homelab, publicar no Marketplace — só que ao final da
série inteira, não a cada fase).

## Fora de escopo

- TLPP (fora do escopo do próprio espelhamento do TDN).
- Deprecated functions/classes (listadas só para não reimplementar).
- Mensagens de erro/advertência (strings de runtime, não recursos de
  linguagem/compilador).
- Funções/classes stub no próprio TDN (vão para o ledger de gaps, não
  bloqueiam a série de fases).
- Bootstrap de classes via AdvPL puro (decisão explícita: 100% Go nativo
  nesta rodada).
