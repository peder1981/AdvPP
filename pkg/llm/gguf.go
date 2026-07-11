// Package llm implementa um motor de inferência mínimo para modelos GGUF
// quantizados em I2_S (ternário, BitNet-style), em Go puro (sem CGO), para
// rodar embutido nos binários do AdvPP nas 3 plataformas suportadas.
package llm

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

const ggufMagic = 0x46554747 // "GGUF" little-endian

// GGMLType identifica o tipo de armazenamento de um tensor no arquivo GGUF.
type GGMLType uint32

const (
	GGMLTypeF32  GGMLType = 0
	GGMLTypeF16  GGMLType = 1
	GGMLTypeQ4_0 GGMLType = 2
	GGMLTypeQ4_1 GGMLType = 3
	GGMLTypeQ5_0 GGMLType = 6
	GGMLTypeQ5_1 GGMLType = 7
	GGMLTypeQ8_0 GGMLType = 8
	GGMLTypeQ8_1 GGMLType = 9
	GGMLTypeQ2_K GGMLType = 10
	GGMLTypeQ3_K GGMLType = 11
	GGMLTypeQ4_K GGMLType = 12
	GGMLTypeQ5_K GGMLType = 13
	GGMLTypeQ6_K GGMLType = 14
	GGMLTypeQ8_K GGMLType = 15
	GGMLTypeBF16 GGMLType = 30
	GGMLTypeI2S  GGMLType = 36
)

func (t GGMLType) String() string {
	if n, ok := ggmlTypeNames[t]; ok {
		return n
	}
	return fmt.Sprintf("type%d", t)
}

var ggmlTypeNames = map[GGMLType]string{
	GGMLTypeF32: "F32", GGMLTypeF16: "F16",
	GGMLTypeQ4_0: "Q4_0", GGMLTypeQ4_1: "Q4_1", GGMLTypeQ5_0: "Q5_0", GGMLTypeQ5_1: "Q5_1",
	GGMLTypeQ8_0: "Q8_0", GGMLTypeQ8_1: "Q8_1",
	GGMLTypeQ2_K: "Q2_K", GGMLTypeQ3_K: "Q3_K", GGMLTypeQ4_K: "Q4_K", GGMLTypeQ5_K: "Q5_K",
	GGMLTypeQ6_K: "Q6_K", GGMLTypeQ8_K: "Q8_K",
	GGMLTypeBF16: "BF16", GGMLTypeI2S: "I2_S",
}

// ggufValueType identifica o tipo de um valor de metadado GGUF.
type ggufValueType uint32

const (
	gvtUint8 ggufValueType = iota
	gvtInt8
	gvtUint16
	gvtInt16
	gvtUint32
	gvtInt32
	gvtFloat32
	gvtBool
	gvtString
	gvtArray
	gvtUint64
	gvtInt64
	gvtFloat64
)

// Tensor descreve um tensor presente no arquivo GGUF.
type Tensor struct {
	Name   string
	Shape  []uint64 // ne[0..n_dims-1], ne[0] varia mais rápido (ordem GGML)
	Type   GGMLType
	Offset uint64 // offset em bytes dentro da seção de dados, já alinhado
	Size   uint64 // tamanho em bytes dos dados do tensor
}

// NElements retorna o número total de elementos do tensor.
func (t Tensor) NElements() uint64 {
	n := uint64(1)
	for _, d := range t.Shape {
		n *= d
	}
	return n
}

// File é um arquivo GGUF aberto: metadados carregados em memória, dados de
// tensor lidos sob demanda via ReadAt (arquivos de 1-2GB não cabem no
// orçamento de simplicidade de um go:embed).
type File struct {
	Version      uint32
	KV           map[string]any
	Tensors      []Tensor
	tensorByName map[string]*Tensor

	f         *os.File
	dataStart int64
}

// Open lê o header, os metadados e a lista de tensores de um arquivo GGUF.
// Os dados dos tensores permanecem em disco e são lidos por TensorData.
func Open(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	r := &reader{br: bufio.NewReaderSize(f, 64*1024)}

	var magic uint32
	r.read(&magic)
	if r.err == nil && magic != ggufMagic {
		f.Close()
		return nil, fmt.Errorf("gguf: magic inválido em %s", path)
	}

	g := &File{f: f, tensorByName: map[string]*Tensor{}}
	r.read(&g.Version)

	var nTensors, nKV uint64
	r.read(&nTensors)
	r.read(&nKV)

	g.KV = make(map[string]any, nKV)
	for i := uint64(0); i < nKV && r.err == nil; i++ {
		key := r.readString()
		val := r.readValue()
		g.KV[key] = val
	}

	g.Tensors = make([]Tensor, nTensors)
	for i := uint64(0); i < nTensors && r.err == nil; i++ {
		var t Tensor
		t.Name = r.readString()
		var nDims uint32
		r.read(&nDims)
		t.Shape = make([]uint64, nDims)
		for d := range t.Shape {
			r.read(&t.Shape[d])
		}
		var ttype uint32
		r.read(&ttype)
		t.Type = GGMLType(ttype)
		r.read(&t.Offset)
		g.Tensors[i] = t
	}

	if r.err != nil {
		f.Close()
		return nil, fmt.Errorf("gguf: %w", r.err)
	}

	align := uint64(32)
	if a, ok := g.Uint32("general.alignment"); ok {
		align = uint64(a)
	}
	dataStart := uint64(r.pos)
	if rem := dataStart % align; rem != 0 {
		dataStart += align - rem
	}
	g.dataStart = int64(dataStart)

	for i := range g.Tensors {
		t := &g.Tensors[i]
		t.Size = tensorByteSize(t.Type, t.Shape)
		g.tensorByName[t.Name] = t
	}

	return g, nil
}

func (g *File) Close() error { return g.f.Close() }

// Tensor busca um tensor pelo nome.
func (g *File) Tensor(name string) (*Tensor, bool) {
	t, ok := g.tensorByName[name]
	return t, ok
}

// TensorRange lê um intervalo de bytes brutos dentro dos dados de um tensor,
// sem materializar o tensor inteiro (necessário para tensores grandes como
// token_embd/output, onde só uma linha por vez interessa).
func (g *File) TensorRange(name string, byteOffset, length uint64) ([]byte, error) {
	t, ok := g.tensorByName[name]
	if !ok {
		return nil, fmt.Errorf("gguf: tensor %q não encontrado", name)
	}
	if byteOffset+length > t.Size {
		return nil, fmt.Errorf("gguf: intervalo fora dos limites do tensor %q", name)
	}
	buf := make([]byte, length)
	if _, err := g.f.ReadAt(buf, g.dataStart+int64(t.Offset)+int64(byteOffset)); err != nil {
		return nil, fmt.Errorf("gguf: lendo tensor %q: %w", name, err)
	}
	return buf, nil
}

// TensorData lê os bytes brutos (ainda quantizados) de um tensor.
func (g *File) TensorData(name string) ([]byte, error) {
	t, ok := g.tensorByName[name]
	if !ok {
		return nil, fmt.Errorf("gguf: tensor %q não encontrado", name)
	}
	buf := make([]byte, t.Size)
	if _, err := g.f.ReadAt(buf, g.dataStart+int64(t.Offset)); err != nil {
		return nil, fmt.Errorf("gguf: lendo tensor %q: %w", name, err)
	}
	return buf, nil
}

// tensorByteSize replica ggml_nbytes para os tipos que o AdvPP suporta.
// ponytail: só cobre os tipos vistos nos modelos-alvo (F32/F16/I2_S);
// adicionar as linhas de Q4_K/Q6_K/etc quando um modelo K-quant entrar.
func tensorByteSize(t GGMLType, shape []uint64) uint64 {
	n := uint64(1)
	for _, d := range shape {
		n *= d
	}
	switch t {
	case GGMLTypeI2S:
		return n/4 + 32
	case GGMLTypeF16:
		return n * 2
	case GGMLTypeF32:
		return n * 4
	default:
		info, ok := blockInfo[t]
		if !ok {
			return n * 4 // fallback ingênuo; ajustar quando o tipo for suportado
		}
		return (n / info.blck) * info.size
	}
}

var blockInfo = map[GGMLType]struct{ blck, size uint64 }{
	GGMLTypeQ4_0: {32, 18}, GGMLTypeQ4_1: {32, 20},
	GGMLTypeQ5_0: {32, 22}, GGMLTypeQ5_1: {32, 24},
	GGMLTypeQ8_0: {32, 34}, GGMLTypeQ8_1: {32, 40},
	GGMLTypeQ2_K: {256, 84}, GGMLTypeQ3_K: {256, 110},
	GGMLTypeQ4_K: {256, 144}, GGMLTypeQ5_K: {256, 176},
	GGMLTypeQ6_K: {256, 210}, GGMLTypeQ8_K: {256, 292},
}

// --- Acesso tipado a metadados ---

func (g *File) Uint32(key string) (uint32, bool) {
	v, ok := numericKV(g.KV[key])
	return uint32(v), ok
}

func (g *File) Float32(key string) (float32, bool) {
	v, ok := numericKV(g.KV[key])
	return float32(v), ok
}

func (g *File) String(key string) (string, bool) {
	s, ok := g.KV[key].(string)
	return s, ok
}

func (g *File) StringArray(key string) ([]string, bool) {
	arr, ok := g.KV[key].([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(arr))
	for i, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		out[i] = s
	}
	return out, true
}

func (g *File) Int32Array(key string) ([]int32, bool) {
	arr, ok := g.KV[key].([]any)
	if !ok {
		return nil, false
	}
	out := make([]int32, len(arr))
	for i, v := range arr {
		n, ok := numericKV(v)
		if !ok {
			return nil, false
		}
		out[i] = int32(n)
	}
	return out, true
}

func numericKV(v any) (float64, bool) {
	switch n := v.(type) {
	case uint8:
		return float64(n), true
	case int8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case int16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case int32:
		return float64(n), true
	case float32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// --- reader sequencial de baixo nível ---

// reader lê sequencialmente do início do arquivo GGUF (header + metadados +
// lista de tensores) através de um bufio.Reader em vez de um ReadAt cru por
// campo. O header de um modelo real tem centenas de campos (nome+dims+
// tipo+offset por tensor, dezenas de chaves de metadado) — um ReadAt por
// campo (a versão anterior) vira uma syscall pread(2) por campo, e essa
// fase dominava o tempo de LoadModel (~52% em profile real, mais que os
// dados de tensor em si, que são poucas leituras grandes). bufio absorve
// centenas de campos por syscall real; `pos` continua rastreado manualmente
// porque dataStart (onde a seção de dados de tensores começa) é calculado a
// partir dele depois que o header acaba.
type reader struct {
	br  *bufio.Reader
	pos int64
	err error
}

func (r *reader) read(v any) {
	if r.err != nil {
		return
	}
	var buf []byte
	switch v.(type) {
	case *uint32, *int32, *float32:
		buf = make([]byte, 4)
	case *uint64, *int64, *float64:
		buf = make([]byte, 8)
	case *uint16, *int16:
		buf = make([]byte, 2)
	case *uint8, *int8, *bool:
		buf = make([]byte, 1)
	default:
		r.err = fmt.Errorf("gguf: tipo não suportado no reader: %T", v)
		return
	}
	if _, err := io.ReadFull(r.br, buf); err != nil {
		r.err = err
		return
	}
	r.pos += int64(len(buf))
	switch p := v.(type) {
	case *uint32:
		*p = binary.LittleEndian.Uint32(buf)
	case *int32:
		*p = int32(binary.LittleEndian.Uint32(buf))
	case *float32:
		*p = math.Float32frombits(binary.LittleEndian.Uint32(buf))
	case *uint64:
		*p = binary.LittleEndian.Uint64(buf)
	case *int64:
		*p = int64(binary.LittleEndian.Uint64(buf))
	case *float64:
		*p = math.Float64frombits(binary.LittleEndian.Uint64(buf))
	case *uint16:
		*p = binary.LittleEndian.Uint16(buf)
	case *int16:
		*p = int16(binary.LittleEndian.Uint16(buf))
	case *uint8:
		*p = buf[0]
	case *int8:
		*p = int8(buf[0])
	case *bool:
		*p = buf[0] != 0
	}
}

func (r *reader) readString() string {
	if r.err != nil {
		return ""
	}
	var n uint64
	r.read(&n)
	if r.err != nil {
		return ""
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r.br, buf); err != nil {
		r.err = err
		return ""
	}
	r.pos += int64(n)
	return string(buf)
}

// readValue lê um valor de metadado GGUF (escalar ou array) como `any`,
// usando os tipos nativos do Go correspondentes.
func (r *reader) readValue() any {
	var vt uint32
	r.read(&vt)
	return r.readValueOfType(ggufValueType(vt))
}

func (r *reader) readValueOfType(vt ggufValueType) any {
	if r.err != nil {
		return nil
	}
	switch vt {
	case gvtUint8:
		var v uint8
		r.read(&v)
		return v
	case gvtInt8:
		var v int8
		r.read(&v)
		return v
	case gvtUint16:
		var v uint16
		r.read(&v)
		return v
	case gvtInt16:
		var v int16
		r.read(&v)
		return v
	case gvtUint32:
		var v uint32
		r.read(&v)
		return v
	case gvtInt32:
		var v int32
		r.read(&v)
		return v
	case gvtFloat32:
		var v float32
		r.read(&v)
		return v
	case gvtBool:
		var v bool
		r.read(&v)
		return v
	case gvtString:
		return r.readString()
	case gvtUint64:
		var v uint64
		r.read(&v)
		return v
	case gvtInt64:
		var v int64
		r.read(&v)
		return v
	case gvtFloat64:
		var v float64
		r.read(&v)
		return v
	case gvtArray:
		var atype uint32
		r.read(&atype)
		var alen uint64
		r.read(&alen)
		out := make([]any, alen)
		for i := range out {
			out[i] = r.readValueOfType(ggufValueType(atype))
		}
		return out
	default:
		r.err = fmt.Errorf("gguf: tipo de valor desconhecido: %d", vt)
		return nil
	}
}
