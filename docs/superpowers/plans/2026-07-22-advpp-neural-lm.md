# LM neural char-level (S4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: subagent-driven-development ou executing-plans.

**Goal:** Montar e treinar um LM neural char-level 100% em AdvPP (`tests/llm/pt_neural.prw`), sobre o stack S2+S3, adicionando a única op de motor que falta: `Reshape` diferenciável.

**Architecture:** Parte 1 = `Variable.Reshape` no autograd + método VM. Parte 2 = NPLM char-level em AdvPL: Embedding→Reshape→Linear→Tanh→Linear→SoftmaxCE, treino Adam+Fit, geração por amostragem.

**Tech Stack:** Go (`pkg/autograd`), a VM, AdvPL puro.

## Global Constraints

- `go test ./...` verde ao fim de cada task; S2/S3 e `pt_llm`/`pt_chat`/`pt_nn` sem regressão (`advplc check`).
- Não modificar `pkg/tensor`. Reusar `pkg/nn` (Linear/Embedding), `pkg/autograd`, `Fit`.
- Índices AdvPL 1-based → 0-based interno (já tratado em Embedding/SoftmaxCE).
- Erros na VM → `verr`/`advplrt.NewError` (capturável).
- Rebuild: `go build -o advplc ./cmd/advplc`.

---

## Task 1: `Variable.Reshape` diferenciável (motor + VM)

**Files:** Modify `pkg/autograd/ops.go`, `pkg/autograd/autograd_test.go`, `pkg/vm/autograd_native.go`; Create `tests/reshape_test.prw`.

**Interfaces:** Produces `func (a *Variable) Reshape(shape []int) (*Variable, error)`; método VM `oV:Reshape(aShape)` → Variable. Consumes `tensor.Reshape` (ops.go:218), `addGrad`.

- [ ] **Step 1: grad-check em `pkg/autograd/autograd_test.go`** — adicionar um teste que faz reshape `[2,3]→[3,2]`, encadeia com `Sum`, `Backward`, e confere via diferenças finitas que o grad da entrada é todo 1 (dSum/dx=1) e tem a forma `[2,3]`. Usar o helper `gradCheck`/`close32` já existentes.

- [ ] **Step 2: rodar → falha** (`Reshape` não existe): `go test ./pkg/autograd`.

- [ ] **Step 3: implementar em `pkg/autograd/ops.go`** (mesmo padrão de `Sum`):
```go
// Reshape muda a forma preservando os dados (e a ordem). Backward reshapa o grad
// de volta à forma original.
func (a *Variable) Reshape(shape []int) (*Variable, error) {
	y, err := a.Value.Reshape(shape)
	if err != nil {
		return nil, err
	}
	out := &Variable{Value: y, parents: []*Variable{a}}
	out.backward = func() {
		dg, err := out.Grad.Reshape(a.Value.Shape)
		if err != nil {
			return
		}
		addGrad(a, dg)
	}
	return out, nil
}
```

- [ ] **Step 4: método VM em `pkg/vm/autograd_native.go`** (dentro de `callVariableMethod`, junto de INDEXROWS):
```go
	case "RESHAPE":
		shp := shapeFromArg(getArg(args, 0))
		r, err := self.Reshape(shp)
		if err != nil {
			return verr(err)
		}
		v.push(wrapVariable(r))
```

- [ ] **Step 5: `go test ./pkg/autograd` → PASS**; `go build -o advplc ./cmd/advplc`.

- [ ] **Step 6: fixture `tests/reshape_test.prw`** — `Variable:FromArray({1,2,3,4,5,6},{2,3})`, `:Reshape({3,2})`, checa `:Value():Shape()` = {3,2}; `:Sum():Backward()` e `:Grad():Shape()` volta {2,3}; reshape inválido `{4,4}` dentro de `Begin Sequence/Recover` é capturado. Rodar: `./advplc run tests/reshape_test.prw` → `OK`.

- [ ] **Step 7: Commit** `git add pkg/autograd/ops.go pkg/autograd/autograd_test.go pkg/vm/autograd_native.go tests/reshape_test.prw && git commit -m "autograd: differentiable Reshape + VM method"`

---

## Task 2: `pt_neural.prw` — tokenizador + exemplos + forward

**Files:** Create `tests/llm/pt_neural.prw`.

**Interfaces:** Consumes `Embedding`, `Linear`, `Variable`, `Reshape`, `SoftmaxCE`, `MemoRead`, `File`, `aSort`, `Asc`, `Chr`, `SubStr`.

- [ ] **Step 1: tokenizador** — `Static Function BuildVocab(cTexto)` devolve `{aId2Ch, oCh2Id}`: coleta chars distintos, `aSort`, monta array id→char (1-based) e um mapa char→id (usar um array paralelo ou JsonObject por code-point). `Static Function Encode(cTexto, oCh2Id)` → array de ids.

- [ ] **Step 2: montador de exemplos** — `Static Function BuildExamples(aIds, nK, nMax)` → `{aX, aAlvo, nN}`: janela deslizante; `aX` achatado (N·k, exemplo-maior), `aAlvo` (N). Amostra por stride se exceder `nMax`.

- [ ] **Step 3: forward** — `Static Function Forward(oEmb, oL1, oL2, aX, nN, nK, nD)` monta `oEmb:Forward(aX):Reshape({nN, nK*nD})`, `oL1:Forward(...):Tanh()`, `oL2:Forward(...)` → devolve logits Variable.

- [ ] **Step 4: auto-teste de forma** — no `User Function PtNeural()`, montar mini-modelo (V pequeno, k=2, D=4, H=8), rodar Forward num mini-batch e conferir `oLog:Value():Shape()` = {nN, V}. Rodar `./advplc run tests/llm/pt_neural.prw` (parcial, só forma). Ainda sem treino/geração.

- [ ] **Step 5: Commit** `git add tests/llm/pt_neural.prw && git commit -m "pt_neural: tokenizer + example builder + forward"`

---

## Task 3: treino + geração + auto-teste

**Files:** Modify `tests/llm/pt_neural.prw`.

- [ ] **Step 1: treino** — `Static Function Treina(oEmb,oL1,oL2,aX,aAlvo,nN,nK,nD,nLR,nEpocas)`: params = concat(`oEmb:Params()`,`oL1:Params()`,`oL2:Params()`); `oOpt := Adam():New(aParams, nLR)`; `nInicial := LossOnly(...)`; `nFinal := Fit({|| Passo(...) }, nEpocas)`; devolve `{nInicial, nFinal}`. `Passo` faz ZeroGrad+forward+SoftmaxCE+Backward+Step, devolve loss escalar.

- [ ] **Step 2: geração** — `Static Function Gera(oEmb,oL1,oL2,aId2Ch,oCh2Id,cSeed,nK,nD,nGerar,nTemp)`: mantém janela de k ids (padding no início), a cada passo forward N=1 → `aLogits := oLog:Value():ToArray()`, aplica temperatura, softmax + amostragem cumulativa (`Random(1000000)/1000000`) em AdvPL, anexa char, desliza a janela. Devolve string.

- [ ] **Step 3: auto-teste determinístico** — `PtNeural()`: mini-corpus embutido repetitivo (ex.: `"o gato subiu no telhado. "` repetido), k=3, D=8, H=16, ~80 épocas, lr 0.05. Verifica `nFinal < nInicial*0.5` e `Len(cGerado) > 0`. Imprime `loss X -> Y` e `OK: pt_neural treinou e gerou.`

- [ ] **Step 4: rodar** `./advplc run tests/llm/pt_neural.prw` → `OK: ...` (rodar 2–3x p/ robustez ao init). Se loss não cair, PARE e reporte — bug real, não mascarar.

- [ ] **Step 5: Commit** `git commit -am "pt_neural: training + generation + self-test"`

---

## Task 4: corpus real + mover corpus + docs

**Files:** Move `corpus.txt`→`tests/llm/corpus.txt`; Modify `tests/llm/pt_nn.prw`, `tests/llm/pt_neural.prw`, `README.md`, `CHANGELOG.md`.

- [ ] **Step 1: mover corpus** `git mv corpus.txt tests/llm/corpus.txt`.

- [ ] **Step 2: `pt_neural` usa corpus real** — em `PtNeural()`, `If File("tests/llm/corpus.txt")` → `MemoRead` e treina numa amostra (nMax ~2000–3000 janelas, k=6, D=24, H=96, épocas ~150) e imprime um trecho gerado de um seed PT-BR; senão o mini-corpus do auto-teste. Manter o auto-teste (mini) como caminho default determinístico; corpus real atrás do `File()`.

- [ ] **Step 3: ajustar `pt_nn.prw`** — trocar `If File("corpus.txt")`/`MemoRead("corpus.txt")` por procurar `tests/llm/corpus.txt` primeiro, com `corpus.txt` como fallback. Rodar `./advplc run tests/llm/pt_nn.prw 2>&1 | tail -1` → sem regressão.

- [ ] **Step 4: rodar `pt_neural` no corpus real** — `./advplc run tests/llm/pt_neural.prw` — confere loss caindo e um trecho gerado (verificação visual).

- [ ] **Step 5: docs** — README: adicionar `tests/llm/pt_neural.prw` à tabela de modelos (o único **neural treinado por gradiente**) + curta seção. CHANGELOG `[Não lançado]`: entrada do S4 (op Reshape + LM neural char-level).

- [ ] **Step 6: regressão + commit** — `go test ./...` verde; `advplc check tests/llm/*.prw`; `git add -A && git commit -m "pt_neural: real corpus run + move corpus + docs"`.

---

## Verificação final
- `go test ./...` verde; grad-check de Reshape passa.
- `tests/reshape_test.prw` e `tests/llm/pt_neural.prw` imprimem `OK`.
- `pt_llm`/`pt_chat`/`pt_nn` sem regressão. Corpus em `tests/llm/`.
- Os 5 critérios de aceite da spec satisfeitos.
