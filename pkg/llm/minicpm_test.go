package llm

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"testing"
)

// --- construção de um GGUF sintético mínimo, arquitetura "minicpm" ---
//
// Não há um arquivo MiniCPM-2B real disponível nesta sessão para testar
// contra ele (ao contrário do Falcon3, ver falcon3Path em gguf_test.go).
// Este helper escreve um arquivo GGUF válido, porém minúsculo (1 camada,
// embedding_length=8), byte a byte no mesmo formato que Open() lê — para
// exercitar LoadModel/Forward de ponta a ponta no caminho F16 novo sem
// depender de um download de ~4-5GB.
const (
	tinyNEmbd     = 8
	tinyNHead     = 2
	tinyNHeadKV   = 2
	tinyHeadDim   = tinyNEmbd / tinyNHead
	tinyNFF       = 8
	tinyVocabSize = 4
	tinyNLayer    = 1
)

type ggufKV struct {
	key string
	vt  ggufValueType
	val any
}

func kvU32(key string, v uint32) ggufKV  { return ggufKV{key, gvtUint32, v} }
func kvF32(key string, v float32) ggufKV { return ggufKV{key, gvtFloat32, v} }
func kvStr(key string, v string) ggufKV  { return ggufKV{key, gvtString, v} }

func writeGGUFValue(buf *bytes.Buffer, vt ggufValueType, val any) {
	binary.Write(buf, binary.LittleEndian, uint32(vt))
	switch vt {
	case gvtUint32:
		binary.Write(buf, binary.LittleEndian, val.(uint32))
	case gvtFloat32:
		binary.Write(buf, binary.LittleEndian, val.(float32))
	case gvtString:
		s := val.(string)
		binary.Write(buf, binary.LittleEndian, uint64(len(s)))
		buf.WriteString(s)
	default:
		panic("writeGGUFValue: tipo não implementado neste helper de teste")
	}
}

// f16Bytes converte um float32 pra 2 bytes F16 little-endian (round-trip
// exato para os pequenos inteiros/half-passos usados nos pesos sintéticos
// deste teste — não é um conversor F32->F16 de propósito geral).
func f16Bytes(v float32) []byte {
	bits := math.Float32bits(v)
	sign := uint16((bits >> 16) & 0x8000)
	exp := int32((bits>>23)&0xFF) - 127 + 15
	mant := uint16((bits >> 13) & 0x3FF)
	if exp <= 0 {
		return []byte{0, byte(sign >> 8)}
	}
	h := sign | uint16(exp<<10) | mant
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, h)
	return b
}

// tinyF16Tensor gera nRows*nCols valores F16 determinísticos e pequenos
// (evita overflow/instabilidade numérica no forward pass sintético).
func tinyF16Tensor(nRows, nCols int, base float32) []byte {
	buf := make([]byte, 0, nRows*nCols*2)
	for i := 0; i < nRows*nCols; i++ {
		v := base + float32(i%5)*0.01
		buf = append(buf, f16Bytes(v)...)
	}
	return buf
}

func tinyF32Vector(n int, v float32) []byte {
	buf := make([]byte, n*4)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// buildTinyMiniCPMGGUF escreve um arquivo GGUF sintético de arquitetura
// "minicpm" em um arquivo temporário e devolve seu caminho. scaleEmbed/
// scaleResidual/scaleLogit são gravados nas chaves minicpm.*_scale
// correspondentes (ver LoadModel) — omitir uma (valor 0) faz esta função
// não gravar a chave, testando o default 1.0 (no-op).
func buildTinyMiniCPMGGUF(t *testing.T, scaleEmbed, scaleResidual, scaleLogit float32) string {
	t.Helper()

	kvs := []ggufKV{
		kvStr("general.architecture", "minicpm"),
		kvU32("minicpm.block_count", tinyNLayer),
		kvU32("minicpm.embedding_length", tinyNEmbd),
		kvU32("minicpm.attention.head_count", tinyNHead),
		kvU32("minicpm.attention.head_count_kv", tinyNHeadKV),
		kvU32("minicpm.feed_forward_length", tinyNFF),
		kvU32("minicpm.rope.dimension_count", tinyHeadDim),
		kvF32("minicpm.rope.freq_base", 10000),
		kvF32("minicpm.attention.layer_norm_rms_epsilon", 1e-5),
		kvU32("minicpm.vocab_size", tinyVocabSize),
	}
	if scaleEmbed != 0 {
		kvs = append(kvs, kvF32("minicpm.embedding_scale", scaleEmbed))
	}
	if scaleResidual != 0 {
		kvs = append(kvs, kvF32("minicpm.residual_scale", scaleResidual))
	}
	if scaleLogit != 0 {
		kvs = append(kvs, kvF32("minicpm.logit_scale", scaleLogit))
	}

	type tensorSpec struct {
		name  string
		shape []uint64
		gtype GGMLType
		data  []byte
	}
	specs := []tensorSpec{
		{"token_embd.weight", []uint64{tinyNEmbd, tinyVocabSize}, GGMLTypeF16, tinyF16Tensor(tinyVocabSize, tinyNEmbd, 0.1)},
		{"blk.0.attn_norm.weight", []uint64{tinyNEmbd}, GGMLTypeF32, tinyF32Vector(tinyNEmbd, 1.0)},
		{"blk.0.attn_q.weight", []uint64{tinyNEmbd, tinyNHead * tinyHeadDim}, GGMLTypeF16, tinyF16Tensor(tinyNHead*tinyHeadDim, tinyNEmbd, 0.05)},
		{"blk.0.attn_k.weight", []uint64{tinyNEmbd, tinyNHeadKV * tinyHeadDim}, GGMLTypeF16, tinyF16Tensor(tinyNHeadKV*tinyHeadDim, tinyNEmbd, 0.05)},
		{"blk.0.attn_v.weight", []uint64{tinyNEmbd, tinyNHeadKV * tinyHeadDim}, GGMLTypeF16, tinyF16Tensor(tinyNHeadKV*tinyHeadDim, tinyNEmbd, 0.05)},
		{"blk.0.attn_output.weight", []uint64{tinyNEmbd, tinyNEmbd}, GGMLTypeF16, tinyF16Tensor(tinyNEmbd, tinyNEmbd, 0.05)},
		{"blk.0.ffn_norm.weight", []uint64{tinyNEmbd}, GGMLTypeF32, tinyF32Vector(tinyNEmbd, 1.0)},
		{"blk.0.ffn_gate.weight", []uint64{tinyNEmbd, tinyNFF}, GGMLTypeF16, tinyF16Tensor(tinyNFF, tinyNEmbd, 0.03)},
		{"blk.0.ffn_up.weight", []uint64{tinyNEmbd, tinyNFF}, GGMLTypeF16, tinyF16Tensor(tinyNFF, tinyNEmbd, 0.03)},
		{"blk.0.ffn_down.weight", []uint64{tinyNFF, tinyNEmbd}, GGMLTypeF16, tinyF16Tensor(tinyNEmbd, tinyNFF, 0.03)},
		{"output_norm.weight", []uint64{tinyNEmbd}, GGMLTypeF32, tinyF32Vector(tinyNEmbd, 1.0)},
		{"output.weight", []uint64{tinyNEmbd, tinyVocabSize}, GGMLTypeF16, tinyF16Tensor(tinyVocabSize, tinyNEmbd, 0.1)},
	}

	var header bytes.Buffer
	binary.Write(&header, binary.LittleEndian, uint32(ggufMagic))
	binary.Write(&header, binary.LittleEndian, uint32(3))
	binary.Write(&header, binary.LittleEndian, uint64(len(specs)))
	binary.Write(&header, binary.LittleEndian, uint64(len(kvs)))
	for _, kv := range kvs {
		binary.Write(&header, binary.LittleEndian, uint64(len(kv.key)))
		header.WriteString(kv.key)
		writeGGUFValue(&header, kv.vt, kv.val)
	}

	// Primeira passada: grava os cabeçalhos de tensor com offset relativo
	// ao início da seção de dados (sem padding entre tensores — Open() só
	// exige alinhamento no INÍCIO da seção, não entre tensores).
	var offset uint64
	tensorHeaders := make([]bytes.Buffer, len(specs))
	for i, s := range specs {
		var b bytes.Buffer
		binary.Write(&b, binary.LittleEndian, uint64(len(s.name)))
		b.WriteString(s.name)
		binary.Write(&b, binary.LittleEndian, uint32(len(s.shape)))
		for _, d := range s.shape {
			binary.Write(&b, binary.LittleEndian, d)
		}
		binary.Write(&b, binary.LittleEndian, uint32(s.gtype))
		binary.Write(&b, binary.LittleEndian, offset)
		tensorHeaders[i] = b
		offset += uint64(len(s.data))
	}

	var out bytes.Buffer
	out.Write(header.Bytes())
	for _, b := range tensorHeaders {
		out.Write(b.Bytes())
	}
	// alinhamento de 32 bytes antes da seção de dados, igual a Open().
	for out.Len()%32 != 0 {
		out.WriteByte(0)
	}
	for _, s := range specs {
		out.Write(s.data)
	}

	f, err := os.CreateTemp(t.TempDir(), "minicpm-tiny-*.gguf")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(out.Bytes()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return f.Name()
}

func TestLoadModelMiniCPMArchitecture(t *testing.T) {
	path := buildTinyMiniCPMGGUF(t, 2.0, 0.5, 1.5)
	m, err := LoadModel(path)
	if err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	defer m.Close()

	if m.Arch != "minicpm" {
		t.Errorf("Arch = %q, want minicpm", m.Arch)
	}
	if m.NLayer != tinyNLayer || m.NEmbd != tinyNEmbd || m.NHead != tinyNHead {
		t.Errorf("hyperparams = {%d,%d,%d}, want {%d,%d,%d}", m.NLayer, m.NEmbd, m.NHead, tinyNLayer, tinyNEmbd, tinyNHead)
	}
	if m.ScaleEmbed != 2.0 || m.ScaleResidual != 0.5 || m.ScaleLogit != 1.5 {
		t.Errorf("scales = {%v,%v,%v}, want {2.0,0.5,1.5}", m.ScaleEmbed, m.ScaleResidual, m.ScaleLogit)
	}

	if _, ok := m.Layers[0].Wq.(*F16Weight); !ok {
		t.Errorf("Layers[0].Wq = %T, want *F16Weight (minicpm usa pesos densos F16, não I2_S)", m.Layers[0].Wq)
	}
}

func TestLoadModelMiniCPMDefaultScalesAreNoop(t *testing.T) {
	path := buildTinyMiniCPMGGUF(t, 0, 0, 0) // nenhuma chave de escala gravada
	m, err := LoadModel(path)
	if err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	defer m.Close()

	if m.ScaleEmbed != 1.0 || m.ScaleResidual != 1.0 || m.ScaleLogit != 1.0 {
		t.Errorf("scales sem chave no GGUF = {%v,%v,%v}, want {1.0,1.0,1.0} (no-op)", m.ScaleEmbed, m.ScaleResidual, m.ScaleLogit)
	}
}

func TestForwardMiniCPMTinySyntheticModel(t *testing.T) {
	path := buildTinyMiniCPMGGUF(t, 1.1, 0.9, 1.0)
	m, err := LoadModel(path)
	if err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	defer m.Close()

	ctx := NewContext(m)
	logits, err := ctx.Forward(0)
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if len(logits) != tinyVocabSize {
		t.Fatalf("len(logits) = %d, want %d", len(logits), tinyVocabSize)
	}
	for i, v := range logits {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Errorf("logits[%d] = %v, want um número finito", i, v)
		}
	}

	// um segundo passo (usa o KV cache) deve continuar produzindo saída
	// válida e avançar a posição.
	if _, err := ctx.Forward(1); err != nil {
		t.Fatalf("Forward (segundo passo): %v", err)
	}
	if ctx.pos != 2 {
		t.Errorf("ctx.pos = %d, want 2", ctx.pos)
	}
}

// --- LoadWeight: dispatch por tipo real do tensor, usando o Falcon3 real
// (I2_S) já usado pelos outros testes deste pacote (gguf_test.go) para o
// caso I2_S, e o próprio "output.weight" dele (F16) para o caso F16 — sem
// necessidade de nenhum arquivo sintético adicional.

func TestLoadWeightDispatchesI2S(t *testing.T) {
	if _, err := os.Stat(falcon3Path); err != nil {
		t.Skipf("modelo de teste não disponível: %v", err)
	}
	g, err := Open(falcon3Path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer g.Close()

	w, err := LoadWeight(g, "blk.0.attn_q.weight")
	if err != nil {
		t.Fatalf("LoadWeight: %v", err)
	}
	if _, ok := w.(*I2SWeight); !ok {
		t.Errorf("LoadWeight(blk.0.attn_q.weight) = %T, want *I2SWeight", w)
	}
}

func TestLoadWeightDispatchesF16MatchesMatMulF16(t *testing.T) {
	if _, err := os.Stat(falcon3Path); err != nil {
		t.Skipf("modelo de teste não disponível: %v", err)
	}
	g, err := Open(falcon3Path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer g.Close()

	tensor, ok := g.Tensor("output.weight")
	if !ok {
		t.Fatal("output.weight não encontrado")
	}
	nCols := int(tensor.Shape[0])
	nRows := int(tensor.Shape[1])

	w, err := LoadWeight(g, "output.weight")
	if err != nil {
		t.Fatalf("LoadWeight: %v", err)
	}
	f16w, ok := w.(*F16Weight)
	if !ok {
		t.Fatalf("LoadWeight(output.weight) = %T, want *F16Weight", w)
	}
	if f16w.NRows != nRows || f16w.NCols != nCols {
		t.Errorf("F16Weight shape = {%d,%d}, want {%d,%d}", f16w.NRows, f16w.NCols, nRows, nCols)
	}

	x := make([]float32, nCols)
	for i := range x {
		x[i] = float32(i%7) * 0.01
	}

	got := f16w.MatMul(x)
	raw, err := g.TensorData("output.weight")
	if err != nil {
		t.Fatalf("TensorData: %v", err)
	}
	want := MatMulF16(raw, nRows, nCols, x)

	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, len(want)=%d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("MatMul()[%d] = %v, want %v (idêntico a MatMulF16 direto)", i, got[i], want[i])
			break
		}
	}
}
