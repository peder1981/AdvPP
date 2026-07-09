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
