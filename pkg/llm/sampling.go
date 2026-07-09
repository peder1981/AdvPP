package llm

import (
	"math/rand"
	"sort"
)

// SamplerConfig controla a amostragem de tokens a partir dos logits.
// Temperature <= 0 força amostragem gulosa (argmax), ignorando TopK/TopP.
type SamplerConfig struct {
	Temperature float32
	TopK        int
	TopP        float32
}

// Greedy retorna o índice do maior logit.
func Greedy(logits []float32) int32 {
	best := logits[0]
	bestI := 0
	for i, v := range logits {
		if v > best {
			best = v
			bestI = i
		}
	}
	return int32(bestI)
}

// Sample escolhe o próximo token segundo temperatura + top-k + top-p
// (nucleus sampling). Com Temperature<=0, é equivalente a Greedy.
func Sample(logits []float32, cfg SamplerConfig, rng *rand.Rand) int32 {
	if cfg.Temperature <= 0 {
		return Greedy(logits)
	}

	probs := make([]float32, len(logits))
	for i, v := range logits {
		probs[i] = v / cfg.Temperature
	}
	Softmax(probs)

	type candidate struct {
		id int
		p  float32
	}
	cand := make([]candidate, len(probs))
	for i, p := range probs {
		cand[i] = candidate{i, p}
	}
	sort.Slice(cand, func(a, b int) bool { return cand[a].p > cand[b].p })

	if cfg.TopK > 0 && cfg.TopK < len(cand) {
		cand = cand[:cfg.TopK]
	}
	if cfg.TopP > 0 && cfg.TopP < 1 {
		var cum float32
		cut := len(cand)
		for i, c := range cand {
			cum += c.p
			if cum >= cfg.TopP {
				cut = i + 1
				break
			}
		}
		cand = cand[:cut]
	}

	var sum float32
	for _, c := range cand {
		sum += c.p
	}
	r := rng.Float32() * sum
	var acc float32
	for _, c := range cand {
		acc += c.p
		if r <= acc {
			return int32(c.id)
		}
	}
	return int32(cand[len(cand)-1].id)
}
