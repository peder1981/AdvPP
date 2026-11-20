package preprocessor

import (
	"bytes"
	"unicode/utf8"
)

// SourceToUTF8 devolve o fonte em UTF-8: se já for UTF-8 válido, como está;
// senão assume CP-1252 (padrão dos fontes Protheus) e converte. Vale para o
// arquivo raiz e para todo #include, para que os dois cheguem ao lexer na
// mesma codificação (pure Go: portável e determinístico em
// Linux/Windows/macOS — sem depender do iconv externo).
func SourceToUTF8(source []byte) string {
	if utf8.Valid(source) {
		return string(source)
	}
	converted, _ := convertWithGoEncoding(source)
	return converted
}

// cp1252ToUTF8 mapeia cada um dos 256 valores de byte possíveis para sua
// codificação UTF-8 (CP-1252 é um superset de ISO-8859-1 com caracteres
// extras em 0x80-0x9F) — construída uma vez em init() por
// buildCP1252ToUTF8Table. convertWithGoEncoding indexa a tabela em vez de
// resolver a mesma cadeia de comparações a cada byte: para um arquivo de
// alguns MB isso é a diferença entre milhões de branches+WriteRune (que
// internamente ainda decide quantos bytes UTF-8 emitir a cada chamada) e
// milhões de acessos de array + Write — foi ~23% do tempo de compilação
// de um arquivo grande em profile real.
var cp1252ToUTF8 = buildCP1252ToUTF8Table()

func buildCP1252ToUTF8Table() [256][]byte {
	var t [256][]byte
	for b := 0; b < 256; b++ {
		t[b] = cp1252ByteToUTF8(byte(b))
	}
	return t
}

// cp1252ByteToUTF8 é a conversão de referência byte a byte (usada só para
// construir cp1252ToUTF8, e em teste, para validar a tabela exaustivamente
// contra os 256 valores possíveis).
func cp1252ByteToUTF8(b byte) []byte {
	if b < 128 {
		return []byte{b}
	}
	if b >= 160 {
		// ISO-8859-1 / CP1252 range (maps directly to Unicode)
		return utf8.AppendRune(nil, rune(b))
	}
	// CP1252 specific characters in range 0x80-0x9F
	switch b {
	case 0x80:
		return utf8.AppendRune(nil, '€') // Euro sign
	case 0x82:
		return utf8.AppendRune(nil, '‚') // Single low-9 quotation mark
	case 0x83:
		return utf8.AppendRune(nil, 'ƒ') // Latin small letter f with hook
	case 0x84:
		return utf8.AppendRune(nil, '„') // Double low-9 quotation mark
	case 0x85:
		return utf8.AppendRune(nil, '…') // Horizontal ellipsis
	case 0x86:
		return utf8.AppendRune(nil, '†') // Dagger
	case 0x87:
		return utf8.AppendRune(nil, '‡') // Double dagger
	case 0x88:
		return utf8.AppendRune(nil, 'ˆ') // Modifier letter circumflex accent
	case 0x89:
		return utf8.AppendRune(nil, '‰') // Per mille sign
	case 0x8A:
		return utf8.AppendRune(nil, 'Š') // Latin capital letter S with caron
	case 0x8B:
		return utf8.AppendRune(nil, '‹') // Single left-pointing angle quotation mark
	case 0x8C:
		return utf8.AppendRune(nil, 'Œ') // Latin capital ligature OE
	case 0x8E:
		return utf8.AppendRune(nil, 'Ž') // Latin capital letter Z with caron
	case 0x91:
		return utf8.AppendRune(nil, '‘') // Left single quotation mark
	case 0x92:
		return utf8.AppendRune(nil, '’') // Right single quotation mark
	case 0x93:
		return utf8.AppendRune(nil, '“') // Left double quotation mark
	case 0x94:
		return utf8.AppendRune(nil, '”') // Right double quotation mark
	case 0x95:
		return utf8.AppendRune(nil, '•') // Bullet
	case 0x96:
		return utf8.AppendRune(nil, '–') // En dash
	case 0x97:
		return utf8.AppendRune(nil, '—') // Em dash
	case 0x98:
		return utf8.AppendRune(nil, '˜') // Small tilde
	case 0x99:
		return utf8.AppendRune(nil, '™') // Trade mark sign
	case 0x9A:
		return utf8.AppendRune(nil, 'š') // Latin small letter s with caron
	case 0x9B:
		return utf8.AppendRune(nil, '›') // Single right-pointing angle quotation mark
	case 0x9C:
		return utf8.AppendRune(nil, 'œ') // Latin small ligature oe
	case 0x9E:
		return utf8.AppendRune(nil, 'ž') // Latin small letter z with caron
	case 0x9F:
		return utf8.AppendRune(nil, 'Ÿ') // Latin capital letter Y with diaeresis
	default:
		// Unknown character, keep as is
		return []byte{b}
	}
}

// convertWithGoEncoding converte bytes CP-1252 para UTF-8 via cp1252ToUTF8.
// Copia em BLOCOS as sequências contíguas de bytes ASCII (b<128, a
// esmagadora maioria de qualquer fonte real — bytes >=128 normalmente só
// aparecem em acentos isolados dentro de comentários/strings) com um único
// buf.Write(source[start:i]) por trecho, em vez de uma chamada de método
// por byte: um arquivo de alguns MB tem só dezenas/centenas de bytes >=128
// mas milhões de bytes ASCII — uma chamada de WriteByte por byte (mesmo
// sendo o método mais barato do bytes.Buffer) ainda paga bounds-check e
// crescimento do buffer a cada chamada, e dominava o profile de um
// arquivo grande mesmo depois da tabela.
func convertWithGoEncoding(source []byte) (string, error) {
	var buf bytes.Buffer
	buf.Grow(len(source) + len(source)/4) // maioria ASCII 1:1; alguns bytes viram 2 bytes UTF-8
	start := 0
	for i, b := range source {
		if b >= 128 {
			if i > start {
				buf.Write(source[start:i])
			}
			buf.Write(cp1252ToUTF8[b])
			start = i + 1
		}
	}
	if start < len(source) {
		buf.Write(source[start:])
	}
	return buf.String(), nil
}
