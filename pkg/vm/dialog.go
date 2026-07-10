package vm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// DialogUI é a extensão opcional de UIProvider para renderizar um MSDIALOG
// legado (fase 4 do renderer web). Recebe o spec em JSON, bloqueia até o
// usuário agir e devolve a ação (também JSON).
type DialogUI interface {
	Dialog(spec []byte) []byte
}

// dlgControl é um controle declarado via `@ x,y SAY/GET/BUTTON` — a posição
// original vira heurística de grade (ordenação por linha y, depois coluna x).
type dlgControl struct {
	Kind    string  `json:"kind"` // say | get | button
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Name    string  `json:"name,omitempty"`    // variável do GET
	Text    string  `json:"text,omitempty"`    // texto do SAY / rótulo do BUTTON
	Value   string  `json:"value,omitempty"`   // valor atual do GET
	Picture string  `json:"picture,omitempty"` // PICTURE do GET
	Index   int     `json:"index"`             // índice do BUTTON

	valType string        // C | N | L — tipo original do GET para converter de volta
	action  advplrt.Value // codeblock do ACTION (button)
	frame   *CallFrame    // frame dono da variável do GET (writeback)
}

// webDialog é o estado Go de um MSDIALOG (campo Native do objeto).
type webDialog struct {
	title    string
	controls []*dlgControl
	onInit   advplrt.Value
}

type dialogSpec struct {
	Title   string          `json:"title"`
	Rows    [][]*dlgControl `json:"rows"`
	Buttons []*dlgControl   `json:"buttons"`
}

type dialogAction struct {
	Action string            `json:"action"` // button | close
	Index  int               `json:"index"`
	Data   map[string]string `json:"data"`
}

// registerDialogNatives adiciona as funções dessugarizadas do DSL de diálogo
// (DEFINE MSDIALOG / @ x,y SAY|GET|BUTTON / ACTIVATE MSDIALOG).
func (v *VM) registerDialogNatives(natives map[string]func([]advplrt.Value) (advplrt.Value, error)) {
	natives["MSDIALOG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// MSDIALOG(x1, y1, x2, y2, title) — coordenadas ignoradas na web
		dlg := &webDialog{title: advplrt.ToString(getArg(args, 4))}
		v.curDialog = dlg
		obj := advplrt.NewObject("MsDialog", nil)
		obj.Native = dlg
		return obj, nil
	}

	natives["AT_SAY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if dlg := v.curDialog; dlg != nil {
			dlg.controls = append(dlg.controls, &dlgControl{
				Kind: "say",
				Y:    advplrt.ToFloat(getArg(args, 0)), // @ linha,coluna: 1º é a linha
				X:    advplrt.ToFloat(getArg(args, 1)),
				Text: advplrt.ToString(getArg(args, 2)),
			})
		}
		return advplrt.Nil, nil
	}

	natives["AT_GET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if dlg := v.curDialog; dlg != nil {
			val := getArg(args, 3)
			ctl := &dlgControl{
				Kind:    "get",
				Y:       advplrt.ToFloat(getArg(args, 0)), // @ linha,coluna
				X:       advplrt.ToFloat(getArg(args, 1)),
				Name:    advplrt.ToString(getArg(args, 2)),
				Value:   strings.TrimRight(advplrt.ToString(val), " "),
				Picture: dialogClause(args, 4, "PICTURE"),
				valType: valueTypeOf(val),
				frame:   v.current,
			}
			dlg.controls = append(dlg.controls, ctl)
		}
		return advplrt.Nil, nil
	}

	natives["AT_BUTTON"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if dlg := v.curDialog; dlg != nil {
			ctl := &dlgControl{
				Kind:   "button",
				Y:      advplrt.ToFloat(getArg(args, 0)), // @ linha,coluna
				X:      advplrt.ToFloat(getArg(args, 1)),
				Text:   advplrt.ToString(getArg(args, 2)),
				action: dialogClauseValue(args, 3, "ACTION"),
			}
			dlg.controls = append(dlg.controls, ctl)
		}
		return advplrt.Nil, nil
	}

	natives["AT_BOX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.Nil, nil // decorativo: sem equivalente na grade web
	}

	natives["ACTIVATE_MSDIALOG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dlg := v.curDialog
		if obj, ok := getArg(args, 0).(*advplrt.ObjectValue); ok {
			if d, ok := obj.Native.(*webDialog); ok {
				dlg = d
			}
		}
		if dlg == nil {
			return advplrt.Nil, fmt.Errorf("ACTIVATE MSDIALOG sem DEFINE MSDIALOG anterior")
		}
		if len(args) > 1 {
			dlg.onInit = args[1]
		}
		err := v.runDialog(dlg)
		v.curDialog = nil
		return advplrt.Nil, err
	}
}

// callMsDialogMethod atende os métodos usuais do objeto de diálogo.
// args não é usado: nenhum destes métodos recebe parâmetro (ACTIVATE/END/
// CLOSE/DEACTIVATE/NEW) — mantido só para uniformidade de assinatura com
// os demais dispatchers de classe nativa (callFormBrowseMethod etc.).
func (v *VM) callMsDialogMethod(obj *advplrt.ObjectValue, method string, _ []advplrt.Value) error {
	dlg, ok := obj.Native.(*webDialog)
	if !ok {
		return fmt.Errorf("MsDialog: objeto sem estado interno")
	}
	switch method {
	case "ACTIVATE":
		if err := v.runDialog(dlg); err != nil {
			return err
		}
		v.curDialog = nil
		v.push(advplrt.Nil)
	case "END", "CLOSE", "DEACTIVATE", "NEW":
		v.push(advplrt.Nil)
	default:
		return fmt.Errorf("unknown method %s on MsDialog", method)
	}
	return nil
}

// runDialog envia o diálogo à UI, espera a ação do usuário, grava os GETs de
// volta nas variáveis e executa o ACTION do botão clicado.
func (v *VM) runDialog(dlg *webDialog) error {
	ui, ok := v.uiProvider.(DialogUI)
	if !ok {
		return fmt.Errorf("MSDIALOG: requer o modo web (advplc serve)")
	}
	if dlg.onInit != nil {
		if _, isBlock := dlg.onInit.(*advplrt.CodeBlockValue); isBlock {
			if _, err := v.evalBlock(dlg.onInit); err != nil {
				fmt.Printf("MSDIALOG ON INIT: %v\n", err)
			}
		}
	}

	spec := buildDialogSpec(dlg)
	data, _ := json.Marshal(spec)

	var act dialogAction
	if err := json.Unmarshal(ui.Dialog(data), &act); err != nil {
		return nil // sessão encerrada: fecha o diálogo
	}

	// writeback: valor digitado volta para a variável local do GET
	for _, ctl := range dlg.controls {
		if ctl.Kind != "get" || ctl.Name == "" || ctl.frame == nil {
			continue
		}
		raw, ok := act.Data[ctl.Name]
		if !ok {
			continue
		}
		if slot, ok := v.localSlot(ctl.frame, ctl.Name); ok {
			ctl.frame.Locals[slot] = convertDialogValue(raw, ctl.valType)
		}
	}

	// ponytail: todo clique de botão fecha o diálogo após rodar o ACTION —
	// os codeblocks deste runtime não capturam locais (oDlg:End() etc.),
	// então o ciclo continuar-aberto viria sem ter como ser encerrado
	if act.Action == "button" {
		for _, ctl := range dlg.controls {
			if ctl.Kind == "button" && ctl.Index == act.Index && ctl.action != nil {
				if _, isBlock := ctl.action.(*advplrt.CodeBlockValue); isBlock {
					if _, err := v.evalBlock(ctl.action); err != nil {
						fmt.Printf("MSDIALOG ACTION: %v\n", err)
					}
				}
			}
		}
	}
	return nil
}

// buildDialogSpec aplica a heurística de grade: ordena por (y, x) e agrupa
// controles com y próximo na mesma linha; botões vão para o rodapé.
func buildDialogSpec(dlg *webDialog) *dialogSpec {
	spec := &dialogSpec{Title: dlg.title, Rows: [][]*dlgControl{}, Buttons: []*dlgControl{}}

	fields := []*dlgControl{}
	for _, ctl := range dlg.controls {
		if ctl.Kind == "button" {
			ctl.Index = len(spec.Buttons)
			spec.Buttons = append(spec.Buttons, ctl)
		} else {
			fields = append(fields, ctl)
		}
	}
	sort.SliceStable(fields, func(i, j int) bool {
		if fields[i].Y != fields[j].Y {
			return fields[i].Y < fields[j].Y
		}
		return fields[i].X < fields[j].X
	})

	const rowTolerance = 8.0 // pixels: SAY e GET da mesma linha raramente têm y idêntico
	var row []*dlgControl
	lastY := -1e9
	for _, ctl := range fields {
		if len(row) > 0 && ctl.Y-lastY > rowTolerance {
			spec.Rows = append(spec.Rows, row)
			row = nil
		}
		row = append(row, ctl)
		lastY = ctl.Y
	}
	if len(row) > 0 {
		spec.Rows = append(spec.Rows, row)
	}
	return spec
}

// localSlot resolve o slot de uma variável local pelo nome (case-insensitive).
func (v *VM) localSlot(frame *CallFrame, name string) (int, bool) {
	info, ok := v.bc.Functions[frame.FuncName]
	if !ok || info.LocalNames == nil {
		return 0, false
	}
	if slot, ok := info.LocalNames[name]; ok {
		return slot, true
	}
	for n, slot := range info.LocalNames {
		if strings.EqualFold(n, name) {
			return slot, true
		}
	}
	return 0, false
}

func valueTypeOf(val advplrt.Value) string {
	switch val.(type) {
	case *advplrt.NumberValue:
		return "N"
	case *advplrt.BoolValue:
		return "L"
	default:
		return "C"
	}
}

func convertDialogValue(raw, valType string) advplrt.Value {
	switch valType {
	case "N":
		n, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		return advplrt.NewNumber(n)
	case "L":
		return advplrt.NewBool(raw == "true" || strings.EqualFold(raw, ".T."))
	default:
		return advplrt.NewString(raw)
	}
}

// dialogClause busca o valor string de uma cláusula etiquetada
// ("PICTURE", "@!", ...) a partir da posição from dos args.
func dialogClause(args []advplrt.Value, from int, clause string) string {
	if val := dialogClauseValue(args, from, clause); val != nil {
		return advplrt.ToString(val)
	}
	return ""
}

func dialogClauseValue(args []advplrt.Value, from int, clause string) advplrt.Value {
	for i := from; i < len(args)-1; i++ {
		if s, ok := args[i].(*advplrt.StringValue); ok && strings.EqualFold(s.Val, clause) {
			return args[i+1]
		}
	}
	return nil
}
