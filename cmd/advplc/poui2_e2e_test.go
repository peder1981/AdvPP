//go:build !windows

package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/vm"
	"github.com/advpl/compiler/pkg/webui"
)

const e2eFase2 = `
User Function Main()
	Local oApp  := PoApp():New("E2E")
	Local oMenu := PoMenu():New(oApp)
	Local oP1   := PoPage():New(oApp, "Um")
	Local oP2   := PoPage():New(oApp, "Dois")
	Local oLs   := PoList():New(oP1)
	Local aIt   := {}
	Local oB    := PoButton():New(oP1, "Rodar", {|| oApp:Spawn("sh", {"-c", "echo um; echo dois"}, ;
		{|c| aAdd(aIt, {c}), oLs:SetItems(aIt)}, {|n| oApp:Notify("processo terminou: " + cValToChar(n))})})
	oMenu:AddItem("Dois", "", {|| oApp:Navigate(oP2)})
	oApp:Activate()
Return
`

type e2eOp2 struct {
	Op   string           `json:"op"`
	Menu json.RawMessage  `json:"menu"`
	Tree json.RawMessage  `json:"tree"`
	Ops  []map[string]any `json:"ops"`
}

// TestPoUIFase2PontaAPonta: menu, Spawn com saída ao vivo numa PoList,
// Notify no fim do processo e Navigate pelo menu, via HTTP real.
func TestPoUIFase2PontaAPonta(t *testing.T) {
	src := filepath.Join(t.TempDir(), "fase2e2e.prw")
	if err := os.WriteFile(src, []byte(e2eFase2), 0o644); err != nil {
		t.Fatal(err)
	}
	bc, err := loadAndCompile(src, &Options{})
	if err != nil {
		t.Fatal(err)
	}
	srv := webui.New("fase2e2e.prw", func(ui *webui.Provider, console *webui.OutWriter) error {
		v := vm.NewVM(bc, true)
		v.SetUIProvider(ui)
		v.SetOutputWriter(console)
		_, err := v.Run()
		return err
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/events?s=f2")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	ops := make(chan e2eOp2, 64)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var ev struct {
				Type string `json:"type"`
				Data e2eOp2 `json:"data"`
			}
			if l := sc.Text(); strings.HasPrefix(l, "data: ") &&
				json.Unmarshal([]byte(strings.TrimPrefix(l, "data: ")), &ev) == nil && ev.Type == "ui" {
				ops <- ev.Data
			}
		}
	}()
	proximo := func() e2eOp2 {
		select {
		case op := <-ops:
			return op
		case <-time.After(5 * time.Second):
			t.Fatal("operação de UI não chegou")
		}
		return e2eOp2{}
	}
	post := func(body string) {
		r, err := http.Post(ts.URL+"/ui?s=f2", "application/json", strings.NewReader(body))
		if err != nil || r.StatusCode != http.StatusNoContent {
			t.Fatalf("POST /ui %s: %v %v", body, err, r)
		}
		r.Body.Close()
	}

	// ids: menu 1, página Um 2, página Dois 3, lista 4, botão 5
	m := proximo()
	if m.Op != "mount" || string(m.Menu) == "null" || !strings.Contains(string(m.Tree), `"title":"Um"`) {
		t.Fatalf("mount: %+v", m)
	}
	post(`{"id":5,"event":"click","value":null}`)
	itens, terminou := 0, false
	for !terminou {
		op := proximo()
		for _, o := range op.Ops {
			if o["op"] == "patch" && o["id"] == float64(4) {
				if its, ok := o["props"].(map[string]any)["items"].([]any); ok {
					itens = len(its)
				}
			}
			if o["op"] == "notify" && o["text"] == "processo terminou: 0" {
				terminou = true
			}
		}
	}
	if itens != 2 {
		t.Errorf("a lista terminou com %d itens, esperado 2", itens)
	}
	post(`{"id":1,"event":"action","value":0}`)
	if m2 := proximo(); m2.Op != "mount" || !strings.Contains(string(m2.Tree), `"title":"Dois"`) {
		t.Errorf("Navigate: %+v", m2)
	}
}
