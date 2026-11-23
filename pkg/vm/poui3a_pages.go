package vm

import (
	"fmt"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// Onda 3a.5 — templates estáticos: PoPageList/PoPageEdit/PoPageDetail.
// São páginas do PoApp (pai = PoApp, como PoPage): Navigate aceita e
// aceitam filhos como PoPage.

// poPageTemplate: tipos de página de template.
func poPageTemplate(typ string) bool {
	switch typ {
	case "pagelist", "pageedit", "pagedetail":
		return true
	}
	return false
}

// po3aPageMethod despacha métodos/eventos dos templates estáticos.
func (v *VM) po3aPageMethod(c *poComp, method string, args []advplrt.Value) (bool, advplrt.Value, error) {
	if !poPageTemplate(c.typ) {
		return false, advplrt.Nil, nil
	}
	a := c.app
	switch method {
	case "ADDACTION":
		// PoPageList:AddAction(cLabel, bAcao)
		if c.typ != "pagelist" {
			return true, advplrt.Nil, fmt.Errorf("AddAction: só em PoPageList/PoPage")
		}
		acts := c.props["actions"].([]map[string]any)
		n := len(acts)
		c.handlers[fmt.Sprintf("action:%d", n)] = getArg(args, 1)
		a.setProp(c, "actions", append(acts, map[string]any{"label": advplrt.ToString(getArg(args, 0)), "index": n}))
	case "SETFILTER":
		// PoPageList:SetFilter(cPlaceholder) — placeholder da busca rápida.
		if c.typ != "pagelist" {
			return true, advplrt.Nil, fmt.Errorf("SetFilter: só em PoPageList")
		}
		a.setProp(c, "filterPlaceholder", advplrt.ToString(getArg(args, 0)))
	case "SETDISCLAIMERS":
		if c.typ != "pagelist" {
			return true, advplrt.Nil, fmt.Errorf("SetDisclaimers: só em PoPageList")
		}
		a.setProp(c, "disclaimers", poToJSON(getArg(args, 0)))
	case "ONFILTER":
		c.handlers["filter"] = getArg(args, 0)
	case "ONADVANCEDSEARCH":
		c.handlers["advancedsearch"] = getArg(args, 0)
	case "ONSAVE":
		c.handlers["save"] = getArg(args, 0)
	case "ONSAVENEW":
		c.handlers["savenew"] = getArg(args, 0)
	case "ONCANCEL":
		c.handlers["cancel"] = getArg(args, 0)
	case "ONEDIT":
		c.handlers["edit"] = getArg(args, 0)
	case "ONREMOVE":
		c.handlers["remove"] = getArg(args, 0)
	case "ONBACK":
		c.handlers["back"] = getArg(args, 0)
	default:
		return false, advplrt.Nil, nil
	}
	return true, advplrt.Nil, nil
}
