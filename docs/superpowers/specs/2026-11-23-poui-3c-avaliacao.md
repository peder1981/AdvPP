# Onda 3c.4 — Avaliação: PoSync, PoSearchAi e po-code-editor

Data: 2026-11-23. Branch: `feat/poui-onda3c-servicos`.

Escopo do plano (`2026-11-22-poui-plano-integral.md` §4.4, Task 3c.4):
avaliar três itens do ecossistema PO-UI/TOTVS, escrever esta análise e
**parar para decisão do operador**. Nada aqui está implementado.

---

## 1. PoSync (`@totvs/protheus-lib-sync` / PoSyncService)

### O que é
Biblioteca da TOTVS para sincronização de dados offline-first: fila de
operações, sincronização incremental com o Protheus, resolução de
conflitos e persistência local (Dexie/IndexedDB por baixo).

### Uso real
- Apps TOTVS modernos (framework `protheus-lib-core`) usam PoSync em
  rotinas de campo (coleta, vistoria, força de vendas) onde a conectividade
  é intermitente.
- No corpus AdvPL do projeto, `FwCallApp` abre apps que consomem
  `protheus-lib-sync`; fontes como `FWA*Sync*` aparecem no acervo TOTVS.
- **No AdvPP**, o equivalente prático já existe em partes: o modo web é
  sempre online (SSE + REST na mesma origem); não há cenário offline real
  porque o "backend" é o próprio `advplc serve` local. Um app externo
  PO-UI aberto via `--poui-app`/`FwCallApp` poderia usar PoSync contra um
  Protheus real — mas aí o sync é responsabilidade do app JS, não do
  AdvPL.

### Dependência de backend
Alta: PoSync espera endpoints TOTVS (`/sync/*`, `rest` de sincronização,
`api/oauth2`) que o AdvPP não implementa. Teria de se embutir a lib JS no
app e implementar o contrato de sync do lado Go — projeto de semanas,
não de task.

### Custo
- **Alto.** Envolve portar um protocolo de sync (delta, conflitos,
  retentativa) e manter paridade com versões da lib.
- Valor para o AdvPP: baixo hoje — não há app AdvPL que declare uso de
  sync offline; o renderer web não tem modo offline.

### Recomendação
**Adiar / fora de escopo**, salvo demanda concreta de um cliente com app
offline. Se um dia necessário, o caminho é servir os endpoints `/sync` do
contrato TOTVS, não reimplementar a lib.

---

## 2. PoSearchAi (`@totvs/protheus-lib-ai-search` / PoSearchAiModule)

### O que é
Componente/directiva PO-UI que adiciona busca com IA (embeddings +
LLM) sobre dados — usa serviços TOTVS de IA (`api/ai/*`) e requer chave/
endpoint configurado.

### Uso real
- Apps TOTVS de consulta inteligente (busca em documentos, base de
  conhecimento) em versões recentes do framework.
- Não aparece em nenhum fonte AdvPL do corpus AdvPP nem nos apps de
  exemplo; é recurso de app JS, não de fonte `.prw`/`.tlpp`.

### Dependência de backend
Altíssima e externa: precisa de provedor de IA (endpoint TOTVS ou
OpenAI-compatible), embeddings e (idealmente) um índice vetorial. O
AdvPP não tem nada disso; o ChromaDB do homelab é infra do operador,
não do produto.

### Custo
- **Alto** em infraestrutura; o componente em si é trivial de embutir
  (uma tag + módulo), mas inútil sem backend.
- Exigiria decisão de produto: qual provedor, onde roda, custo por
  consulta, LGPD dos dados indexados.

### Recomendação
**Fora de escopo** até haver requisito de negócio explícito. Se aprovado
no futuro, implementar como serviço Go proxy (`/ai/search`) + embed do
componente, mantendo a chave fora do fonte AdvPL.

---

## 3. po-code-editor (CodeMirror do PO-UI)

### O que é
Editor de código do PO-UI (`po-code-editor`) construído sobre CodeMirror
— realce de sintaxe, lint, autocompletion, temas.

### Uso real
- Usado em apps TOTVS para edição de fórmulas, scripts e configurações.
- **O AdvPP já tem equivalente próprio**: `AdvCodeEditor`
  (`web/src/app/adv-code-editor.ts`), feito sob medida para AdvPL/TLPP —
  realce das linguagens do compilador (`advpl`, `tlpp`, `json`, `sh`,
  `text`), markers de erro/aviso vinda do compilador
  (`SetMarkers`/`GetMarkers`), `OnSave` (Ctrl+S), read-only e altura —
  tudo já integrado ao laço de eventos PoApp e aos testes da casa.

### Dependência de backend
Nenhuma nova: `po-code-editor` é componente puro de frontend.

### Custo
- **Baixo** para embutir como componente `PoCodeEditor` paralelo
  (tag + props), mas **duplicado**: dois editores de código na mesma SPA
  para o mesmo propósito.
- O diferencial do `po-code-editor` é autocompletion/lint genéricos do
  CodeMirror — o `AdvCodeEditor` já cobre o caso de uso real (edição de
  fonte AdvPL com diagnóstico do compilador), que é mais valioso que o
  genérico.

### Recomendação
**Não adotar como substituto.** Manter `AdvCodeEditor` como editor
padrão. Se um app precisar especificamente do `po-code-editor` (ex.:
contrato de um app TOTVS de terceiros), a alternativa de menor custo é
registrá-lo como "fora de escopo justificado" no `COMPONENT_STATUS.md`,
ou expor um `PoCodeEditor` fino que delega ao componente PO-UI — task
pequena, sob demanda.

---

## Síntese para decisão

| Item | Veredito sugerido | Esforço | Valor |
|---|---|---|---|
| PoSync | Adiar / fora de escopo | Alto | Baixo hoje |
| PoSearchAi | Fora de escopo | Alto (infra) | Depende de produto |
| po-code-editor | Não substituir `AdvCodeEditor` | Baixo p/ embutir, redundante | Baixo incremental |

**Decisão do operador (2026-11-23):** os três vereditos foram
confirmados. Registrado em `docs/COMPONENT_STATUS.md` (seção "fora de
escopo, com motivo").
