package db

import (
	"os"
	"testing"
)

// TestDbapiBridgeCodec valida o codec de frames sem bridge real:
// encode/decode simétrico de campos, incluindo NULL.
func TestDbapiBridgeCodec(t *testing.T) {
	fields := []dbapiField{
		dbapiStr("127.0.0.1"),
		{isNull: true},
		dbapiStr("SELECT 1"),
	}
	payload := []byte{dbapiOpConnect}
	payload = dbapiAppendU32(payload, uint32(len(fields)))
	for _, f := range fields {
		payload = dbapiAppendField(payload, f)
	}

	// decodifica como o bridge faria
	if payload[0] != dbapiOpConnect {
		t.Fatalf("opcode errado: %d", payload[0])
	}
	p := &dbapiParser{buf: payload[1:]}
	nf, err := p.u32()
	if err != nil || nf != 3 {
		t.Fatalf("nfields: %d err=%v", nf, err)
	}
	fs, err := p.fields(nf)
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	if dbapiFieldStr(fs[0]) != "127.0.0.1" {
		t.Fatalf("field0 = %q", dbapiFieldStr(fs[0]))
	}
	if !fs[1].isNull {
		t.Fatal("field1 deveria ser NULL")
	}
	if dbapiFieldStr(fs[2]) != "SELECT 1" {
		t.Fatalf("field2 = %q", dbapiFieldStr(fs[2]))
	}
}

// TestDbapiSQLEngineIntegration é o teste E2E real: bridge + dbaccess64 +
// Postgres. Só roda quando ADVPP_DBAPI_IT=1 e o ambiente está de pé:
//
//	ADVPP_DBAPI_BRIDGE=127.0.0.1:17890  (advpp-dbapi-bridge rodando)
//	DbConnection cfg: Host=127.0.0.1 Port=7890 Service=POSTGRES/p1212510
func TestDbapiSQLEngineIntegration(t *testing.T) {
	if os.Getenv("ADVPP_DBAPI_IT") != "1" {
		t.Skip("defina ADVPP_DBAPI_IT=1 com bridge+dbaccess no ar")
	}
	cfg := ConnConfig{
		Host:    "127.0.0.1",
		Port:    7890,
		Service: "POSTGRES/p1212510",
	}
	eng, err := NewDbapiEngineFrom(cfg)
	if err != nil {
		t.Fatalf("NewDbapiEngineFrom: %v", err)
	}
	defer eng.Close()

	// read path: SelectArea sobre tabela real do Protheus
	if err := eng.SelectArea("SX2990"); err != nil {
		t.Fatalf("SelectArea SX2990: %v", err)
	}
	if eng.RecCount() == 0 {
		t.Fatal("SX2990 vazio — esperado dicionário populado")
	}
	t.Logf("SX2990: %d registros, %d colunas", eng.RecCount(), len(eng.columns))

	if err := eng.GoTop(); err != nil {
		t.Fatalf("GoTop: %v", err)
	}
	v, err := eng.FieldGet("X2_NOME")
	if err != nil {
		t.Fatalf("FieldGet: %v", err)
	}
	if v.String() == "" {
		t.Fatal("X2_NOME vazio no primeiro registro")
	}
	t.Logf("primeiro X2_NOME: %q", v.String())

	// navegação
	if err := eng.Skip(1); err != nil {
		t.Fatalf("Skip: %v", err)
	}
	if eng.RecNo() != 2 {
		t.Fatalf("RecNo após Skip(1): %d, esperado 2", eng.RecNo())
	}

	// SQLEngine.QueryRows
	rows, err := eng.QueryRows("SELECT X2_CHAVE, X2_NOME FROM SX2990 LIMIT 2")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("QueryRows retornou %d linhas, esperado 2", len(rows))
	}
	for _, r := range rows {
		if r["X2_CHAVE"] == "" {
			t.Fatal("X2_CHAVE vazio no resultado")
		}
	}
	t.Logf("QueryRows OK: %d linhas", len(rows))

	// erro SQL deve voltar como erro Go, não crash
	if _, err := eng.QueryRows("SELECT * FROM TABELA_INEXISTENTE_XX"); err == nil {
		t.Fatal("QueryRows em tabela inexistente deveria falhar")
	} else {
		t.Logf("erro esperado capturado: %v", err)
	}

	// engine continua viva após o erro
	if _, err := eng.QueryRows("SELECT 1 AS UM"); err != nil {
		t.Fatalf("engine morreu após erro anterior: %v", err)
	}
}
