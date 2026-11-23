package vm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Onda 3a.4 — PoUpload: destino registrado no shared, URL com token,
// setters de extensão/tamanho/múltiplo e OnUpload(aArquivos).

func TestPoUploadMount(t *testing.T) {
	v, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oUp  := PoUpload():New(oPg, "Anexo", "uploads")
	oUp:SetExtensions({".png", ".jpg"})
	oUp:SetMaxSizeMB(5)
	oUp:SetMultiple(.T.)
Return
`)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tree noMount
	}
	if err := json.Unmarshal(v.poApp.mountJSON(), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tree.Children) != 1 {
		t.Fatalf("filhos = %d", len(m.Tree.Children))
	}
	up := m.Tree.Children[0]
	if up.Type != "upload" {
		t.Fatalf("type = %s", up.Type)
	}
	url, _ := up.Props["url"].(string)
	if !strings.HasPrefix(url, "/poui-upload?token=") || len(url) < 40 {
		t.Fatalf("url = %q", url)
	}
	if up.Props["multiple"] != true {
		t.Fatalf("multiple = %v", up.Props["multiple"])
	}
	rest, _ := up.Props["restrictions"].(map[string]any)
	if rest == nil || rest["maxFileSize"] != float64(5*1024*1024) {
		t.Fatalf("restrictions = %v", up.Props["restrictions"])
	}
	exts, _ := rest["allowedExtensions"].([]any)
	if len(exts) != 2 {
		t.Fatalf("allowedExtensions = %v", rest["allowedExtensions"])
	}
}

func TestPoUploadEvento(t *testing.T) {
	// ids: página 1, upload 2
	f := newFakeUI(
		fakeEv{2, "upload", []byte(`[{"cNome":"a.txt","cCaminhoGravado":"/x/a.txt","nBytes":3}]`)},
	)
	out, errc, done := rodaComUI(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	Local oPg  := PoPage():New(oApp, "P")
	Local oUp  := PoUpload():New(oPg, "Anexo", "uploads")
	oUp:OnUpload({|o, a| ConOut("N=" + cValToChar(Len(a)) + "|" + a[1]["cNome"] + "|" + cValToChar(a[1]["nBytes"])), oApp:Close()})
	oApp:Activate()
Return
`, f)
	esperaFim(t, done, errc)
	if !strings.Contains(out.String(), "N=1|a.txt|3") {
		t.Errorf("saída = %q", out.String())
	}
}

func TestPoUploadPaiInvalido(t *testing.T) {
	_, _, err := rodaPrg(t, `
User Function Main()
	Local oApp := PoApp():New("T")
	PoUpload():New(oApp, "x", "uploads")
Return
`)
	if err == nil || !strings.Contains(err.Error(), "pai inválido") {
		t.Errorf("erro = %v", err)
	}
}
