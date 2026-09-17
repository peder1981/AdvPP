package llm

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Layer contém os pesos de uma camada do transformer llama-padrão (sem as
// normas extras "SubLN" do bitnet-b1.58 — este modelo não as tem). O
// formato de cada peso (I2_S ternário ou F16 denso) é decidido por
// LoadWeight a partir do tipo GGML real do tensor, não fixado aqui.
type Layer struct {
	AttnNorm          []float32
	Wq, Wk, Wv, Wo    LayerWeights
	FFNNorm           []float32
	Wgate, Wup, Wdown LayerWeights
}

// Model contém os hiperparâmetros e pesos carregados de um GGUF de
// arquitetura "llama" (Falcon3-3B-1.58bit, pesos I2_S) ou "minicpm"
// (MiniCPM-2B, pesos F16 — ver LoadWeight). ScaleEmbed/ScaleResidual/
// ScaleLogit são os três multiplicadores muP específicos do MiniCPM;
// ficam em 1.0 (no-op) para "llama", preservando o forward pass do
// Falcon3 byte-a-byte.
type Model struct {
	g *File

	Arch      string
	NLayer    int
	NEmbd     int
	NHead     int
	NHeadKV   int
	HeadDim   int
	NFF       int
	RopeDims  int
	FreqBase  float32
	RMSEps    float32
	VocabSize int

	// ScaleEmbed escala o embedding do token logo após a leitura da
	// tabela; ScaleResidual escala a contribuição de cada camada (atenção
	// e FFN) antes de somar ao residual; ScaleLogit escala o estado
	// oculto final antes da projeção de logits. Nomes de chave GGUF e
	// direção exata (multiplicar vs. dividir) marcados como PENDENTE DE
	// VERIFICAÇÃO abaixo, em LoadModel — não confirmados contra um
	// arquivo MiniCPM real nem contra o código-fonte do llama.cpp nesta
	// sessão.
	ScaleEmbed    float32
	ScaleResidual float32
	ScaleLogit    float32

	Layers     []Layer
	OutputNorm []float32
	Output     LayerWeights // output.weight — F16 (Falcon3) ou Q6_K (MiniCPM Q4_K_M)

	tokEmbdName string
}

// LoadModel abre um GGUF e carrega todos os pesos em memória (~ tamanho do
// arquivo). ponytail: sem streaming/mmap por camada por enquanto — revisar
// se o consumo de RAM for um problema para modelos maiores que este 3B.
func LoadModel(path string) (*Model, error) {
	g, err := Open(path)
	if err != nil {
		return nil, err
	}

	arch, _ := g.String("general.architecture")
	if arch != "llama" && arch != "minicpm" {
		g.Close()
		return nil, fmt.Errorf("llm: arquitetura %q não suportada (só \"llama\" ou \"minicpm\" por enquanto)", arch)
	}

	m := &Model{g: g, Arch: arch, tokEmbdName: "token_embd.weight"}
	// Chaves GGUF são prefixadas pelo nome da arquitetura por convenção do
	// próprio formato (ex.: "llama.block_count", "minicpm.block_count") —
	// generalizado aqui em vez de fixo em "llama." para servir os dois.
	prefix := arch + "."
	nLayer, _ := g.Uint32(prefix + "block_count")
	nEmbd, _ := g.Uint32(prefix + "embedding_length")
	nHead, _ := g.Uint32(prefix + "attention.head_count")
	nHeadKV, _ := g.Uint32(prefix + "attention.head_count_kv")
	nFF, _ := g.Uint32(prefix + "feed_forward_length")
	ropeDims, _ := g.Uint32(prefix + "rope.dimension_count")
	freqBase, _ := g.Float32(prefix + "rope.freq_base")
	rmsEps, _ := g.Float32(prefix + "attention.layer_norm_rms_epsilon")
	vocabSize, _ := g.Uint32(prefix + "vocab_size")

	// Escalas muP do MiniCPM — PENDENTE DE VERIFICAÇÃO: nomes de chave e
	// direção (multiplicar/dividir) não confirmados contra um arquivo
	// MiniCPM real nem contra o código-fonte do llama.cpp nesta sessão.
	// Ausentes (ok==false) => 1.0 (no-op), preservando o forward pass do
	// "llama"/Falcon3 exatamente como era antes desta mudança.
	m.ScaleEmbed = 1.0
	m.ScaleResidual = 1.0
	m.ScaleLogit = 1.0
	if arch == "minicpm" {
		if v, ok := g.Float32(prefix + "embedding_scale"); ok {
			m.ScaleEmbed = v
		}
		if v, ok := g.Float32(prefix + "residual_scale"); ok {
			m.ScaleResidual = v
		}
		if v, ok := g.Float32(prefix + "logit_scale"); ok {
			m.ScaleLogit = v
		}
	}

	m.NLayer = int(nLayer)
	m.NEmbd = int(nEmbd)
	m.NHead = int(nHead)
	m.NHeadKV = int(nHeadKV)
	if m.NHead == 0 {
		g.Close()
		return nil, fmt.Errorf("llm: attention.head_count ausente ou zero")
	}
	m.HeadDim = m.NEmbd / m.NHead
	m.NFF = int(nFF)
	m.RopeDims = int(ropeDims)
	m.FreqBase = freqBase
	m.RMSEps = rmsEps
	m.VocabSize = int(vocabSize)

	loadNorm := func(name string) ([]float32, error) {
		raw, err := g.TensorData(name)
		if err != nil {
			return nil, err
		}
		out := make([]float32, len(raw)/4)
		for i := range out {
			out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
		}
		return out, nil
	}

	m.Layers = make([]Layer, m.NLayer)
	for il := 0; il < m.NLayer; il++ {
		var l Layer
		var err error
		if l.AttnNorm, err = loadNorm(fmt.Sprintf("blk.%d.attn_norm.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		if l.FFNNorm, err = loadNorm(fmt.Sprintf("blk.%d.ffn_norm.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		if l.Wq, err = LoadWeight(g, fmt.Sprintf("blk.%d.attn_q.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		if l.Wk, err = LoadWeight(g, fmt.Sprintf("blk.%d.attn_k.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		if l.Wv, err = LoadWeight(g, fmt.Sprintf("blk.%d.attn_v.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		if l.Wo, err = LoadWeight(g, fmt.Sprintf("blk.%d.attn_output.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		if l.Wgate, err = LoadWeight(g, fmt.Sprintf("blk.%d.ffn_gate.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		if l.Wup, err = LoadWeight(g, fmt.Sprintf("blk.%d.ffn_up.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		if l.Wdown, err = LoadWeight(g, fmt.Sprintf("blk.%d.ffn_down.weight", il)); err != nil {
			g.Close()
			return nil, err
		}
		m.Layers[il] = l
	}

	if m.OutputNorm, err = loadNorm("output_norm.weight"); err != nil {
		g.Close()
		return nil, err
	}
	if m.Output, err = LoadWeight(g, "output.weight"); err != nil {
		g.Close()
		return nil, err
	}

	return m, nil
}

func (m *Model) Close() error { return m.g.Close() }

// Context é uma sessão de geração: mantém o KV cache de uma sequência única
// (sem batching, sem múltiplas sequências simultâneas).
type Context struct {
	m      *Model
	kCache [][][]float32 // [layer][posição][NHeadKV*HeadDim]
	vCache [][][]float32
	pos    int
}

// NewContext cria uma sessão de geração vazia para o modelo.
func NewContext(m *Model) *Context {
	return &Context{
		m:      m,
		kCache: make([][][]float32, m.NLayer),
		vCache: make([][][]float32, m.NLayer),
	}
}

// Forward roda um passo de inferência para o próximo token da sequência,
// atualizando o KV cache e retornando os logits (tamanho VocabSize).
func (c *Context) Forward(token int32) ([]float32, error) {
	m := c.m
	x, err := EmbedRowGeneric(m.g, m.tokEmbdName, int(token), m.NEmbd)
	if err != nil {
		return nil, err
	}
	scaleInPlace(x, m.ScaleEmbed)

	groupSize := m.NHead / m.NHeadKV

	for il, layer := range m.Layers {
		inpSA := append([]float32(nil), x...)

		cur := RMSNorm(x, layer.AttnNorm, m.RMSEps)
		q := layer.Wq.MatMul(cur)
		k := layer.Wk.MatMul(cur)
		v := layer.Wv.MatMul(cur)

		for h := 0; h < m.NHead; h++ {
			RoPE(q[h*m.HeadDim:(h+1)*m.HeadDim], m.HeadDim, c.pos, m.RopeDims, m.FreqBase)
		}
		for h := 0; h < m.NHeadKV; h++ {
			RoPE(k[h*m.HeadDim:(h+1)*m.HeadDim], m.HeadDim, c.pos, m.RopeDims, m.FreqBase)
		}

		c.kCache[il] = append(c.kCache[il], k)
		c.vCache[il] = append(c.vCache[il], v)

		attnOut := attention(m, il, c, q, groupSize)
		o := layer.Wo.MatMul(attnOut)
		scaleInPlace(o, m.ScaleResidual)
		AddInPlace(o, inpSA)
		x = o

		inpFF := append([]float32(nil), x...)
		cur = RMSNorm(x, layer.FFNNorm, m.RMSEps)
		gate := layer.Wgate.MatMul(cur)
		up := layer.Wup.MatMul(cur)
		SwiGLU(gate, up)
		down := layer.Wdown.MatMul(gate)
		scaleInPlace(down, m.ScaleResidual)
		AddInPlace(down, inpFF)
		x = down
	}

	xNorm := RMSNorm(x, m.OutputNorm, m.RMSEps)
	scaleInPlace(xNorm, m.ScaleLogit)
	logits := m.Output.MatMul(xNorm)

	c.pos++
	return logits, nil
}

// scaleInPlace multiplica x por s. Usado pelos três multiplicadores muP do
// MiniCPM (ScaleEmbed/ScaleResidual/ScaleLogit); s==1.0 é um no-op exato em
// ponto flutuante (sem perda de precisão), então o forward pass de um
// modelo "llama" (Falcon3) sai byte-a-byte idêntico ao de antes desta
// mudança.
func scaleInPlace(x []float32, s float32) {
	if s == 1.0 {
		return
	}
	for i := range x {
		x[i] *= s
	}
}

// attention calcula a atenção causal GQA para todas as posições já
// cacheadas (incluindo a que acabou de ser inserida em c.kCache/vCache).
// Paralelizado por cabeça: cada uma lê q/k/v e escreve em uma faixa
// disjunta de `out`, então não há necessidade de sincronização entre elas.
func attention(m *Model, il int, c *Context, q []float32, groupSize int) []float32 {
	nPos := len(c.kCache[il])
	scale := float32(1.0 / math.Sqrt(float64(m.HeadDim)))
	out := make([]float32, m.NEmbd)

	parallelRows(m.NHead, func(h0, h1 int) {
		scores := make([]float32, nPos)
		for h := h0; h < h1; h++ {
			kvHead := h / groupSize
			qh := q[h*m.HeadDim : (h+1)*m.HeadDim]

			for t := 0; t < nPos; t++ {
				kt := c.kCache[il][t][kvHead*m.HeadDim : (kvHead+1)*m.HeadDim]
				var dot float32
				for i := 0; i < m.HeadDim; i++ {
					dot += qh[i] * kt[i]
				}
				scores[t] = dot * scale
			}
			Softmax(scores)

			oh := out[h*m.HeadDim : (h+1)*m.HeadDim]
			for t := 0; t < nPos; t++ {
				vt := c.vCache[il][t][kvHead*m.HeadDim : (kvHead+1)*m.HeadDim]
				w := scores[t]
				for i := 0; i < m.HeadDim; i++ {
					oh[i] += w * vt[i]
				}
			}
		}
	})
	return out
}
