package vm

import (
	"strings"
	"testing"
)

func mapaTeste() map[string]MapaCampo {
	return map[string]MapaCampo{
		"filial":  {Prop: "filial", Coluna: "NNR_FILIAL", Tipo: "C", Filtra: true, Ordena: true},
		"codigo":  {Prop: "codigo", Coluna: "NNR_CODIGO", Tipo: "C", Filtra: true, Ordena: true},
		"descri":  {Prop: "descri", Coluna: "NNR_DESCRI", Tipo: "C", Filtra: true, Ordena: true},
		"estoque": {Prop: "estoque", Coluna: "NNR_ESTOQUE", Tipo: "N", Filtra: true, Ordena: false},
		"senha":   {Prop: "senha", Coluna: "NNR_SENHA", Tipo: "C", Filtra: false, Ordena: false},
	}
}

func TestFiltroSimples(t *testing.T) {
	sql, args, err := TraduzFiltro("filial eq '01' and codigo ge '000010'", mapaTeste())
	if err != nil {
		t.Fatalf("TraduzFiltro: %v", err)
	}
	if !strings.Contains(sql, "NNR_FILIAL = ?") || !strings.Contains(sql, "NNR_CODIGO >= ?") {
		t.Fatalf("sql = %q", sql)
	}
	if len(args) != 2 || args[0] != "01" || args[1] != "000010" {
		t.Fatalf("args = %v", args)
	}
	if strings.Contains(sql, "01") || strings.Contains(sql, "000010") {
		t.Fatalf("valor literal vazou no SQL: %q", sql)
	}
}

func TestFiltroFuncoes(t *testing.T) {
	sql, args, err := TraduzFiltro("contains(descri,'50%')", mapaTeste())
	if err != nil {
		t.Fatalf("TraduzFiltro: %v", err)
	}
	if !strings.Contains(sql, "NNR_DESCRI LIKE ? ESCAPE '\\'") {
		t.Fatalf("sql = %q", sql)
	}
	if len(args) != 1 || args[0] != "%50\\%%" {
		t.Fatalf("args = %v", args)
	}

	sql, args, err = TraduzFiltro("startswith(descri,'Ce') and endswith(codigo,'1')", mapaTeste())
	if err != nil {
		t.Fatalf("TraduzFiltro: %v", err)
	}
	if len(args) != 2 || args[0] != "Ce%" || args[1] != "%1" {
		t.Fatalf("args = %v", args)
	}
}

func TestFiltroHostil(t *testing.T) {
	mapa := mapaTeste()

	// Injeção dentro de string vira UM valor literal parametrizado.
	sql, args, err := TraduzFiltro("descri eq 'x'' or 1=1 --'", mapa)
	if err != nil {
		t.Fatalf("TraduzFiltro: %v", err)
	}
	if len(args) != 1 || args[0] != "x' or 1=1 --" {
		t.Fatalf("args = %v", args)
	}
	if strings.Contains(sql, "1=1") || strings.Contains(sql, "--") {
		t.Fatalf("injeção vazou no SQL: %q", sql)
	}

	// Aspas fecham a string; o resto é token inválido -> erro.
	if _, _, err := TraduzFiltro("descri eq 'x'; drop", mapa); err == nil {
		t.Fatal("filtro com ';' deveria dar erro")
	}

	// Campo existente mas não filtrável -> erro citando o nome.
	if _, _, err := TraduzFiltro("senha eq '1'", mapa); err == nil ||
		!strings.Contains(err.Error(), "senha") {
		t.Fatalf("erro = %v, quer menção a senha", err)
	}

	// Campo fora do mapa -> erro.
	if _, _, err := TraduzFiltro("hack eq '1'", mapa); err == nil {
		t.Fatal("campo não mapeado deveria dar erro")
	}

	// Parêntese aberto sem fechar -> erro.
	if _, _, err := TraduzFiltro("(descri eq 'x'", mapa); err == nil {
		t.Fatal("parêntese aberto deveria dar erro")
	}
}

func TestFiltroOperadoresENull(t *testing.T) {
	sql, args, err := TraduzFiltro("estoque gt 10 and filial ne null", mapaTeste())
	if err != nil {
		t.Fatalf("TraduzFiltro: %v", err)
	}
	if !strings.Contains(sql, "NNR_ESTOQUE > ?") || !strings.Contains(sql, "NNR_FILIAL IS NOT NULL") {
		t.Fatalf("sql = %q", sql)
	}
	if len(args) != 1 || args[0] != 10.0 {
		t.Fatalf("args = %v", args)
	}
}

func TestOrdemECampos(t *testing.T) {
	ord, err := TraduzOrdem("-descri,codigo", mapaTeste())
	if err != nil {
		t.Fatalf("TraduzOrdem: %v", err)
	}
	if ord != "NNR_DESCRI DESC, NNR_CODIGO ASC" {
		t.Fatalf("ordem = %q", ord)
	}

	// Campo não ordenável -> erro.
	if _, err := TraduzOrdem("-estoque", mapaTeste()); err == nil {
		t.Fatal("ordenar por campo não ordenável deveria dar erro")
	}
	// Campo não mapeado -> erro citando o nome.
	if _, err := TraduzOrdem("senha", mapaTeste()); err == nil ||
		!strings.Contains(err.Error(), "senha") {
		t.Fatalf("erro = %v", err)
	}

	cols, err := TraduzCampos("codigo,descri", mapaTeste())
	if err != nil {
		t.Fatalf("TraduzCampos: %v", err)
	}
	if len(cols) != 2 || cols[0] != "NNR_CODIGO" || cols[1] != "NNR_DESCRI" {
		t.Fatalf("cols = %v", cols)
	}

	// Vazio = todas as mapeadas.
	cols, err = TraduzCampos("", mapaTeste())
	if err != nil || len(cols) != 5 {
		t.Fatalf("cols = %v, err = %v", cols, err)
	}

	// Campo não mapeado -> erro.
	if _, err := TraduzCampos("codigo,xyz", mapaTeste()); err == nil {
		t.Fatal("campo xyz não mapeado deveria dar erro")
	}
}
