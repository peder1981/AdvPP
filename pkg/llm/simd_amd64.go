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

// dotF16BlocksAVX2 calcula Σ f16_to_f32(rowF16[16*b : 16*b+16]) * x[8*b :
// 8*b+8] para nBlocks blocos de 8 valores F16 (16 bytes) cada, usando
// VCVTPH2PS (conversão half->float em hardware) + VFMADD231PS. Usada por
// MatMulF16 (projeção de saída, a maior matmul do forward pass — vocab_size
// linhas): o caminho escalar (Float16ToFloat32 via tabela + multiplicação)
// ainda soma dominava o profile mesmo depois da tabela. Implementada em
// simd_amd64.s.
//
// Pré-condições (não checadas aqui, garantidas pelo chamador): len(rowF16)
// >= nBlocks*16, len(x) >= nBlocks*8.
func dotF16BlocksAVX2(rowF16 []byte, x []float32, nBlocks int) float32
