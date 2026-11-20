package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/vm"
	"github.com/advpl/compiler/pkg/webui"
)

type e2eNo struct {
	ID       int            `json:"id"`
	Type     string         `json:"type"`
	Props    map[string]any `json:"props"`
	Children []e2eNo        `json:"children"`
}

type e2eOp struct {
	Op   string `json:"op"`
	Tree *e2eNo `json:"tree"`
	Ops  []struct {
		ID    int            `json:"id"`
		Props map[string]any `json:"props"`
	} `json:"ops"`
}

// TestPoUIPontaAPonta: examples/poui/fase1.prw pelo mesmo caminho do
// `advplc serve` — mount no SSE, clique via POST /ui vira patch, e o timer
// gera patch sozinho.
func TestPoUIPontaAPonta(t *testing.T) {
	bc, err := loadAndCompile("../../examples/poui/fase1.prw", &Options{})
	if err != nil {
		t.Fatal(err)
	}
	srv := webui.New("fase1.prw", func(ui *webui.Provider, console *webui.OutWriter) error {
		v := vm.NewVM(bc, true)
		v.SetUIProvider(ui)
		v.SetOutputWriter(console)
		_, err := v.Run()
		return err
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events?s=e2e")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	ops := make(chan e2eOp, 64)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var ev struct {
				Type string `json:"type"`
				Data e2eOp  `json:"data"`
			}
			if l := sc.Text(); strings.HasPrefix(l, "data: ") &&
				json.Unmarshal([]byte(strings.TrimPrefix(l, "data: ")), &ev) == nil && ev.Type == "ui" {
				ops <- ev.Data
			}
		}
	}()

	var mount e2eOp
	select {
	case mount = <-ops:
	case <-time.After(5 * time.Second):
		t.Fatal("mount não chegou")
	}
	if mount.Op != "mount" || mount.Tree == nil || mount.Tree.Type != "page" {
		t.Fatalf("primeira operação: %+v", mount)
	}
	// Na fase1.prw: filhos = tag hora, input, table, tag mensagem, button
	ids := map[string]int{}
	tags := 0
	for _, c := range mount.Tree.Children {
		if c.Type == "tag" {
			tags++
			ids["tag"+strconv.Itoa(tags)] = c.ID
		} else {
			ids[c.Type] = c.ID
		}
	}
	hora, msg, btn := ids["tag1"], ids["tag2"], ids["button"]
	if hora == 0 || msg == 0 || btn == 0 {
		t.Fatalf("componentes não encontrados no mount: %v", ids)
	}

	// Clique com o nome vazio: a mensagem vira o aviso
	r, err := http.Post(ts.URL+"/ui?s=e2e", "application/json",
		strings.NewReader(`{"id":`+strconv.Itoa(btn)+`,"event":"click","value":null}`))
	if err != nil || r.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /ui: %v %v", err, r)
	}
	r.Body.Close()

	viuClique, viuTimer := false, false
	fim := time.After(3 * time.Second)
	for !(viuClique && viuTimer) {
		select {
		case op := <-ops:
			for _, p := range op.Ops {
				if p.ID == msg && p.Props["value"] == "Informe um nome antes de adicionar" {
					viuClique = true
				}
				if p.ID == hora {
					viuTimer = true
				}
			}
		case <-fim:
			t.Fatalf("patches: clique=%v timer=%v", viuClique, viuTimer)
		}
	}
}
