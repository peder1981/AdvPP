package vm

import (
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// newTUnicodeObject constrói o objeto da classe tUnicode (TDN: TLPP -
// Classes úteis / tUnicode). Sem estado Go próprio — Normalize e
// ConvertEncoding operam diretamente sobre os argumentos recebidos.
func newTUnicodeObject() *advplrt.ObjectValue {
	return advplrt.NewObject("TUNICODE", nil)
}

// tUnicodeNormForm mapeia CONVMODE_FLAG (0=NFC,1=NFD,2=NFKC,3=NFKD,
// conforme constantes documentadas na página TDN "Normalize") para
// golang.org/x/text/unicode/norm.Form. A ordem dos iota de norm.Form
// (NFC=0, NFD=1, NFKC=2, NFKD=3) já bate exatamente com os valores da
// TDN.
func tUnicodeNormForm(flag int) (norm.Form, bool) {
	switch flag {
	case 0:
		return norm.NFC, true
	case 1:
		return norm.NFD, true
	case 2:
		return norm.NFKC, true
	case 3:
		return norm.NFKD, true
	default:
		return norm.NFC, false
	}
}

func (v *VM) callTUnicodeMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	switch method {
	case "NEW":
		v.push(obj)
		return nil

	case "NORMALIZE":
		// Normalize( < sInput >, < @sConvStr >, < CONVMODE_FLAG > ) -> nRet
		//
		// LIMITAÇÃO CONHECIDA (docs/tdn-known-limitations.md, "Parâmetros
		// por referência (@var) em natives"): @sConvStr é um parâmetro
		// escalar de saída (string) e este VM não tem mecanismo para mutar
		// uma variável escalar do chamador — diferente de HMGet/HMList
		// (cujo out-param é array, tipo referência), aqui não existe
		// escape hatch possível: uma string não é um tipo referência neste
		// VM em nenhuma circunstância. O valor de retorno nRet (0=sucesso,
		// -1=entrada inválida) é real e é a única forma suportada de
		// checar o resultado; o texto normalizado computado de verdade
		// fica inacessível ao código chamador, exatamente como as demais
		// funções já documentadas nesta categoria (GetGlbVars, IPCWaitEx,
		// SFTP*, XmlC14N, DynCall CallFunction/CallMethod).
		sInput := advplrt.ToString(getArg(args, 0))
		flagVal := int(advplrt.ToFloat(getArg(args, 2)))
		form, ok := tUnicodeNormForm(flagVal)
		if !ok {
			v.push(advplrt.NewNumber(-1))
			return nil
		}
		if !utf8.ValidString(sInput) {
			v.push(advplrt.NewNumber(-1))
			return nil
		}
		_ = form.String(sInput) // computado de verdade; não observável via @sConvStr (ver nota acima)
		v.push(advplrt.NewNumber(0))
		return nil

	case "CONVERTENCODING":
		// ConvertEncoding( < cText >, < cFromEncoding >, < cToEncoding > ) -> cRet
		//
		// NOTA DE CONFIANÇA (🟡 INFERIDO): a TDN documenta que
		// "ConvertEncoding é equivalente às DecodeUTF/EncodeUTF utilizadas
		// originalmente no AdvPL", sem publicar uma subpágina própria de
		// sintaxe/parâmetros no material disponível. Implementado por
		// analogia direta a STRICONV (pkg/vm/string_native.go, já cobre a
		// mesma equivalência DecodeUTF/EncodeUTF), reaproveitando
		// resolveEncoding/convertCharSet — mesma lista de codepages
		// suportados, mesmo retorno Nil em nome de codepage desconhecido.
		s := getArgString(args, 0, "")
		fromName := getArgString(args, 1, "")
		toName := getArgString(args, 2, "")
		from := resolveEncoding(fromName)
		to := resolveEncoding(toName)
		if from == nil && !isUTF8Name(fromName) {
			v.push(advplrt.Nil)
			return nil
		}
		if to == nil && !isUTF8Name(toName) {
			v.push(advplrt.Nil)
			return nil
		}
		v.push(advplrt.NewString(convertCharSet(s, from, to)))
		return nil

	default:
		return fmt.Errorf("unknown method %s on tUnicode", method)
	}
}
