package vm

import (
	"fmt"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// TWebChannel e TWebEngine — classes nativas do canal SmartClient↔Navegador
// do Protheus (assinaturas extraídas do dump binário do corpus 12.1.2510,
// _binary_class.prw):
//
//	class twebchannel
//	data lconnected as logical
//	data bjstoadvpl as codeblock
//	data nport as numeric
//	method new() / connect() / disconnect() / advpltojs(ccodetype, ccontent)
//
//	class twebengine from TControl
//	data curl as character
//	data bloadfinished as codeblock
//	data bdlstatus as codeblock
//	data clang as character
//	data bjsconmsg as codeblock
//	method new(ownd,nrow,ncol,nwidth,nheight,curl,nport) / navigate(curl) /
//	runjavascript() / goback() / goforward() / setasmain() / sethtml() /
//	clearcache() / reload()
//
// No AdvPP o "Navegador" é a aba do app PO-UI servida pelo advplc serve —
// não existe SmartClient desktop. O TWebChannel liga-se à sessão web via
// PouiAppUI (mesmo binding do canal interno do FwCallApp); o TWebEngine
// mantém o estado fiel (URL, codeblocks de evento) sem superfície visual
// própria — navigate/runjavascript/etc. são no-op documentados.

func init() {
	registerNativeBase("TWEBCHANNEL", newTWebChannelObject)
	registerNativeBase("TWEBENGINE", newTWebEngineObject)
}

// tWebChannelState é o estado nativo do TWebChannel. ui é o canal da sessão
// (PouiAppUI); nil em instâncias standalone — connect/advpltojs tentam
// resolver v.uiProvider na hora da chamada.
type tWebChannelState struct {
	ui PouiAppUI
}

func newTWebChannelObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TWebChannel", nil)
	obj.Native = &tWebChannelState{}
	obj.Props["LCONNECTED"] = advplrt.NewBool(false)
	obj.Props["NPORT"] = advplrt.NewNumber(0)
	return obj
}

// channelUI resolve a PouiAppUI do canal: a ligada na criação (FwCallApp)
// ou o provider da VM (sessão webui).
func (st *tWebChannelState) channelUI(v *VM) PouiAppUI {
	if st.ui != nil {
		return st.ui
	}
	if ui, ok := v.uiProvider.(PouiAppUI); ok {
		st.ui = ui
	}
	return st.ui
}

// NativeCallMethod implementa nativeBaseMethodHandler para TWebChannel.
// method chega em maiúsculas.
func (st *tWebChannelState) NativeCallMethod(v *VM, obj *advplrt.ObjectValue, method string, args []advplrt.Value) (advplrt.Value, error) {
	switch method {
	case "NEW":
		return obj, nil
	case "CONNECT":
		// Equivalente web do connect() SmartClient↔Navegador: só conecta
		// quando há sessão web (PouiAppUI) para carregar a página.
		obj.SetProp("LCONNECTED", advplrt.NewBool(st.channelUI(v) != nil))
		return obj, nil
	case "DISCONNECT":
		obj.SetProp("LCONNECTED", advplrt.NewBool(false))
		return obj, nil
	case "ADVPLTOJS":
		if ui := st.channelUI(v); ui != nil {
			ui.PouiEnvia(advplrt.ToString(getArg(args, 0)), advplValToString(getArg(args, 1)))
		}
		return advplrt.Nil, nil
	case "DESTROY":
		st.ui = nil
		obj.SetProp("LCONNECTED", advplrt.NewBool(false))
		return advplrt.Nil, nil
	}
	return advplrt.Nil, fmt.Errorf("TWebChannel: método desconhecido %q", method)
}

// tWebEngineState é o estado nativo do TWebEngine. Os dados públicos
// (curl, clang e os codeblocks de evento) vivem em obj.Props — visíveis ao
// AdvPL como propriedades, fiel ao `data ... as ...` do dump.
type tWebEngineState struct{}

func newTWebEngineObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TWebEngine", nil)
	obj.Native = &tWebEngineState{}
	obj.Props["CURL"] = advplrt.NewString("")
	obj.Props["CLANG"] = advplrt.NewString("")
	return obj
}

// NativeCallMethod implementa nativeBaseMethodHandler para TWebEngine.
// Sem superfície visual no modo web, os métodos de navegação mantêm o
// estado (URL/lang) e devolvem o próprio objeto; runjavascript não tem
// página para executar — no-op documentado.
func (st *tWebEngineState) NativeCallMethod(v *VM, obj *advplrt.ObjectValue, method string, args []advplrt.Value) (advplrt.Value, error) {
	switch method {
	case "NEW":
		// new(ownd, nrow, ncol, nwidth, nheight, curl, nport): posição 5 é
		// a URL inicial e 6 a porta do canal — o corpus chama com 5 args
		// (sem curl/nport), então ambos são opcionais.
		if s := advplrt.ToString(getArg(args, 5)); s != "" {
			obj.SetProp("CURL", advplrt.NewString(s))
		}
		if n := advplrt.ToFloat(getArg(args, 6)); n != 0 {
			obj.SetProp("NPORT", advplrt.NewNumber(n))
		}
		return obj, nil
	case "NAVIGATE":
		obj.SetProp("CURL", advplrt.NewString(advplrt.ToString(getArg(args, 0))))
		return obj, nil
	case "RUNJAVASCRIPT":
		// Sem página embutida no modo web — código JS não tem destino.
		return obj, nil
	case "SETHTML":
		obj.SetProp("HTML", advplrt.NewString(advplValToString(getArg(args, 0))))
		obj.SetProp("BASEURL", advplrt.NewString(advplrt.ToString(getArg(args, 1))))
		return obj, nil
	case "GOBACK", "GOFORWARD", "SETASMAIN", "CLEARCACHE", "RELOAD":
		return obj, nil
	}
	return advplrt.Nil, fmt.Errorf("TWebEngine: método desconhecido %q", method)
}
