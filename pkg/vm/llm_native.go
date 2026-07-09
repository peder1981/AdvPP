package vm

import (
	"fmt"
	"math/rand"

	"github.com/advpl/compiler/pkg/llm"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// llmState é o estado Go da classe LLM (campo Native do objeto): o modelo
// carregado, o tokenizer e o contexto de geração (KV cache) de uma sessão.
type llmState struct {
	model *llm.Model
	tok   *llm.Tokenizer
	ctx   *llm.Context
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

// generate roda o prompt pela VM do transformer e amostra até nMaxTokens
// novos tokens (padrão: 64, greedy). Args: cPrompt, [nMaxTokens], [nTemp].
func (st *llmState) generate(args []advplrt.Value) (string, error) {
	if st.model == nil {
		return "", fmt.Errorf("chame New() primeiro")
	}
	prompt := advplrt.ToString(getArg(args, 0))

	maxTokens := 64
	if len(args) > 1 {
		maxTokens = int(advplrt.ToFloat(args[1]))
	}
	var temp float32
	if len(args) > 2 {
		temp = float32(advplrt.ToFloat(args[2]))
	}

	var logits []float32
	var err error
	for _, id := range st.tok.Encode(prompt) {
		if logits, err = st.ctx.Forward(id); err != nil {
			return "", err
		}
	}

	rng := rand.New(rand.NewSource(1))
	var generated []int32
	for i := 0; i < maxTokens; i++ {
		next := llm.Sample(logits, llm.SamplerConfig{Temperature: temp}, rng)
		if next == st.tok.EOS() {
			break
		}
		generated = append(generated, next)
		if logits, err = st.ctx.Forward(next); err != nil {
			return "", err
		}
	}
	return st.tok.Decode(generated), nil
}
