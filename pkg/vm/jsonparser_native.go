package vm

import (
	"encoding/json"
	"fmt"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// newTJsonParserObject constrói o objeto da "Classe TJsonParser" (TDN: Não
// Visual / TJsonParser). Não tem estado Go próprio: cada chamada de
// Json_Hash/Json_Parser opera diretamente sobre os argumentos recebidos.
func newTJsonParserObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TJSONPARSER", nil)
	obj.Props["JSON_OK"] = advplrt.False
	return obj
}

// tJsonFlatten decompõe um valor decodificado por encoding/json (top-level
// de um objeto JSON) na forma que o exemplo da TDN espera de oJHM: uma
// chave por campo de primeiro nível (ex.: "inteiro" -> 100) e, quando o
// campo é um array, uma chave adicional por elemento no formato
// "campo[i]" (1-based, ex.: "ROWS[1]" -> primeiro elemento de ROWS) —
// replica exatamente o par de acessos HMGet(oJHM,"inteiro",...) e
// HMGet(oJHM,"ROWS[1]",...) do exemplo da própria página do TDN. Não
// achata recursivamente objetos aninhados (a TDN não demonstra
// HMGet(oJHM,"ROWS[1].OBJ.x",...) em nenhum exemplo).
func tJsonFlatten(top map[string]interface{}, state *hashMapState, fields *[]string) {
	for k, val := range top {
		*fields = append(*fields, k)
		state.set(advplrt.NewString(k), jsonToAdvplValue(val))
		if arr, ok := val.([]interface{}); ok {
			for i, elem := range arr {
				key := fmt.Sprintf("%s[%d]", k, i+1)
				state.set(advplrt.NewString(key), jsonToAdvplValue(elem))
			}
		}
	}
}

func (v *VM) callTJsonParserMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	switch method {
	case "NEW":
		v.push(obj)
		return nil

	case "JSON_HASH":
		// Json_Hash( < cJson >, < nLen >, < @aJsonfields >, < @nRetParser >, < @oJHM > ) -> lRet
		//
		// LIMITAÇÃO CONHECIDA (docs/tdn-known-limitations.md, "Parâmetros
		// por referência (@var) em natives"): @nRetParser é escalar
		// numérico e nunca é populado neste VM. @aJsonfields É populado
		// quando o chamador passa um array de verdade (arrays são tipo
		// referência, mesmo mecanismo de HMList). @oJHM só é populado
		// quando o chamador já passa um objeto tHashMap real (ex.:
		// `oJHM := tHashMap():New()` antes da chamada) — objetos também
		// são tipo referência neste VM (mesmo mecanismo documentado para
		// TRunDll:CallFunction/CallMethod). O exemplo literal da própria
		// TDN inicializa `oJHM := .F.` antes de passar por @oJHM, esperando
		// que a função aloque o hashmap e o devolva por referência a partir
		// de um escalar — isso não é possível neste VM (mesma limitação
		// arquitetural de todo `@var`); o caminho suportado é pré-criar o
		// tHashMap antes de chamar Json_Hash.
		cJson := advplrt.ToString(getArg(args, 0))

		var top map[string]interface{}
		err := json.Unmarshal([]byte(cJson), &top)
		if err != nil {
			obj.Props["JSON_OK"] = advplrt.False
			v.push(advplrt.False)
			return nil
		}

		var fields []string
		if hashObj, ok := getArg(args, 4).(*advplrt.ObjectValue); ok {
			if state := hmGetHash(hashObj); state != nil {
				tJsonFlatten(top, state, &fields)
			}
		} else {
			// Sem oJHM real para popular: ainda assim calcula os nomes de
			// campo para @aJsonfields, que independe do hashmap.
			for k := range top {
				fields = append(fields, k)
			}
		}

		if arr, ok := getArg(args, 2).(*advplrt.ArrayValue); ok {
			elems := make([]advplrt.Value, len(fields))
			for i, f := range fields {
				elems[i] = advplrt.NewString(f)
			}
			arr.Elements = elems
		}

		obj.Props["JSON_OK"] = advplrt.True
		v.push(advplrt.True)
		return nil

	case "JSON_PARSER":
		// Json_Parser( < cJson >, < nLen >, < @aFields > ) -> lRet
		//
		// NOTA DE CONFIANÇA (🟡 INFERIDO): a página TDN fornecida lista
		// "Json_Parser" apenas no índice de métodos da classe, sem uma
		// subpágina de sintaxe/exemplo própria neste PDF (diferente de
		// Json_Hash, cujo exemplo completo está documentado). Implementado
		// por analogia — parsing real do JSON, populando @aFields com os
		// nomes de campo de primeiro nível (mesmo cálculo de Json_Hash,
		// sem o passo de HashMap) — mas não verificado contra a página de
		// detalhe real do método. Mesma limitação de @var de Json_Hash.
		cJson := advplrt.ToString(getArg(args, 0))

		var top map[string]interface{}
		if err := json.Unmarshal([]byte(cJson), &top); err != nil {
			obj.Props["JSON_OK"] = advplrt.False
			v.push(advplrt.False)
			return nil
		}

		if arr, ok := getArg(args, 2).(*advplrt.ArrayValue); ok {
			elems := make([]advplrt.Value, 0, len(top))
			for k := range top {
				elems = append(elems, advplrt.NewString(k))
			}
			arr.Elements = elems
		}

		obj.Props["JSON_OK"] = advplrt.True
		v.push(advplrt.True)
		return nil

	default:
		return fmt.Errorf("unknown method %s on TJsonParser", method)
	}
}
