package llm

import "fmt"

// LayerWeights abstrai o formato de armazenamento de um tensor de peso de
// camada (I2_S ternário ou F16 denso) atrás de uma única operação de
// matmul, para que Forward não precise saber qual formato o modelo usa.
type LayerWeights interface {
	MatMul(x []float32) []float32
}

// MatMul implementa LayerWeights para I2SWeight (BitNet/Falcon3-1.58bit) —
// mesmo caminho já usado por Forward antes desta abstração existir.
func (w *I2SWeight) MatMul(x []float32) []float32 {
	return MatMulI2S(w, x)
}

// F16Weight é um tensor de peso denso em F16 (llama.cpp "F16"), usado por
// modelos cujas camadas MatMulI2S não sabe ler (ex.: MiniCPM-2B). MatMul
// aqui só generaliza MatMulF16 (já validado, usado antes só pela projeção
// de saída do Falcon3) pra qualquer camada, sem kernel novo.
type F16Weight struct {
	Data  []byte
	NRows int
	NCols int
}

func (w *F16Weight) MatMul(x []float32) []float32 {
	return MatMulF16(w.Data, w.NRows, w.NCols, x)
}

// LoadWeight lê um tensor de peso de camada em qualquer formato suportado
// (I2_S ou F16), decidido pelo tipo GGML real do tensor gravado no arquivo
// — não pela arquitetura do modelo, para que uma combinação ainda não
// prevista (ex.: um "llama" futuro em F16) funcione sem mudança aqui.
func LoadWeight(g *File, name string) (LayerWeights, error) {
	t, ok := g.Tensor(name)
	if !ok {
		return nil, fmt.Errorf("llm: tensor %q não encontrado", name)
	}
	switch t.Type {
	case GGMLTypeI2S:
		return LoadI2SWeight(g, name)
	case GGMLTypeF16:
		return loadF16Weight(g, t, name)
	case GGMLTypeQ4_K:
		return LoadQ4KWeight(g, name)
	case GGMLTypeQ6_K:
		return LoadQ6KWeight(g, name)
	default:
		return nil, fmt.Errorf("llm: tensor %q em formato %v não suportado (só I2_S, F16, Q4_K e Q6_K por enquanto)", name, t.Type)
	}
}

func loadF16Weight(g *File, t *Tensor, name string) (*F16Weight, error) {
	if len(t.Shape) != 2 {
		return nil, fmt.Errorf("llm: tensor %q tem %d dimensões, esperado 2", name, len(t.Shape))
	}
	nCols := int(t.Shape[0])
	nRows := int(t.Shape[1])
	raw, err := g.TensorData(name)
	if err != nil {
		return nil, err
	}
	if len(raw) < nCols*nRows*2 {
		return nil, fmt.Errorf("llm: tensor %q menor que o esperado (%d bytes)", name, len(raw))
	}
	return &F16Weight{Data: raw, NRows: nRows, NCols: nCols}, nil
}
