package injector

// PadToSize adiciona padding NUL ao final do código para manter tamanho fixo.
func PadToSize(code []byte, targetSize int) []byte {
	if len(code) >= targetSize {
		return code[:targetSize]
	}
	padded := make([]byte, targetSize)
	copy(padded, code)
	return padded
}

// TruncateToSize limita o código ao tamanho alvo.
func TruncateToSize(code []byte, targetSize int) []byte {
	if len(code) <= targetSize {
		return code
	}
	return code[:targetSize]
}
