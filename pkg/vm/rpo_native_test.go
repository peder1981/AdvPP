package vm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestChkRpoChg(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["CHKRPOCHG"].Fn(nil)
	if err != nil {
		t.Fatalf("ChkRpoChg retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("ChkRpoChg() = %v, quer .T. (AdvPP nunca recarrega config em execução)", got)
	}
}

func TestGetAPOInfo(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ExemplosTDN.prw")
	if err := os.WriteFile(path, []byte("user function exemplo()\nreturn\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	mt := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	got, err := v.natives["GETAPOINFO"].Fn([]advplrt.Value{advplrt.NewString(path)})
	if err != nil {
		t.Fatalf("GetAPOInfo retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 5 {
		t.Fatalf("GetAPOInfo() = %v, quer array de 5 elementos", got)
	}
	if arr.Elements[0].(*advplrt.StringValue).Val != "ExemplosTDN.prw" {
		t.Errorf("aData[1] = %q, quer %q", arr.Elements[0].(*advplrt.StringValue).Val, "ExemplosTDN.prw")
	}
	if arr.Elements[1].(*advplrt.StringValue).Val != "AdvPL" {
		t.Errorf("aData[2] = %q, quer %q", arr.Elements[1].(*advplrt.StringValue).Val, "AdvPL")
	}
	dt, ok := arr.Elements[3].(*advplrt.DateValue)
	if !ok || !dt.Val.Equal(mt) {
		t.Errorf("aData[4] = %v, quer %v", arr.Elements[3], mt)
	}

	// Edge case: arquivo inexistente -> array vazio
	got2, err := v.natives["GETAPOINFO"].Fn([]advplrt.Value{advplrt.NewString(filepath.Join(tmpDir, "naoexiste.prw"))})
	if err != nil {
		t.Fatalf("GetAPOInfo(inexistente) retornou erro: %v", err)
	}
	arr2 := got2.(*advplrt.ArrayValue)
	if len(arr2.Elements) != 0 {
		t.Errorf("GetAPOInfo(inexistente) = %v, quer array vazio", got2)
	}
}

func TestGetApoRes(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "recurso.per")
	if err := os.WriteFile(path, []byte("conteudo do resource"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := v.natives["GETAPORES"].Fn([]advplrt.Value{advplrt.NewString(path)})
	if err != nil {
		t.Fatalf("GetApoRes retornou erro: %v", err)
	}
	if got.(*advplrt.StringValue).Val != "conteudo do resource" {
		t.Errorf("GetApoRes() = %q, quer %q", got.(*advplrt.StringValue).Val, "conteudo do resource")
	}

	// Edge case: resource inexistente -> ""
	got2, err := v.natives["GETAPORES"].Fn([]advplrt.Value{advplrt.NewString(filepath.Join(tmpDir, "naoexiste.per"))})
	if err != nil {
		t.Fatalf("GetApoRes(inexistente) retornou erro: %v", err)
	}
	if got2.(*advplrt.StringValue).Val != "" {
		t.Errorf("GetApoRes(inexistente) = %q, quer \"\"", got2.(*advplrt.StringValue).Val)
	}
}

func TestGetDependency(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	got, err := v.natives["GETDEPENDENCY"].Fn([]advplrt.Value{advplrt.NewString("dtappsrv-2117.prw")})
	if err != nil {
		t.Fatalf("GetDependency retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 0 {
		t.Errorf("GetDependency() = %v, quer array vazio (sem grafo de dependencia por fonte em AdvPP)", got)
	}

	// Edge case: sFonte vazio
	got2, err := v.natives["GETDEPENDENCY"].Fn([]advplrt.Value{advplrt.NewString("")})
	if err != nil {
		t.Fatalf("GetDependency('') retornou erro: %v", err)
	}
	if len(got2.(*advplrt.ArrayValue).Elements) != 0 {
		t.Errorf("GetDependency('') = %v, quer array vazio", got2)
	}
}

func TestGetFuncArray(t *testing.T) {
	bc := &compiler.Bytecode{
		Functions: compiler.FunctionMap{
			"U_TESTE001": {Name: "U_TESTE001"},
			"U_TESTE002": {Name: "U_TESTE002"},
			"OUTRAFUNC":  {Name: "OUTRAFUNC"},
		},
	}
	v := NewVM(bc, false)

	got, err := v.natives["GETFUNCARRAY"].Fn([]advplrt.Value{advplrt.NewString("U_TESTE*")})
	if err != nil {
		t.Fatalf("GetFuncArray retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 2 {
		t.Fatalf("GetFuncArray('U_TESTE*') = %v, quer 2 elementos", got)
	}
	if arr.Elements[0].(*advplrt.StringValue).Val != "U_TESTE001" || arr.Elements[1].(*advplrt.StringValue).Val != "U_TESTE002" {
		t.Errorf("GetFuncArray('U_TESTE*') = %v, quer [U_TESTE001, U_TESTE002] (ordenado)", got)
	}

	// Edge case: máscara sem casamento -> array vazio
	got2, err := v.natives["GETFUNCARRAY"].Fn([]advplrt.Value{advplrt.NewString("ZZZNAOEXISTE*")})
	if err != nil {
		t.Fatalf("GetFuncArray(sem casamento) retornou erro: %v", err)
	}
	if len(got2.(*advplrt.ArrayValue).Elements) != 0 {
		t.Errorf("GetFuncArray('ZZZNAOEXISTE*') = %v, quer array vazio", got2)
	}

	// Deve também casar natives registradas no VM (ex: RETIMGTYPE)
	got3, err := v.natives["GETFUNCARRAY"].Fn([]advplrt.Value{advplrt.NewString("RETIMGTYPE")})
	if err != nil {
		t.Fatalf("GetFuncArray('RETIMGTYPE') retornou erro: %v", err)
	}
	if len(got3.(*advplrt.ArrayValue).Elements) != 1 {
		t.Errorf("GetFuncArray('RETIMGTYPE') = %v, quer 1 elemento (native do proprio VM)", got3)
	}
}

func TestGetRpoLog(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	got, err := v.natives["GETRPOLOG"].Fn([]advplrt.Value{advplrt.NewNumber(1)})
	if err != nil {
		t.Fatalf("GetRpoLog retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 2 {
		t.Fatalf("GetRpoLog(1) = %v, quer array de 2 elementos [infoRPO, qtdPatches]", got)
	}
	qtd, ok := arr.Elements[1].(*advplrt.NumberValue)
	if !ok || qtd.Val != 0 {
		t.Errorf("GetRpoLog(1)[2] = %v, quer 0 (AdvPP nao tem sistema de patches)", arr.Elements[1])
	}

	// Edge case: sem parâmetro (default = RPO padrão)
	got2, err := v.natives["GETRPOLOG"].Fn(nil)
	if err != nil {
		t.Fatalf("GetRpoLog() retornou erro: %v", err)
	}
	if len(got2.(*advplrt.ArrayValue).Elements) != 2 {
		t.Errorf("GetRpoLog() = %v, quer array de 2 elementos", got2)
	}
}

func TestGetSrcArray(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	got, err := v.natives["GETSRCARRAY"].Fn([]advplrt.Value{advplrt.NewString("*.PRW")})
	if err != nil {
		t.Fatalf("GetSrcArray retornou erro: %v", err)
	}
	arr, ok := got.(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 0 {
		t.Errorf("GetSrcArray('*.PRW') = %v, quer array vazio (sem indice de fontes em AdvPP)", got)
	}

	// Edge case: nRPO fora do intervalo documentado -> array vazio
	got2, err := v.natives["GETSRCARRAY"].Fn([]advplrt.Value{advplrt.NewString("*.PRW"), advplrt.NewNumber(9)})
	if err != nil {
		t.Fatalf("GetSrcArray('*.PRW', 9) retornou erro: %v", err)
	}
	if len(got2.(*advplrt.ArrayValue).Elements) != 0 {
		t.Errorf("GetSrcArray('*.PRW', 9) = %v, quer array vazio", got2)
	}
}

func TestRetImgType(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	tmpDir := t.TempDir()

	bmpPath := filepath.Join(tmpDir, "img.bmp")
	if err := os.WriteFile(bmpPath, []byte("BM\x00\x00\x00\x00"), 0644); err != nil {
		t.Fatalf("WriteFile bmp: %v", err)
	}
	jpgPath := filepath.Join(tmpDir, "img.jpg")
	if err := os.WriteFile(jpgPath, []byte{0xFF, 0xD8, 0xFF, 0xE0}, 0644); err != nil {
		t.Fatalf("WriteFile jpg: %v", err)
	}

	got, err := v.natives["RETIMGTYPE"].Fn([]advplrt.Value{advplrt.NewString(bmpPath)})
	if err != nil {
		t.Fatalf("RetImgType(bmp) retornou erro: %v", err)
	}
	if got.(*advplrt.NumberValue).Val != 1 {
		t.Errorf("RetImgType(bmp) = %v, quer 1", got)
	}

	got2, err := v.natives["RETIMGTYPE"].Fn([]advplrt.Value{advplrt.NewString(jpgPath)})
	if err != nil {
		t.Fatalf("RetImgType(jpg) retornou erro: %v", err)
	}
	if got2.(*advplrt.NumberValue).Val != 2 {
		t.Errorf("RetImgType(jpg) = %v, quer 2", got2)
	}

	// Edge case: arquivo inexistente/nao identificado -> 0
	got3, err := v.natives["RETIMGTYPE"].Fn([]advplrt.Value{advplrt.NewString(filepath.Join(tmpDir, "naoexiste.jpg"))})
	if err != nil {
		t.Fatalf("RetImgType(inexistente) retornou erro: %v", err)
	}
	if got3.(*advplrt.NumberValue).Val != 0 {
		t.Errorf("RetImgType(inexistente) = %v, quer 0", got3)
	}
}
