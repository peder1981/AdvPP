package vm

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/advpl/compiler/pkg/llm"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// llmState é o estado Go da classe LLM (campo Native do objeto): o modelo
// carregado, o tokenizer e o contexto de geração (KV cache) de uma sessão.
type llmState struct {
	model *llm.Model
	tok   *llm.Tokenizer
	ctx   *llm.Context
	// suprime ativa a supressão de pensamento (técnicas do PiG em
	// inferência crua: fechar "<think>" no prefill + remover blocos da
	// saída). Default desligado: preserva o comportamento atual.
	suprime bool
	// comBOS prefixa o BOS do tokenizer ao prefill (instrutivos como Qwen3
	// degradam sem ele; MiniCPM/Falcon3 validados funcionam sem — default
	// desligado para não mudar comportamento validado).
	comBOS bool
}

func newLLMObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("LLM", nil)
	obj.Native = &llmState{}
	return obj
}

// callLLMMethod implementa a classe nativa LLM (pkg/llm): carrega um GGUF
// I2_S (BitNet/Falcon3-1.58bit) e gera texto, sem CGO nem dependências
// externas — o mesmo motor validado em pkg/llm.
func (v *VM) callLLMMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*llmState)
	if !ok {
		return fmt.Errorf("LLM: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		path := advplrt.ToString(getArg(args, 0))
		g, err := llm.Open(path)
		if err != nil {
			return fmt.Errorf("LLM:New: %w", err)
		}
		tok, err := llm.NewTokenizer(g)
		g.Close()
		if err != nil {
			return fmt.Errorf("LLM:New: %w", err)
		}
		model, err := llm.LoadModel(path)
		if err != nil {
			return fmt.Errorf("LLM:New: %w", err)
		}
		st.model = model
		st.tok = tok
		st.ctx = llm.NewContext(model)
		v.push(obj)

	case "TOKENIZE":
		if st.tok == nil {
			return fmt.Errorf("LLM:Tokenize: chame New() primeiro")
		}
		ids := st.tok.Encode(advplrt.ToString(getArg(args, 0)))
		elems := make([]advplrt.Value, len(ids))
		for i, id := range ids {
			elems[i] = advplrt.NewNumber(float64(id))
		}
		v.push(advplrt.NewArray(elems))

	case "DECODE":
		if st.tok == nil {
			return fmt.Errorf("LLM:Decode: chame New() primeiro")
		}
		arr, ok := getArg(args, 0).(*advplrt.ArrayValue)
		if !ok {
			return fmt.Errorf("LLM:Decode: esperado array de token ids")
		}
		ids := make([]int32, len(arr.Elements))
		for i, e := range arr.Elements {
			ids[i] = int32(advplrt.ToFloat(e))
		}
		v.push(advplrt.NewString(st.tok.Decode(ids)))

	case "GENERATE":
		text, err := st.generate(args)
		if err != nil {
			return fmt.Errorf("LLM:Generate: %w", err)
		}
		v.push(advplrt.NewString(text))

	case "COMBOS":
		// ComBOS([lAtivo]): prefixa o BOS do tokenizer ao prefill.
		// Sem argumento, liga.
		if len(args) == 0 {
			st.comBOS = true
		} else {
			st.comBOS = advplrt.ToBool(getArg(args, 0))
		}
		v.push(advplrt.NewString("combos=" + boolStr(st.comBOS)))

	case "SUPRIMEPENSAMENTO":
		// SuprimePensamento([lAtivo]): liga/desliga a supressão de blocos
		// "<think>". Sem argumento, liga.
		if len(args) == 0 {
			st.suprime = true
		} else {
			st.suprime = advplrt.ToBool(getArg(args, 0))
		}
		v.push(advplrt.NewString("suprime=" + boolStr(st.suprime)))

	case "CLOSE":
		if st.model != nil {
			st.model.Close()
			st.model, st.tok, st.ctx = nil, nil, nil
		}
		v.push(advplrt.Nil)

	default:
		return fmt.Errorf("LLM: método desconhecido %q", method)
	}
	return nil
}

// llmTimeoutSeconds resolve o teto de geração: ADVPP_LLM_TIMEOUT_SECS
// (segundos; 0 = sem teto). Sem a variável, vale o padrão moderno de 30
// minutos — a era 2.0.3 documentava 5 minutos que nunca foram implementados
// (não havia nenhum watchdog no caminho de geração).
func llmTimeoutSeconds() int64 {
	const modernDefault = 30 * 60
	s := os.Getenv("ADVPP_LLM_TIMEOUT_SECS")
	if s == "" {
		return modernDefault
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return modernDefault
	}
	return n
}

// generate roda o prompt pela VM do transformer e amostra até nMaxTokens
// novos tokens (padrão: 64, greedy). Args: cPrompt, [nMaxTokens], [nTemp].
// Respeita o teto de llmTimeoutSeconds (erro capturável via Try/Catch;
// parcial descartada para não entregar texto cortado como se completo fosse).
func (st *llmState) generate(args []advplrt.Value) (string, error) {
	if st.model == nil {
		return "", fmt.Errorf("chame New() primeiro")
	}
	prompt := advplrt.ToString(getArg(args, 0))
	if st.suprime {
		prompt = llm.CloseThinkingPrefill(prompt)
	}
	prefill := st.tok.Encode(prompt)
	if st.comBOS && st.tok.BOS() != 0 {
		prefill = append([]int32{st.tok.BOS()}, prefill...)
	}

	maxTokens := 64
	if len(args) > 1 {
		maxTokens = int(advplrt.ToFloat(args[1]))
	}
	var temp float32
	if len(args) > 2 {
		temp = float32(advplrt.ToFloat(args[2]))
	}

	limitSecs := llmTimeoutSeconds()
	var deadline time.Time
	if limitSecs > 0 {
		deadline = time.Now().Add(time.Duration(limitSecs) * time.Second)
	}
	expired := func() bool {
		return limitSecs > 0 && time.Now().After(deadline)
	}

	start := time.Now()
	var logits []float32
	var err error
	for _, id := range prefill {
		if expired() {
			return "", fmt.Errorf("timeout LLM após %ds no prefill (%d tokens gerados)", int64(time.Since(start).Seconds()), 0)
		}
		if logits, err = st.ctx.Forward(id); err != nil {
			return "", err
		}
	}

	rng := rand.New(rand.NewSource(1))
	var generated []int32
	for i := 0; i < maxTokens; i++ {
		if expired() {
			return "", fmt.Errorf("timeout LLM após %ds com %d/%d tokens", int64(time.Since(start).Seconds()), len(generated), maxTokens)
		}
		next := llm.Sample(logits, llm.SamplerConfig{Temperature: temp}, rng)
		if next == st.tok.EOS() {
			break
		}
		generated = append(generated, next)
		if logits, err = st.ctx.Forward(next); err != nil {
			return "", err
		}
	}
	text := st.tok.Decode(generated)
	if st.suprime {
		text = llm.StripThinking(text)
	}
	return text, nil
}

// boolStr formata lógico para mensagens (".T." canonico do AdvPL).
func boolStr(b bool) string {
	if b {
		return ".T."
	}
	return ".F."
}
