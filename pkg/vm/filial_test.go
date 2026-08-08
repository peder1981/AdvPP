package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/db"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestRpcSetEnvGravaFilialAtiva(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	_, err := v.natives["RPCSETENV"].Fn([]advplrt.Value{advplrt.NewString("010101")})
	if err != nil {
		t.Fatalf("RpcSetEnv retornou erro: %v", err)
	}
	if v.filialAtiva != "010101" {
		t.Errorf("filialAtiva = %q, quer %q", v.filialAtiva, "010101")
	}
}

func TestTruncarFilial(t *testing.T) {
	cases := []struct {
		filial string
		nivel  int
		want   string
	}{
		{"010101", 6, "010101"},
		{"010101", 4, "0101  "},
		{"010101", 2, "01    "},
		{"010101", 0, "      "},
		{"", 6, "      "},   // sessão sem filial definida ainda
		{"01", 6, "01    "}, // filial curta (nao deveria acontecer, mas nao pode panicar)
	}
	for _, c := range cases {
		got := truncarFilial(c.filial, c.nivel)
		if got != c.want {
			t.Errorf("truncarFilial(%q, %d) = %q, quer %q", c.filial, c.nivel, got, c.want)
		}
	}
}

func TestResolveFilialSemConfigUsaNivel6(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	v.filialAtiva = "010101"
	// nil SQLEngine (nenhum banco conectado) -- deve cair no default 6,
	// nao panicar.
	got := v.resolveFilial(nil, "UNI")
	if got != "010101" {
		t.Errorf("resolveFilial sem engine = %q, quer %q (default nivel 6)", got, "010101")
	}
}

func TestFWxFilialComNivelConfigurado(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/filial_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	if err := eng.Exec("CREATE TABLE X2_FILIAL_COMPART (TABELA TEXT PRIMARY KEY, NIVEL INTEGER NOT NULL)"); err != nil {
		t.Fatalf("create X2_FILIAL_COMPART: %v", err)
	}
	if err := eng.Exec("INSERT INTO X2_FILIAL_COMPART (TABELA, NIVEL) VALUES ('PLANO_CONTAS', 4)"); err != nil {
		t.Fatalf("insert config: %v", err)
	}

	v := NewVM(&compiler.Bytecode{}, false)
	v.SetDBEngine(eng)
	v.filialAtiva = "010101"

	got, err := v.natives["FWXFILIAL"].Fn([]advplrt.Value{advplrt.NewString("PLANO_CONTAS")})
	if err != nil {
		t.Fatalf("FWxFilial erro: %v", err)
	}
	if got.(*advplrt.StringValue).Val != "0101  " {
		t.Errorf("FWxFilial('PLANO_CONTAS') = %q, quer %q (nivel 4)", got.(*advplrt.StringValue).Val, "0101  ")
	}

	// Tabela sem config: cai no default 6.
	got2, _ := v.natives["FWXFILIAL"].Fn([]advplrt.Value{advplrt.NewString("UNI")})
	if got2.(*advplrt.StringValue).Val != "010101" {
		t.Errorf("FWxFilial('UNI') sem config = %q, quer %q (default nivel 6)", got2.(*advplrt.StringValue).Val, "010101")
	}
}

func TestResolveFilialComTableNaoExiste(t *testing.T) {
	tmpDir := t.TempDir()
	eng, err := db.NewSQLiteEngine(tmpDir + "/filial_missing_table_test.db")
	if err != nil {
		t.Fatalf("NewSQLiteEngine: %v", err)
	}
	defer eng.Close()

	// Propositalmente não criar a tabela X2_FILIAL_COMPART -- isso fará
	// QueryRows retornar um erro (table not found), exercitando o
	// caminho fail-closed do resolveFilial.

	v := NewVM(&compiler.Bytecode{}, false)
	v.SetDBEngine(eng)
	v.filialAtiva = "010101"

	got := v.resolveFilial(eng, "QUALQUERTABELA")
	if got != "010101" {
		t.Errorf("resolveFilial com table inexistente = %q, quer %q (default nivel 6, fail-closed)", got, "010101")
	}
}
