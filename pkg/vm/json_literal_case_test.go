package vm

import (
	"encoding/json"
	"testing"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// O literal JSON do TLPP guarda as chaves como escritas (o JSON da
// resposta REST precisa de "items"/"hasNext"/"message", que o PO-UI lê);
// colchetes diferenciam maiúsculas, e o acesso por propriedade continua
// achando a chave sem diferenciar.
func TestJsonLiteralPreservaCaixa(t *testing.T) {
	bc := compilarPrg(t, `
User Function Lit()
	Local j := {"items": {1, 2}, "hasNext": .F.}
	j:hasNext := .T.
	j["message"] := "ok"
	Return {j, j["items"], j:items, j:HASNEXT, j["HASNEXT"]}
`)
	v := NewVM(bc, false)
	res, err := v.RunFunction("U_LIT", nil)
	if err != nil {
		t.Fatal(err)
	}
	e := res.(*advplrt.ArrayValue).Elements
	b, _ := json.Marshal(advplValueToJSON(e[0]))
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if len(m) != 3 || m["hasNext"] != true || m["message"] != "ok" || m["items"] == nil {
		t.Fatalf("JSON = %s, quer chaves items/hasNext/message (hasNext atualizado, sem duplicar)", b)
	}
	if _, ok := e[1].(*advplrt.ArrayValue); !ok {
		t.Fatalf(`j["items"] = %v, quer o array`, e[1])
	}
	if _, ok := e[2].(*advplrt.ArrayValue); !ok {
		t.Fatalf("j:items = %v, quer o array", e[2])
	}
	if !advplrt.ToBool(e[3]) {
		t.Fatalf("j:HASNEXT = %v, quer .T.", e[3])
	}
	if e[4] != advplrt.Nil && e[4] != nil {
		if _, isNil := e[4].(*advplrt.NilValue); !isNil {
			t.Fatalf(`j["HASNEXT"] = %v, quer Nil (colchetes diferenciam maiúsculas)`, e[4])
		}
	}
}
