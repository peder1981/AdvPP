//go:build !amd64

package llm

// hasAVX2 é sempre falso fora de amd64: dotI2SRow usa exclusivamente o
// caminho escalar puro em arm64 e demais arquiteturas — já validado, sem
// assembly não testável nesta máquina de desenvolvimento (só temos
// hardware/emulação amd64 disponível para verificar SIMD na prática).
const hasAVX2 = false

func dotI2SBlocksAVX2(packed []byte, q []int8, nBlocks int) int32 {
	panic("llm: dotI2SBlocksAVX2 chamado fora de amd64")
}
