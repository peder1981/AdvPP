package vm

import (
	"encoding/json"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3a.2 — data e hora. Fronteira do plano: Date AdvPL <-> ISO
// AAAA-MM-DD; datetime <-> ISO 8601; hora <-> "HH:MM[:SS]". O frontend
// entrega Date (JSON ISO) ou string formatada; aqui tudo vira data AdvPL.

// poDataISO converte um valor AdvPL para "AAAA-MM-DD" (vazio -> "").
func poDataISO(x advplrt.Value) string {
	switch v := advplrt.Deref(x).(type) {
	case *advplrt.DateValue:
		if v == nil || v.Val.IsZero() {
			return ""
		}
		return v.Val.Format("2006-01-02")
	case advplrt.Value:
		s := strings.TrimSpace(advplrt.ToString(v))
		if s == "" {
			return ""
		}
		// Aceita dd/mm/yyyy e ISO na entrada.
		if t, err := time.Parse("02/01/2006", s); err == nil {
			return t.Format("2006-01-02")
		}
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t.Format("2006-01-02")
		}
		return ""
	}
	return ""
}

// poDataDeValor converte o que vem do browser (ISO, ISO datetime ou
// dd/mm/yyyy) para DateValue. Vazio/inválido -> Nil.
func poDataDeValor(x advplrt.Value) advplrt.Value {
	s := strings.TrimSpace(advplrt.ToString(x))
	if s == "" || s == "null" {
		return advplrt.Nil
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "02/01/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return advplrt.NewDate(t)
		}
	}
	return advplrt.Nil
}

// po3aDateMethod despacha os métodos dos componentes de data/hora.
// Devolve true quando o método foi tratado.
func (v *VM) po3aDateMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	a := c.app
	switch c.typ {
	case "datepicker":
		switch method {
		case "SETVALUE":
			a.setProp(c, "value", poDataISO(getArg(args, 0)))
		case "GETVALUE":
			return true, poDataDeValor(poFromJSON(c.props["value"])), nil
		case "SETMINDATE":
			a.setProp(c, "minDate", poDataISO(getArg(args, 0)))
		case "SETMAXDATE":
			a.setProp(c, "maxDate", poDataISO(getArg(args, 0)))
		case "SETFORMAT":
			a.setProp(c, "format", advplrt.ToString(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "daterange":
		switch method {
		case "SETVALUE":
			a.setProp(c, "value", map[string]any{
				"start": poDataISO(getArg(args, 0)),
				"end":   poDataISO(getArg(args, 1)),
			})
		case "GETVALUE":
			r := poRangeValor(c.props["value"])
			return true, r, nil
		case "SETMINDATE":
			a.setProp(c, "minDate", poDataISO(getArg(args, 0)))
		case "SETMAXDATE":
			a.setProp(c, "maxDate", poDataISO(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	case "datetimepicker":
		switch method {
		case "SETVALUE":
			a.setProp(c, "value", argTexto(args, 0, ""))
		case "GETVALUE":
			return true, poFromJSON(c.props["value"]), nil
		case "SETMINDATE":
			a.setProp(c, "minDate", argTexto(args, 0, ""))
		case "SETMAXDATE":
			a.setProp(c, "maxDate", argTexto(args, 0, ""))
		default:
			return false, advplrt.Nil, nil
		}
	case "timepicker":
		switch method {
		case "SETVALUE":
			a.setProp(c, "value", argTexto(args, 0, ""))
		case "GETVALUE":
			return true, poFromJSON(c.props["value"]), nil
		case "SETFORMAT":
			a.setProp(c, "format", advplrt.ToString(getArg(args, 0)))
		default:
			return false, advplrt.Nil, nil
		}
	default:
		return false, advplrt.Nil, nil
	}
	return true, advplrt.Nil, nil
}

// poRangeValor devolve o valor do range como JsonObject {dIni, dFim}
// com datas AdvPL (plano 3a.2: GetValue() -> {dIni,dFim}).
func poRangeValor(x any) advplrt.Value {
	obj := advplrt.NewObject("JsonObject", nil)
	ini, fim := "", ""
	switch r := x.(type) {
	case map[string]any:
		ini, _ = r["start"].(string)
		fim, _ = r["end"].(string)
	}
	obj.SetProp("dIni", poDataDeValor(advplrt.NewString(ini)))
	obj.SetProp("dFim", poDataDeValor(advplrt.NewString(fim)))
	return obj
}

// poRangeDoBrowser: o "change" do po-datepicker-range chega como
// {"start":"aaaa-mm-dd","end":"aaaa-mm-dd"} — vira JsonObject {dIni,dFim}
// para o OnChange. Demais tipos de data usam poValorDoBrowser.
func poRangeDoBrowser(val []byte) advplrt.Value {
	var m map[string]any
	if err := json.Unmarshal(val, &m); err != nil {
		return poRangeValor(nil)
	}
	return poRangeValor(m)
}
