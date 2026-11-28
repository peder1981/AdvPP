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

// tWebChannelStateOf extrai o estado nativo de um TWebChannel, aceitando
// as duas formas de obj.Native: o estado direto (newTWebChannelObject) e o
// *ObjectValue da fábrica registerNativeBase (instância via :New()).
func tWebChannelStateOf(o *advplrt.ObjectValue) *tWebChannelState {
	if st, ok := o.Native.(*tWebChannelState); ok {
		return st
	}
	if inner, ok := o.Native.(*advplrt.ObjectValue); ok {
		if st, ok := inner.Native.(*tWebChannelState); ok {
			return st
		}
	}
	return nil
}

// tWebEngineStateOf extrai o estado nativo de um TWebEngine, aceitando as
// mesmas duas formas de obj.Native do tWebChannelStateOf.
func tWebEngineStateOf(o *advplrt.ObjectValue) *tWebEngineState {
	if st, ok := o.Native.(*tWebEngineState); ok {
		return st
	}
	if inner, ok := o.Native.(*advplrt.ObjectValue); ok {
		if st, ok := inner.Native.(*tWebEngineState); ok {
			return st
		}
	}
	return nil
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
		uk := strings.ToUpper(k)
		if strings.HasSuffix(uk, ":JSTOADVPL") || strings.HasSuffix(uk, ".JSTOADVPL") {
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
// cSource, p7, p8, p9, lParam10) — assinatura completa documentada no
// corpus 12.1.2510 (VEIA372.prw). O corpus só usa cApp; quando o chamador
// passa um TWebChannel próprio no parâmetro 4 (oChannel) ele é reutilizado
// como canal do JsToAdvpl; oEngine/oOwner/cHost são objetos de desktop sem
// equivalente web e p7-p10 são aceitos e ignorados.
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
	appURL, err := ui.AbrePouiAppPorNome(nome, dirs)
	if err != nil {
		return advplrt.Nil, fmt.Errorf("FwCallApp: %v (dirs: %s)", err, strings.Join(dirs, ", "))
	}
	defer ui.FechaPouiApp()

	// oEngine (parâmetro 2): no desktop é o TWebEngine que hospeda a página
	// do app; no modo web a "página" é a aba aberta acima. Quando o caller
	// passa um TWebEngine, registramos a URL navegada no CURL dele — o
	// estado observável do engine permanece fiel (navigate posterior
	// sobrescreve, como no desktop).
	if o, ok := advplrt.Deref(getArg(args, 2)).(*advplrt.ObjectValue); ok {
		if tWebEngineStateOf(o) != nil && appURL != "" {
			o.SetProp("CURL", advplrt.NewString(appURL))
		}
	}

	// Canal do JsToAdvpl: o TWebChannel passado pelo chamador (parâmetro
	// oChannel) tem precedência — o código pode ter configurado
	// lConnected/nPort/bJsToAdvpl nele. Sem ele, criamos um ligado à
	// sessão, igual ao canal que o Protheus entrega ao JsToAdvpl.
	var channel *advplrt.ObjectValue
	if o, ok := advplrt.Deref(getArg(args, 3)).(*advplrt.ObjectValue); ok {
		// obj.Native de uma base registrada é o *ObjectValue da fábrica —
		// o estado real fica no .Native dele (mesmo desembrulho de
		// nativeBaseHandler).
		if st := tWebChannelStateOf(o); st != nil {
			channel = o
			if st.ui == nil {
				st.ui = ui
			}
		}
	}
	if channel == nil {
		channel = newTWebChannelObject()
		tWebChannelStateOf(channel).ui = ui
	}
	jsName := resolveJsToAdvpl(v.bc, cSource)

	for {
		tipo, conteudo, fechou := ui.PouiNext(0)
		if fechou {
			return advplrt.Nil, nil
		}
		// bJsToAdvpl tem precedência sobre a função JsToAdvpl: o corpus
		// registra o callback no canal ({|self,codeType,codeContent| ...}),
		// que é quem o SmartClient invocaria — quando setado, ele É o
		// handler (e tipicamente delega à JsToAdvpl por conta própria).
		if cb, ok := channel.Props["BJSTOADVPL"].(*advplrt.CodeBlockValue); ok && cb != nil {
			if _, err := v.callBlockSync(cb, channel, advplrt.NewString(tipo), advplrt.NewString(conteudo)); err != nil {
				v.writeOut(fmt.Sprintf("[FwCallApp] bJsToAdvpl(%s): %v\n", tipo, err))
			}
			continue
		}
		if jsName == "" {
			continue // sem callback: eventos descartados
		}
		if _, err := v.callFuncSync(jsName, channel, advplrt.NewString(tipo), advplrt.NewString(conteudo)); err != nil {
			v.writeOut(fmt.Sprintf("[FwCallApp] JsToAdvpl(%s): %v\n", tipo, err))
		}
	}
}

// fwOpenTab implementa FWOPENTAB(cUrl): abre cUrl numa aba nova do
// navegador e retorna imediatamente — sem canal poui e sem bloquear a
// sessão (o FwCallApp prende o programa no laço PouiNext até a aba
// fechar, o que deixa a página principal morta nesse intervalo). Serve
// para abrir páginas da mesma origem (o cliente valida: '/' mas não
// '//'). Fora do modo web (advplc run, testes) ignora.
func (v *VM) fwOpenTab(args []advplrt.Value) (advplrt.Value, error) {
	ui, ok := v.uiProvider.(interface{ OpenTab(string) })
	if !ok {
		return advplrt.Nil, nil
	}
	ui.OpenTab(strings.TrimSpace(advplrt.ToString(getArg(args, 0))))
	return advplrt.Nil, nil
}
