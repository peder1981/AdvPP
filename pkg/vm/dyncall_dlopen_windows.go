//go:build windows

package vm

import "golang.org/x/sys/windows"

// dynCallDlopen/Sym/Close — variante Windows. purego não implementa
// Dlopen/Dlsym/Dlclose neste SO (seu próprio doc comment recomenda
// golang.org/x/sys/windows.LoadLibrary/GetProcAddress/FreeLibrary) — já
// é dependência indireta existente do projeto (go.mod), promovida aqui a
// direta. purego.RegisterFunc, usado pelo resto de dyncall_native.go, é
// agnóstico de como o endereço do símbolo foi obtido — só esta resolução
// de símbolo muda por SO.
func dynCallDlopen(path string) (uintptr, error) {
	h, err := windows.LoadLibrary(path)
	return uintptr(h), err
}

func dynCallDlsym(handle uintptr, name string) (uintptr, error) {
	return windows.GetProcAddress(windows.Handle(handle), name)
}

func dynCallDlclose(handle uintptr) error {
	return windows.FreeLibrary(windows.Handle(handle))
}
