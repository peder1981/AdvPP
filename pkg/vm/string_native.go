package vm

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/unicode"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerStringNatives registra as funções novas da categoria TDN
// "Manipulacao-de-string" (Task 30). As 26 funções já existentes da categoria
// (ALLTRIM, ASC, AT, CHR, ISALPHA, ISDIGIT, ISLOWER, ISUPPER, LEFT, LEN,
// LOWER, LTRIM, PADC, PADL, PADR, RAT, REPLICATE, RIGHT, RTRIM, SPACE,
// STRTOKARR, STRTRAN, STUFF, SUBSTR, TRANSFORM, UPPER) não são duplicadas.
//
// Nota arquitetural: parâmetros por referência (`@cStr`, `@cBufferOut`,
// `@cTarget`, `@nLenghtOut`) não são graváveis nesta VM (natives recebem
// cópias de `[]advplrt.Value`). Nas funções cujo único canal de resultado é o
// parâmetro `@`, o buffer/string modificado é devolvido como valor de retorno
// (forma suportada de uso) e o gap é documentado em
// docs/tdn-known-limitations.md.
func (v *VM) registerStringNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// ANSIToOEM( < cStringAnsi > ) -> cRet
	// Converte ANSI (Windows-1252) para OEM/MS-DOS. Usa CP850 (codepage OEM
	// do Windows pt-BR, CP_OEMCP), que cobre todos os acentos latinos; a TDN
	// cita CP437, mas CP437 não possui ã/õ/À (caracteres comuns em pt-BR) e o
	// exemplo da TDN não discrimina entre as duas (bytes idênticos p/ ç/ä).
	natives["ANSITOOEM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		return advplrt.NewString(convertCharSet(s, charmap.Windows1252, charmap.CodePage850)), nil
	}

	// OEMToANSI( < cStringOEM > ) -> cRet
	// Converte OEM/MS-DOS para ANSI (Windows-1252). Usa CP850 (CP_OEMCP).
	natives["OEMTOANSI"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		return advplrt.NewString(convertCharSet(s, charmap.CodePage850, charmap.Windows1252)), nil
	}

	// Encode64( [cToConvert], [cFilePath*], [lZip*], [lChangeCase*] ) -> cRet
	// Codifica uma string ASCII em BASE64 (RFC 4648). Os parâmetros de
	// arquivo (cFilePath/lZip/lChangeCase) não são suportados nesta VM.
	natives["ENCODE64"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		if advplrt.IsNil(getArg(args, 0)) || s == "" {
			if f := getArgString(args, 1, ""); f != "" {
				return advplrt.Nil, fmt.Errorf("Encode64: variante de arquivo (cFilePath) não suportada nesta VM")
			}
		}
		return advplrt.NewString(base64.StdEncoding.EncodeToString([]byte(s))), nil
	}

	// Decode64( < cToConvert >, [ cFilePath* ], [ lChangeCase* ] ) -> cRet
	// Decodifica uma string BASE64 para o formato original.
	natives["DECODE64"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		if f := getArgString(args, 1, ""); f != "" {
			return advplrt.Nil, fmt.Errorf("Decode64: variante de arquivo (cFilePath) não suportada nesta VM")
		}
		dec, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			// tolera base64 sem padding (formas alternativas)
			dec, err = base64.RawStdEncoding.DecodeString(s)
		}
		if err != nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(string(dec)), nil
	}

	// EncodeUTF8( < cText >, < cEncoding > ) -> cRet
	// Converte string de um code-page (default cp1252) para UTF-8.
	natives["ENCODEUTF8"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		enc := resolveEncoding(getArgString(args, 1, "cp1252"))
		if enc == nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(convertToUTF8(s, enc)), nil
	}

	// DecodeUTF8( < cText >, < cEncoding > ) -> cRet
	// Converte string UTF-8 para um code-page (default cp1252).
	natives["DECODEUTF8"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		enc := resolveEncoding(getArgString(args, 1, "cp1252"))
		if enc == nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(convertFromUTF8(s, enc)), nil
	}

	// EncodeUTF16( < cText >, [ nEndian ] ) -> cRet
	// Converte string CP1252 para UTF-16. nEndian: 1 = Big-Endian (padrão),
	// 2 = Little-Endian.
	natives["ENCODEUTF16"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		endian := int(advplrt.ToFloat(getArg(args, 1)))
		var e unicode.Endianness = unicode.BigEndian
		if endian == 2 {
			e = unicode.LittleEndian
		}
		return advplrt.NewString(encodeUTF16(s, e)), nil
	}

	// DecodeUTF16( < cText >, [ nEndian ] ) -> cRet
	// Converte string UTF-16 para CP1252. nEndian: 1 = Big-Endian (padrão),
	// 2 = Little-Endian.
	natives["DECODEUTF16"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		endian := int(advplrt.ToFloat(getArg(args, 1)))
		var e unicode.Endianness = unicode.BigEndian
		if endian == 2 {
			e = unicode.LittleEndian
		}
		return advplrt.NewString(decodeUTF16(s, e)), nil
	}

	// Descend( < cString > ) -> cRet
	// Retorna a forma complementada da string, para indexação decrescente.
	// CHR(0) sempre retorna CHR(0); demais bytes são invertidos (255 - byte).
	natives["DESCEND"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		out := make([]byte, len(s))
		for i := 0; i < len(s); i++ {
			if s[i] == 0 {
				out[i] = 0
			} else {
				out[i] = 255 - s[i]
			}
		}
		return advplrt.NewString(string(out)), nil
	}

	// GetDtoVal( < cDtoVal > ) -> nRet
	// Converte string numérica para número. Um caractere '.' presente indica
	// fracionamento na ordem em que aparece na string. Não considera negativo.
	natives["GETDTOVAL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		return advplrt.NewNumber(getDtoVal(s)), nil
	}

	// Match( < cValue >, < cMask > ) -> lRet
	// Valida se cValue está formatado conforme o padrão cMask (* e ? são
	// coringas; * = 0+ caracteres, ? = 1 caractere). Não é case sensitive.
	// cMask vazio retorna .T.
	natives["MATCH"] = func(args []advplrt.Value) (advplrt.Value, error) {
		val := strings.ToUpper(getArgString(args, 0, ""))
		mask := strings.ToUpper(getArgString(args, 1, ""))
		if mask == "" {
			return advplrt.True, nil
		}
		return advplrt.NewBool(matchPattern(val, mask)), nil
	}

	// MathC( < cNum1 >, < cOperacao >, < cNum2 > ) -> cRet
	// Realiza operação matemática com strings numéricas. Operadores: / + * - e
	// ('e' = exponenciação). Retorna string com até 18 casas de precisão.
	natives["MATHC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		n1 := advplrt.ToFloat(getArg(args, 0))
		op := strings.TrimSpace(getArgString(args, 1, ""))
		n2 := advplrt.ToFloat(getArg(args, 2))
		var res float64
		switch op {
		case "/":
			if n2 == 0 {
				return advplrt.Nil, nil
			}
			res = n1 / n2
		case "+":
			res = n1 + n2
		case "*":
			res = n1 * n2
		case "-":
			res = n1 - n2
		case "e", "E":
			res = math.Pow(n1, n2)
		default:
			return advplrt.Nil, nil
		}
		return advplrt.NewString(strconv.FormatFloat(res, 'f', -1, 64)), nil
	}

	// Pad( < xExp >, < nLen >, [ cFill ] ) -> cRet
	// Mesmo comportamento de PadR: preenche à direita com cFill (padrão
	// espaço) até nLen; trunca se maior; nLen <= 0 retorna "".
	natives["PAD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return nativePadRight(args), nil
	}

	// STRICONV( < cText >, < fromCodePage >, < toCodePage > ) -> cRet
	// Converte string de um codepage para outro. "UTF-8"/"utf8" é tratado como
	// passthrough no lado correspondente.
	natives["STRICONV"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		fromName := getArgString(args, 1, "")
		toName := getArgString(args, 2, "")
		from := resolveEncoding(fromName)
		to := resolveEncoding(toName)
		if from == nil && !isUTF8Name(fromName) {
			return advplrt.Nil, nil
		}
		if to == nil && !isUTF8Name(toName) {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(convertCharSet(s, from, to)), nil
	}

	// StrTokArr2( < cValue >, < cToken >, [ lEmptyStr ] ) -> aRet
	// Divide cValue usando a sequência cToken como separador (diferente de
	// StrTokArr, que usa cada caractere). Elementos vazios são omitidos por
	// padrão (lEmptyStr=.T. os inclui).
	natives["STRTOKARR2"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		token := getArgString(args, 1, "")
		lEmpty := advplrt.ToBool(getArg(args, 2))
		return advplrt.NewArray(strTokArr2(s, token, lEmpty)), nil
	}

	// Compress( < @cBufferOut >, < @nLenghtOut >, < cBufferIn >, < nLenghtIn > )
	// Compacta um buffer. O algoritmo proprietário TOTVS não é reproduzível;
	// usa-se zlib (RFC 1950) para round-trip interno consistente com
	// UnCompress. O buffer compactado é devolvido como valor de retorno.
	natives["COMPRESS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		in := getArgString(args, 2, "")
		nLen := int(advplrt.ToFloat(getArg(args, 3)))
		if nLen < 0 {
			return advplrt.Nil, errors.New("Unpacked size underflow on compress")
		}
		if nLen > len(in) {
			return advplrt.Nil, errors.New("Unpacked size overflow on compress")
		}
		if nLen > 0 && nLen < len(in) {
			in = in[:nLen]
		}
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		if _, err := zw.Write([]byte(in)); err != nil {
			return advplrt.Nil, nil
		}
		if err := zw.Close(); err != nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(buf.String()), nil
	}

	// UnCompress( < @cBufferOut >, < @nLenghtOut >, < cBufferIn >, < nLenghtIn > )
	// Descompacta buffer gerado por Compress. O buffer descompactado é
	// devolvido como valor de retorno.
	natives["UNCOMPRESS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		in := getArgString(args, 2, "")
		nLen := int(advplrt.ToFloat(getArg(args, 3)))
		if nLen < 0 {
			return advplrt.Nil, errors.New("Packed size underflow on uncompress")
		}
		if nLen > len(in) {
			return advplrt.Nil, errors.New("Packed size overflow on uncompress")
		}
		if nLen > 0 && nLen < len(in) {
			in = in[:nLen]
		}
		zr, err := zlib.NewReader(bytes.NewReader([]byte(in)))
		if err != nil {
			return advplrt.Nil, nil
		}
		out, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(string(out)), nil
	}

	// GzStrComp( < cSource >, < @cTarget >, < @nTargetLen > ) -> lRet
	// Compacta string no formato gzip (GNU zip). String vazia gera exceção.
	// A string compactada é devolvida como valor de retorno.
	natives["GZSTRCOMP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := getArgString(args, 0, "")
		if src == "" {
			return advplrt.Nil, errors.New("Error in GzStrComp(): String is empty.")
		}
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write([]byte(src)); err != nil {
			return advplrt.Nil, nil
		}
		if err := gw.Close(); err != nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(buf.String()), nil
	}

	// GzStrDecomp( < cSource >, < nSourceLen >, < @cTarget > ) -> lRet
	// Descompacta string no formato gzip. A string descompactada é devolvida
	// como valor de retorno.
	natives["GZSTRDECOMP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		src := getArgString(args, 0, "")
		nLen := int(advplrt.ToFloat(getArg(args, 1)))
		if nLen > 0 && nLen < len(src) {
			src = src[:nLen]
		}
		gr, err := gzip.NewReader(bytes.NewReader([]byte(src)))
		if err != nil {
			return advplrt.Nil, nil
		}
		out, err := io.ReadAll(gr)
		gr.Close()
		if err != nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(string(out)), nil
	}

	// BitOn( < cStr >, < nStart >, < nTest >, < nLength > ) -> nRet
	// Verifica se os primeiros nTest bits (a partir de nStart) estão em 0.
	// Retorna 1 se todos os bits testados estão em 0; caso contrário 0.
	natives["BITON"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		nStart := int(advplrt.ToFloat(getArg(args, 1)))
		nTest := int(advplrt.ToFloat(getArg(args, 2)))
		nLength := int(advplrt.ToFloat(getArg(args, 3)))
		if len(s) < nLength {
			return advplrt.Nil, errors.New("Bit string out of bounds on BitOn")
		}
		bits := stringToBits(s)
		total := 0
		if nLength > 0 {
			total = (nLength + 1) * 8
		} else {
			total = len(bits)
		}
		end := nStart + nTest - 1
		if end > total {
			end = total
		}
		for i := nStart - 1; i < end; i++ {
			if i >= 0 && i < len(bits) && bits[i] == 1 {
				return advplrt.NewNumber(0), nil
			}
		}
		return advplrt.NewNumber(1), nil
	}

	// Look4Bit( < cStr >, < nStart >, < nTest >, < nLength > ) -> nRet
	// Retorna a quantidade de bits 1 entre nStart e (nStart+nTest-1),
	// limitado ao último byte nLength (índice 0-based).
	natives["LOOK4BIT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		nStart := int(advplrt.ToFloat(getArg(args, 1)))
		nTest := int(advplrt.ToFloat(getArg(args, 2)))
		nLength := int(advplrt.ToFloat(getArg(args, 3)))
		if nStart < 1 {
			return advplrt.Nil, errors.New("Start Bit out of bounds on Look4Bit")
		}
		if nLength < 0 {
			return advplrt.Nil, errors.New("Length Bit out of bounds on Look4Bit")
		}
		if len(s) < nLength {
			return advplrt.Nil, errors.New("Bit string out of bounds on Look4Bit")
		}
		bits := stringToBits(s)
		total := len(bits)
		if nLength > 0 {
			total = (nLength + 1) * 8
		}
		if nStart > total {
			nStart = total + 1
		}
		end := nStart + nTest - 1
		if end > total {
			end = total
		}
		count := 0
		for i := nStart - 1; i < end && i < len(bits); i++ {
			if bits[i] == 1 {
				count++
			}
		}
		return advplrt.NewNumber(float64(count)), nil
	}

	// NotBit( < @cStr >, < nLength > )
	// Inverte os bits dos nLength primeiros caracteres. A string modificada é
	// devolvida como valor de retorno (o parâmetro @ não é gravável na VM).
	natives["NOTBIT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		nLength := int(advplrt.ToFloat(getArg(args, 1)))
		if len(s) < nLength {
			return advplrt.Nil, errors.New("Bit string out of bounds on NotBit")
		}
		bytes := []byte(s)
		if nLength > len(bytes) {
			nLength = len(bytes)
		}
		for i := 0; i < nLength; i++ {
			bytes[i] = ^bytes[i]
		}
		return advplrt.NewString(string(bytes)), nil
	}

	// StuffBit( < @cStr >, < nStart >, < nTest >, < nLength > )
	// Coloca nTest bits em 1 a partir de nStart. A string modificada é
	// devolvida como valor de retorno.
	natives["STUFFBIT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		nStart := int(advplrt.ToFloat(getArg(args, 1)))
		nTest := int(advplrt.ToFloat(getArg(args, 2)))
		nLength := int(advplrt.ToFloat(getArg(args, 3)))
		if nStart < 0 {
			return advplrt.Nil, errors.New("Start Bit out of bounds on StuffBit")
		}
		if nTest < 0 {
			return advplrt.Nil, errors.New("Test Bit out of bounds on StuffBit")
		}
		if nLength < 0 {
			return advplrt.Nil, errors.New("Length Bit out of bounds on StuffBit")
		}
		if len(s) < nLength {
			return advplrt.Nil, errors.New("Bit string out of bounds on StuffBit")
		}
		bits := stringToBits(s)
		total := len(bits)
		if nLength > 0 {
			total = (nLength + 1) * 8
		}
		if nStart == 0 && nTest > total {
			nTest = nTest - 1
		}
		if nStart < 1 {
			nStart = 1
		}
		end := nStart + nTest - 1
		if end > total {
			end = total
		}
		for i := nStart - 1; i < end && i < len(bits); i++ {
			bits[i] = 1
		}
		return advplrt.NewString(bitsToString(bits)), nil
	}

	// UnStuff( < @cStr >, < nStart >, < nTest >, < nLength > )
	// Coloca nTest bits em 0 a partir de nStart. A string modificada é
	// devolvida como valor de retorno.
	natives["UNSTUFF"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := getArgString(args, 0, "")
		nStart := int(advplrt.ToFloat(getArg(args, 1)))
		nTest := int(advplrt.ToFloat(getArg(args, 2)))
		nLength := int(advplrt.ToFloat(getArg(args, 3)))
		if nStart < 0 {
			return advplrt.Nil, errors.New("Start bit underflow on UnStuff")
		}
		if nTest < 0 {
			return advplrt.Nil, errors.New("Test Bit underflow on UnStuff")
		}
		if len(s) < nLength {
			return advplrt.Nil, errors.New("Bit string length out of bounds on UnStuff")
		}
		bits := stringToBits(s)
		total := len(bits)
		if nLength > 0 {
			total = (nLength + 1) * 8
		}
		if nStart < 1 {
			nStart = 1
		}
		end := nStart + nTest - 1
		if end > total {
			end = total
		}
		for i := nStart - 1; i < end && i < len(bits); i++ {
			bits[i] = 0
		}
		return advplrt.NewString(bitsToString(bits)), nil
	}

	// MLCount( < cText >, [ nLinLen ], [ nTabSize ], [ lQuebra ] ) -> nLin
	// Conta linhas de texto com múltiplas linhas considerando quebra de linha
	// em nLinLen (default 79), tabulação nTabSize (default 4) e lQuebra
	// (default .T. = não quebra palavra; .F. = quebra a palavra no limite).
	natives["MLCOUNT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		text := getArgString(args, 0, "")
		nLinLen := int(advplrt.ToFloat(getArg(args, 1)))
		nTabSize := int(advplrt.ToFloat(getArg(args, 2)))
		// lQuebra default .T. (não quebra palavra) quando não informado
		lQuebra := true
		if len(args) > 3 && !advplrt.IsNil(getArg(args, 3)) {
			lQuebra = advplrt.ToBool(getArg(args, 3))
		}
		if nLinLen <= 0 {
			nLinLen = 79
		}
		if nTabSize <= 0 {
			nTabSize = 4
		}
		return advplrt.NewNumber(float64(countMemoLines(text, nLinLen, nTabSize, lQuebra))), nil
	}
}

// --- helpers -----------------------------------------------------------------

// nativePadRight replica o comportamento de PadR (usado por Pad, que é
// idêntico a PadR conforme a TDN).
func nativePadRight(args []advplrt.Value) advplrt.Value {
	s := advplrt.ToString(getArg(args, 0))
	size := int(advplrt.ToFloat(getArg(args, 1)))
	pad := " "
	if len(args) >= 3 {
		pad = advplrt.ToString(getArg(args, 2))
	}
	if pad == "" {
		pad = " "
	}
	if size <= 0 {
		return advplrt.NewString("")
	}
	if len(pad) > 1 {
		pad = pad[:1]
	}
	for len(s) < size {
		s = s + pad
	}
	if len(s) > size {
		s = s[:size]
	}
	return advplrt.NewString(s)
}

// convertCharSet converte s do encoding `from` para o encoding `to`. Um
// encoding nil (UTF-8, formato nativo do VM) é tratado como passthrough no
// lado correspondente.
func convertCharSet(s string, from, to encoding.Encoding) string {
	if s == "" {
		return s
	}
	decoded := s
	if from != nil {
		d, err := from.NewDecoder().String(s)
		if err == nil {
			decoded = d
		}
	}
	if to == nil {
		return decoded
	}
	encoded, err := to.NewEncoder().String(decoded)
	if err != nil {
		return decoded
	}
	return encoded
}

// convertToUTF8 converte s (em encoding `enc`) para UTF-8. enc nil (UTF-8)
// é passthrough.
func convertToUTF8(s string, enc encoding.Encoding) string {
	if enc == nil || s == "" {
		return s
	}
	out, err := enc.NewDecoder().String(s)
	if err != nil {
		return s
	}
	return out
}

// convertFromUTF8 converte s (UTF-8) para o encoding `enc`. enc nil (UTF-8)
// é passthrough.
func convertFromUTF8(s string, enc encoding.Encoding) string {
	if enc == nil || s == "" {
		return s
	}
	out, err := enc.NewEncoder().String(s)
	if err != nil {
		return s
	}
	return out
}

// resolveEncoding resolve um nome de codepage da TDN ("cp1252", "iso8859-1",
// "UTF-8", etc.) para um encoding de golang.org/x/text. Retorna nil se não
// reconhecido. O nome "utf-8"/"utf8" é tratado pelo chamador como passthrough
// (nil = não decodifica/encoda), não como erro.
func resolveEncoding(name string) encoding.Encoding {
	if name == "" {
		return nil
	}
	lower := strings.ToLower(name)
	switch lower {
	case "cp1252", "windows-1252", "win1252", "ansi":
		return charmap.Windows1252
	case "cp437", "ibm437", "oem":
		return charmap.CodePage437
	case "cp850", "ibm850":
		return charmap.CodePage850
	case "iso8859-1", "iso-8859-1", "latin1", "latin-1":
		return charmap.ISO8859_1
	}
	if e, err := ianaindex.IANA.Encoding(lower); err == nil {
		return e
	}
	return nil
}

// isUTF8Name indica se um nome de codepage representa UTF-8 (passthrough).
func isUTF8Name(name string) bool {
	l := strings.ToLower(strings.TrimSpace(name))
	return l == "utf-8" || l == "utf8"
}

// encodeUTF16 converte uma string CP1252 para UTF-16 com o endianness dado.
func encodeUTF16(s string, e unicode.Endianness) string {
	if s == "" {
		return ""
	}
	// Converte CP1252 -> Unicode (rune)
	decoded, err := charmap.Windows1252.NewDecoder().String(s)
	if err != nil {
		decoded = s
	}
	enc := unicode.UTF16(e, unicode.IgnoreBOM)
	out, err := enc.NewEncoder().String(decoded)
	if err != nil {
		return ""
	}
	return out
}

// decodeUTF16 converte uma string UTF-16 para CP1252 com o endianness dado.
func decodeUTF16(s string, e unicode.Endianness) string {
	if s == "" {
		return ""
	}
	enc := unicode.UTF16(e, unicode.IgnoreBOM)
	decoded, err := enc.NewDecoder().String(s)
	if err != nil {
		return ""
	}
	out, err := charmap.Windows1252.NewEncoder().String(decoded)
	if err != nil {
		return decoded
	}
	return out
}

// getDtoVal converte uma string numérica em número (sem negativo). Um '.'
// presente indica fracionamento: dígitos antes do '.' formam a parte inteira
// (na ordem em que aparecem) e os dígitos depois formam a fração.
func getDtoVal(s string) float64 {
	if s == "" {
		return 0
	}
	intPart := ""
	fracPart := ""
	seenDot := false
	for _, r := range s {
		switch {
		case r == '.':
			seenDot = true
		case r >= '0' && r <= '9':
			if seenDot {
				fracPart += string(r)
			} else {
				intPart += string(r)
			}
		}
	}
	// Inteiro = dígitos concatenados na ordem
	n := 0.0
	for _, ch := range intPart {
		n = n*10 + float64(ch-'0')
	}
	if seenDot && fracPart != "" {
		div := math.Pow(10, float64(len(fracPart)))
		f := 0.0
		for _, ch := range fracPart {
			f = f*10 + float64(ch-'0')
		}
		n += f / div
	}
	return n
}

// matchPattern implementa o glob de Match: '*' = 0+ chars, '?' = 1 char.
func matchPattern(val, mask string) bool {
	return globMatch(mask, val)
}

func globMatch(pattern, s string) bool {
	// conversão para DP de glob clássico
	m := len(pattern)
	n := len(s)
	dp := make([][]bool, m+1)
	for i := range dp {
		dp[i] = make([]bool, n+1)
	}
	dp[0][0] = true
	for i := 1; i <= m; i++ {
		if pattern[i-1] == '*' {
			dp[i][0] = dp[i-1][0]
		}
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			switch pattern[i-1] {
			case '*':
				dp[i][j] = dp[i-1][j] || dp[i][j-1]
			case '?':
				dp[i][j] = dp[i-1][j-1]
			default:
				if pattern[i-1] == s[j-1] {
					dp[i][j] = dp[i-1][j-1]
				}
			}
		}
	}
	return dp[m][n]
}

// stringToBits converte uma string em um slice de bits MSB-first (bit 0 do
// byte 0 é o bit mais significativo).
func stringToBits(s string) []byte {
	bits := make([]byte, 0, len(s)*8)
	for i := 0; i < len(s); i++ {
		for b := 7; b >= 0; b-- {
			bits = append(bits, (s[i]>>uint(b))&1)
		}
	}
	return bits
}

// bitsToString converte um slice de bits MSB-first de volta em bytes.
func bitsToString(bits []byte) string {
	nBytes := (len(bits) + 7) / 8
	out := make([]byte, nBytes)
	for i, b := range bits {
		if b != 0 {
			out[i/8] |= 1 << uint(7-(i%8))
		}
	}
	return string(out)
}

// strTokArr2 divide s pelo separador de sequência token, omitindo elementos
// vazios a menos que lEmpty seja verdadeiro.
func strTokArr2(s, token string, lEmpty bool) []advplrt.Value {
	if token == "" {
		return []advplrt.Value{advplrt.NewString(s)}
	}
	// ignora ASCII 0 na string (conforme TDN)
	s = strings.ReplaceAll(s, "\x00", "")
	parts := strings.Split(s, token)
	elems := make([]advplrt.Value, 0, len(parts))
	for _, p := range parts {
		if !lEmpty && p == "" {
			continue
		}
		elems = append(elems, advplrt.NewString(p))
	}
	return elems
}

// countMemoLines conta linhas de um texto memo considerando quebra em nLinLen,
// tabs de nTabSize e lQuebra (default .T. = não quebra palavras; .F. = quebra
// palavras no limite da linha).
func countMemoLines(text string, nLinLen, nTabSize int, lQuebra bool) int {
	// expande tabs
	expanded := ""
	for _, r := range text {
		if r == '\t' {
			expanded += strings.Repeat(" ", nTabSize)
		} else {
			expanded += string(r)
		}
	}
	lines := 0
	for _, raw := range strings.Split(expanded, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if line == "" {
			if strings.HasSuffix(raw, "\n") {
				lines++
			}
			continue
		}
		lines += wrapCount(line, nLinLen, lQuebra)
	}
	if lines == 0 && text != "" {
		lines = 1
	}
	return lines
}

// wrapCount conta quantas linhas de no máximo width são necessárias para s.
// lQuebra=true: quebra no último espaço antes do limite (não quebra palavra).
// lQuebra=false: quebra no limite exato da linha e trima o espaço inicial da
// linha seguinte (comportamento do MemoLine, conforme exemplo da TDN).
func wrapCount(s string, width int, lQuebra bool) int {
	count := 0
	for len(s) > width {
		if lQuebra {
			cut := strings.LastIndex(s[:width], " ")
			if cut <= 0 {
				cut = width // palavra mais longa que a linha: quebra no limite
			}
			count++
			s = strings.TrimLeft(s[cut:], " ")
		} else {
			count++
			s = strings.TrimLeft(s[width:], " ")
		}
	}
	if s != "" {
		count++
	}
	return count
}

