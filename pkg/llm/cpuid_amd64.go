//go:build amd64

package llm

// cpuid executa a instrução CPUID (implementada em cpuid_amd64.s) e
// devolve os 4 registradores resultantes, para checar recursos do
// processador sem depender de bibliotecas externas.
func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)

// hasAVX2 é detectado uma única vez na carga do pacote. Se falso (CPU
// amd64 antiga, pré-2013), MatMulI2S usa o caminho escalar puro — nunca
// executa código AVX2 em hardware que não o suporta.
var hasAVX2 = detectAVX2()

func detectAVX2() bool {
	maxLeaf, _, _, _ := cpuid(0, 0)
	if maxLeaf < 7 {
		return false
	}
	_, ebx, _, _ := cpuid(7, 0)
	const avx2Bit = 1 << 5 // CPUID.(EAX=7,ECX=0):EBX.AVX2
	return ebx&avx2Bit != 0
}

// hasF16CFMA é detectado uma única vez na carga do pacote — indica que a
// CPU tem F16C (VCVTPH2PS: conversão half->float em hardware) e FMA
// (VFMADD231PS), usados por dotF16BlocksAVX2 (MatMulF16, a maior matmul do
// forward pass — projeção de saída, vocab_size linhas). Ambas as
// extensões chegaram junto com AVX2 na mesma geração de CPU (Haswell/
// Excavator em diante), mas são bits de CPUID separados — checadas à
// parte para nunca executar VCVTPH2PS/VFMADD em hardware sem suporte.
var hasF16CFMA = detectF16CFMA()

func detectF16CFMA() bool {
	if !hasAVX2 {
		return false
	}
	_, _, ecx, _ := cpuid(1, 0)
	const f16cBit = 1 << 29 // CPUID.01H:ECX.F16C
	const fmaBit = 1 << 12  // CPUID.01H:ECX.FMA
	return ecx&f16cBit != 0 && ecx&fmaBit != 0
}
