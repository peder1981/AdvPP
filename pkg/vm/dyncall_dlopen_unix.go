//go:build !windows

package vm

import "github.com/ebitengine/purego"

// dynCallDlopen/Sym/Close abstraem a resolução de símbolos de biblioteca
// dinâmica por SO: purego.Dlopen/Dlsym/Dlclose cobrem Linux/macOS/BSD.
// purego explicitamente NÃO implementa essas três em Windows (ver
// dyncall_dlopen_windows.go) — cada plataforma tem seu próprio arquivo
// com a mesma assinatura, nenhuma lógica de tRunDll depende de qual.
func dynCallDlopen(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

func dynCallDlsym(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}

func dynCallDlclose(handle uintptr) error {
	return purego.Dlclose(handle)
}
