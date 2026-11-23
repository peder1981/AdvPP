package vm

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/tools/shared"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// PouiAppUI é a extensão opcional de UIProvider que implementa o canal do
// app PO-UI (modo web, advplc serve). O pkg/webui.Provider a satisfaz; a
// VM nunca importa pkg/webui.
type PouiAppUI interface {
	// AbrePouiAppPorNome localiza o app em dirs (na ordem), registra o
	// canal na sessão e devolve a URL que a aba carrega.
	AbrePouiAppPorNome(nome string, dirs []string) (string, error)
	// PouiNext bloqueia até um jsToAdvpl, o fechamento do app ou o fim da
	// sessão. fechou=true encerra o FwCallApp.
	PouiNext(timeout time.Duration) (tipo, conteudo string, fechou bool)
	// PouiEnvia entrega um advpltojs à aba do app.
	PouiEnvia(tipo, conteudo string)
	// FechaPouiApp desregistra o app ao sair do FwCallApp.
	FechaPouiApp()
}

// tWebChannelState é o estado do objeto nativo oWebChannel entregue ao
// JsToAdvpl: AdvPLToJS(cType, cContent) vira um advpltojs no canal.
type tWebChannelState struct {
	ui PouiAppUI
}

// NativeCallMethod despacha os métodos do TWebChannel.
func (st *tWebChannelState) NativeCallMethod(v *VM, obj *advplrt.ObjectValue, method string, args []advplrt.Value) (advplrt.Value, error) {
	switch method {
	case "ADVPLTOJS":
		if st.ui == nil {
			return advplrt.Nil, nil
		}
		tipo := advplrt.ToString(getArg(args, 0))
		conteudo := advplValToString(getArg(args, 1))
		st.ui.PouiEnvia(tipo, conteudo)
		return advplrt.Nil, nil
	}
	return advplrt.Nil, fmt.Errorf("TWebChannel: método desconhecido %q", method)
}

// advplValToString serializa o conteúdo do AdvPLToJS: string passa direto,
// demais tipos viram JSON (o shim entrega protheusResponse como string).
func advplValToString(val advplrt.Value) string {
	switch x := val.(type) {
	case *advplrt.StringValue:
		return x.Val
	case nil, *advplrt.NilValue:
		return ""
	}
	if j, err := json.Marshal(poToJSON(val)); err == nil {
		return string(j)
	}
	return advplrt.ToString(val)
}

// callFuncSync executa uma função do bytecode de forma síncrona dentro de
// um native (mesmo padrão de callBlockSync, mas para funções comuns: os
// argumentos ocupam locals[0..n], sem o self do codeblock).
func (v *VM) callFuncSync(name string, args ...advplrt.Value) (advplrt.Value, error) {
	key, ok := v.bc.ResolveFunc(name)
	if !ok {
		return advplrt.Nil, fmt.Errorf("função %s não encontrada", name)
	}
	info := v.bc.Functions[key]
	locals := newLocals(info.NumLocals)
	for i := 0; i < len(args) && i < info.NumParams; i++ {
		locals[i] = args[i]
	}
	baseDepth := len(v.frames)
	savedCurrent := v.current
	frame := &CallFrame{
		FuncName:  key,
		Code:      v.bc.Code,
		IP:        info.Offset,
		Locals:    locals,
		StackBase: len(v.stack),
		ArgCount:  len(args),
	}
	v.frames = append(v.frames, frame)
	v.current = frame

	for len(v.frames) > baseDepth {
		if v.current == nil || v.current.IP >= len(v.current.Code) {
			break
		}
		instr := v.current.Code[v.current.IP]
		v.current.IP++
		if err := v.execute(instr); err != nil {
			v.unwindFramesTo(baseDepth)
			v.current = savedCurrent
			return advplrt.Nil, err
		}
		if instr.Op == compiler.OP_HALT {
			break
		}
	}
	if len(v.frames) > baseDepth {
		v.unwindFramesTo(baseDepth)
	}
	v.current = savedCurrent
	if len(v.stack) > frame.StackBase {
		return v.pop(), nil
	}
	return advplrt.Nil, nil
}

// resolveJsToAdvpl acha a Static Function JsToAdvpl do fonte indicado por
// cSource ("STATIC:<ARQ>:JSTOADVPL" em projeto; em fonte único a Static
// compila com o nome puro "JSTOADVPL"). Sem cSource, aceita qualquer
// JsToAdvpl do bytecode. Fallback final: U_JSTOADVPL.
func resolveJsToAdvpl(bc *compiler.Bytecode, cSource string) string {
	var candidatos []string
	if cSource != "" {
		base := strings.ToUpper(filepath.Base(cSource))
		for _, suf := range []string{"", ".PRW", ".PRX", ".TLPP"} {
			candidatos = append(candidatos, "STATIC:"+base+suf+":JSTOADVPL")
		}
	}
	for _, c := range candidatos {
		if _, ok := bc.Functions[c]; ok {
			return c
		}
	}
	if key, ok := bc.ResolveFunc("JSTOADVPL"); ok {
		return key
	}
	var estaticas []string
	for k := range bc.Functions {
		if strings.HasSuffix(strings.ToUpper(k), ":JSTOADVPL") {
			estaticas = append(estaticas, k)
		}
	}
	sort.Strings(estaticas)
	if len(estaticas) > 0 {
		return estaticas[0]
	}
	if key, ok := bc.ResolveFunc("U_JSTOADVPL"); ok {
		return key
	}
	return ""
}

// fwCallApp implementa FwCallApp(cApp, oOwner, oEngine, oChannel, cHost,
// cSource, ...): abre o app PO-UI numa aba da sessão web e despacha os
// jsToAdvpl para a Static Function JsToAdvpl do fonte (ou U_JSTOADVPL).
// Parâmetro 6 (cSource) seleciona o fonte que hospeda o JsToAdvpl;
// os demais (2-5, 7+) são aceitos e ignorados — o corpus 12.1.2510 só
// usa cApp (ex.: VEIA372 passa a assinatura completa comentada + ".t.").
func (v *VM) fwCallApp(args []advplrt.Value) (advplrt.Value, error) {
	ui, ok := v.uiProvider.(PouiAppUI)
	if !ok {
		return advplrt.Nil, fmt.Errorf("FwCallApp: requer o modo web (advplc serve)")
	}
	nome := strings.TrimSpace(advplrt.ToString(getArg(args, 0)))
	if nome == "" {
		return advplrt.Nil, fmt.Errorf("FwCallApp: nome do app vazio")
	}
	cSource := advplrt.ToString(getArg(args, 5))

	dirs := []string{}
	if v.sourceDir != "" {
		dirs = append(dirs, v.sourceDir)
	}
	if cfg, err := shared.LoadConfig(); err == nil {
		dirs = append(dirs, cfg.PouiAppsDirs...)
	}
	if _, err := ui.AbrePouiAppPorNome(nome, dirs); err != nil {
		return advplrt.Nil, fmt.Errorf("FwCallApp: %v (dirs: %s)", err, strings.Join(dirs, ", "))
	}
	defer ui.FechaPouiApp()

	channel := advplrt.NewObject("TWebChannel", nil)
	channel.Native = &tWebChannelState{ui: ui}
	jsName := resolveJsToAdvpl(v.bc, cSource)

	for {
		tipo, conteudo, fechou := ui.PouiNext(0)
		if fechou {
			return advplrt.Nil, nil
		}
		if jsName == "" {
			continue // sem callback: eventos descartados
		}
		if _, err := v.callFuncSync(jsName, channel, advplrt.NewString(tipo), advplrt.NewString(conteudo)); err != nil {
			v.writeOut(fmt.Sprintf("[FwCallApp] JsToAdvpl(%s): %v\n", tipo, err))
		}
	}
}
