package vm

import (
	"encoding/json"
	"fmt"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Classes que completam o catálogo do llms.txt do PO-UI:
//   - PoDynamicForm():New(oPai, aCampos[, jValor]) — po-dynamic-form.
//     SetFields(a), SetValue(j), GetValue() -> JsonObject e
//     OnChange(oForm, cCampo, xValor). aCampos no formato dos templates
//     dinâmicos ({cProp, cLabel, cTipo, lKey, lFilter, lVisible}).
//   - PoLogin():New(oPai, cLabel, cValor) — po-login (campo de login);
//     mesmo uso do PoInput (SetValue/GetValue/OnChange).
//   - PoModalPasswordRecovery():New(oPai, cTipo) — po-modal-password-
//     recovery (@po-ui/ng-templates); cTipo email|cpf|all. Open(), Close(),
//     Completed(), SetContactEmail(c), SetPhoneMask(c) e
//     OnSubmit(oModal, jDados).

var poTiposRecuperacao = map[string]bool{"email": true, "cpf": true, "all": true}

// poComplementoNew: props iniciais de dynamicform/passwordrecovery.
func poComplementoNew(c *poComp, args []advplrt.Value) error {
	switch c.typ {
	case "dynamicform":
		fields, err := poDynFields(getArg(args, 1))
		if err != nil {
			return err
		}
		c.props["fields"] = fields
		valor, _ := poToJSON(getArg(args, 2)).(map[string]any)
		if valor == nil {
			valor = map[string]any{}
		}
		c.props["value"] = valor
	case "passwordrecovery":
		tipo := strings.ToLower(argTexto(args, 1, "email"))
		if !poTiposRecuperacao[tipo] {
			return fmt.Errorf("PoModalPasswordRecovery: tipo inválido %q (email, cpf, all)", tipo)
		}
		c.props["type"] = tipo
		c.props["open"] = false
		c.props["completed"] = false
		c.props["contactEmail"] = ""
		c.props["phoneMask"] = ""
	}
	return nil
}

// poComplementoMethod: métodos próprios das classes acima; os genéricos
// (SetValue, GetValue, OnChange, SetLabel...) seguem no callPoCompMethod.
func poComplementoMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	a := c.app
	switch c.typ {
	case "dynamicform":
		switch method {
		case "SETFIELDS":
			fields, err := poDynFields(getArg(args, 0))
			if err != nil {
				return true, advplrt.Nil, err
			}
			a.setProp(c, "fields", fields)
			return true, advplrt.Nil, nil
		case "SETVALUE":
			valor, _ := poToJSON(getArg(args, 0)).(map[string]any)
			if valor == nil {
				valor = map[string]any{}
			}
			a.setProp(c, "value", valor)
			return true, advplrt.Nil, nil
		}
	case "passwordrecovery":
		switch method {
		case "OPEN":
			a.setProp(c, "completed", false)
			a.setProp(c, "open", true)
		case "CLOSE":
			a.setProp(c, "open", false)
		case "COMPLETED":
			a.setProp(c, "completed", true)
		case "SETCONTACTEMAIL":
			a.setProp(c, "contactEmail", advplrt.ToString(getArg(args, 0)))
		case "SETPHONEMASK":
			a.setProp(c, "phoneMask", advplrt.ToString(getArg(args, 0)))
		case "ONSUBMIT":
			c.handlers["recoverysubmit"] = getArg(args, 0)
		default:
			return false, advplrt.Nil, nil
		}
		return true, advplrt.Nil, nil
	}
	return false, advplrt.Nil, nil
}

// poCampoAlterado: evento fieldchange do PoDynamicForm. Atualiza o valor
// guardado (veio do browser: sem patch) e devolve campo e valor.
func poCampoAlterado(c *poComp, val []byte) (string, advplrt.Value, bool) {
	var body struct {
		Property string          `json:"property"`
		Value    json.RawMessage `json:"value"`
	}
	if json.Unmarshal(val, &body) != nil || body.Property == "" {
		return "", advplrt.Nil, false
	}
	valor := advplrt.Value(advplrt.Nil)
	var bruto any
	if len(body.Value) > 0 && json.Unmarshal(body.Value, &bruto) == nil {
		valor = poFromJSONPoEvent(body.Value)
	}
	m, _ := c.props["value"].(map[string]any)
	if m == nil {
		m = map[string]any{}
		c.props["value"] = m
	}
	m[body.Property] = bruto
	return body.Property, valor, true
}
