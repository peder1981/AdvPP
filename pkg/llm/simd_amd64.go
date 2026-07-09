//go:build amd64

package llm

// dotI2SBlocksAVX2 processa nBlocks blocos completos de 128 valores
// ternários (32 bytes empacotados cada) via AVX2, retornando a soma dos
// CÓDIGOS BRUTOS (0/1/2, sem o mapeamento para -1/0/+1) multiplicados por
// q — replica a técnica de ggml-bitnet-mad.cpp (VPMADDUBSW exige um
// operando unsigned; por isso opera nos códigos crus). O chamador aplica a
// correção `-Σq` para obter o dot product ternário verdadeiro (ver
// dotI2SRow em i2s.go). Implementada em simd_amd64.s.
//
// Pré-condições (não checadas aqui, garantidas pelo chamador): len(packed)
// >= nBlocks*32, len(q) >= nBlocks*128.
func dotI2SBlocksAVX2(packed []byte, q []int8, nBlocks int) int32
