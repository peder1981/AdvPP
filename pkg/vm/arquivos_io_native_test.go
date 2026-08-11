package vm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// arquivosIO é o driver de teste das 29 natives de Manipulação de arquivos.
// As natives ainda não estão registradas em natives.go, então o teste registra
// via registerArquivosioNatives num mapa local (conforme spec de teste).
type arquivosIO struct {
	t       *testing.T
	v       *VM
	natives map[string]func(args []advplrt.Value) (advplrt.Value, error)
}

func newArquivosIO(t *testing.T) *arquivosIO {
	t.Helper()
	v := NewVM(&compiler.Bytecode{}, false)
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){}
	v.registerArquivosioNatives(natives)
	return &arquivosIO{t: t, v: v, natives: natives}
}

func (a *arquivosIO) call(name string, args ...advplrt.Value) advplrt.Value {
	a.t.Helper()
	fn, ok := a.natives[name]
	if !ok {
		a.t.Fatalf("native %s não registrada", name)
	}
	res, err := fn(args)
	if err != nil {
		a.t.Fatalf("%s retornou erro Go: %v", name, err)
	}
	return res
}

func (a *arquivosIO) callBool(name string, args ...advplrt.Value) bool {
	a.t.Helper()
	b, ok := a.call(name, args...).(*advplrt.BoolValue)
	if !ok {
		a.t.Fatalf("%s não retornou BoolValue", name)
	}
	return b.Val
}

func (a *arquivosIO) callNum(name string, args ...advplrt.Value) float64 {
	a.t.Helper()
	n, ok := a.call(name, args...).(*advplrt.NumberValue)
	if !ok {
		a.t.Fatalf("%s não retornou NumberValue", name)
	}
	return n.Val
}

func (a *arquivosIO) callStr(name string, args ...advplrt.Value) string {
	a.t.Helper()
	s, ok := a.call(name, args...).(*advplrt.StringValue)
	if !ok {
		a.t.Fatalf("%s não retornou StringValue", name)
	}
	return s.Val
}

func (a *arquivosIO) callArray(name string, args ...advplrt.Value) *advplrt.ArrayValue {
	a.t.Helper()
	arr, ok := a.call(name, args...).(*advplrt.ArrayValue)
	if !ok {
		a.t.Fatalf("%s não retornou ArrayValue", name)
	}
	return arr
}

func (a *arquivosIO) writeFile(path, content string) {
	a.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		a.t.Fatalf("falha ao criar %s: %v", path, err)
	}
}

func (a *arquivosIO) readFile(path string) string {
	a.t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		a.t.Fatalf("falha ao ler %s: %v", path, err)
	}
	return string(data)
}

func (a *arquivosIO) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TestADIR lista arquivos criados em diretório temporário e valida o formato
// do array de nomes (incluindo os aliases "." e ".." quando aAtributos é dado).
func TestADIR(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	alpha := filepath.Join(dir, "alpha.txt")
	beta := filepath.Join(dir, "beta.bin")
	a.writeFile(alpha, "alpha")
	a.writeFile(beta, "beta")

	spec := filepath.Join(dir, "*.*")

	// Sem aAtributos e com lChangeCase=false: só os arquivos, case preservado.
	arr := a.callArray("ADIR", advplrt.NewString(spec), advplrt.Nil, advplrt.Nil,
		advplrt.Nil, advplrt.Nil, advplrt.Nil, advplrt.NewBool(false))
	names := make([]string, 0, len(arr.Elements))
	for _, e := range arr.Elements {
		names = append(names, e.(*advplrt.StringValue).Val)
	}
	if !containsStr(names, "alpha.txt") || !containsStr(names, "beta.bin") {
		t.Errorf("ADIR sem atributos: esperava alpha.txt/beta.bin, obteve %v", names)
	}

	// Com aAtributos informado: inclui "." e ".." e converte para minúsculas.
	arr = a.callArray("ADIR", advplrt.NewString(spec), advplrt.Nil, advplrt.Nil,
		advplrt.Nil, advplrt.Nil, advplrt.NewArray(nil), advplrt.NewBool(true))
	names = make([]string, 0, len(arr.Elements))
	for _, e := range arr.Elements {
		names = append(names, e.(*advplrt.StringValue).Val)
	}
	if len(names) < 4 {
		t.Fatalf("ADIR com atributos: esperava . e .. + arquivos, obteve %v", names)
	}
	if names[0] != "." || names[1] != ".." {
		t.Errorf("ADIR com atributos: primeiros elementos deveriam ser '.'/'..', obteve %v", names[:2])
	}
}

func TestCGetFile(t *testing.T) {
	a := newArquivosIO(t)
	// Sem GUI: deve retornar "" (cancelado) sem panic — assert apenas o tipo.
	if s := a.callStr("CGETFILE", advplrt.NewString("*.prw"), advplrt.NewString("Selecione")); s != "" {
		t.Errorf("CGETFILE sem GUI deveria retornar \"\", obteve %q", s)
	}
}

func TestTFileDialog(t *testing.T) {
	a := newArquivosIO(t)
	// Sem GUI: deve retornar "" sem panic.
	if s := a.callStr("TFILEDIALOG", advplrt.NewString("*.txt"), advplrt.NewString("Abrir")); s != "" {
		t.Errorf("TFILEDIALOG sem GUI deveria retornar \"\", obteve %q", s)
	}
}

func TestChmod(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "chmod.txt")
	a.writeFile(p, "conteudo")

	if !a.callBool("CHMOD", advplrt.NewString(p), advplrt.NewNumber(600)) {
		t.Error("CHMOD para 0600 deveria retornar .T.")
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat após CHMOD: %v", err)
	}
	if runtime.GOOS == "windows" {
		// No Windows o chmod só alterna o bit read-only: qualquer permissão
		// com bit de escrita (0200) deixa o arquivo gravável; a checagem de
		// valor octal exato não se aplica.
		if info.Mode().Perm()&0o200 == 0 {
			t.Errorf("permissão esperada gravável, obteve %v", info.Mode().Perm())
		}
	} else if info.Mode().Perm() != 0o600 {
		t.Errorf("permissão esperada 0600, obteve %v", info.Mode().Perm())
	}
	// Arquivo continua legível após a mudança de permissão.
	if got := a.readFile(p); got != "conteudo" {
		t.Errorf("conteúdo após CHMOD deveria ser 'conteudo', obteve %q", got)
	}

	if a.callBool("CHMOD", advplrt.NewString(filepath.Join(dir, "nao_existe.txt")), advplrt.NewNumber(644)) {
		t.Error("CHMOD em arquivo inexistente deveria retornar .F.")
	}
}

func TestExistDir(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()

	if !a.callBool("EXISTDIR", advplrt.NewString(dir)) {
		t.Error("EXISTDIR em diretório existente deveria retornar .T.")
	}
	if a.callBool("EXISTDIR", advplrt.NewString(filepath.Join(dir, "nao_existe"))) {
		t.Error("EXISTDIR em diretório inexistente deveria retornar .F.")
	}
	// Arquivo não é diretório.
	f := filepath.Join(dir, "arquivo.txt")
	a.writeFile(f, "x")
	if a.callBool("EXISTDIR", advplrt.NewString(f)) {
		t.Error("EXISTDIR em arquivo comum deveria retornar .F.")
	}
}

func TestDirRemove(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()

	empty := filepath.Join(dir, "vazio")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !a.callBool("DIRREMOVE", advplrt.NewString(empty)) {
		t.Error("DIRREMOVE em diretório vazio deveria retornar .T.")
	}
	if a.fileExists(empty) {
		t.Error("diretório deveria ter sido removido")
	}

	// Diretório não-vazio: os.Remove falha (spec exige vazio).
	nonEmpty := filepath.Join(dir, "cheio")
	if err := os.Mkdir(nonEmpty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	a.writeFile(filepath.Join(nonEmpty, "dentro.txt"), "x")
	if a.callBool("DIRREMOVE", advplrt.NewString(nonEmpty)) {
		t.Error("DIRREMOVE em diretório não-vazio deveria retornar .F.")
	}
}

func TestDirectory(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	a.writeFile(filepath.Join(dir, "alpha.txt"), "aaaa")
	a.writeFile(filepath.Join(dir, "beta.bin"), "bbbbbb")

	spec := filepath.Join(dir, "*.*")
	arr := a.callArray("DIRECTORY", advplrt.NewString(spec), advplrt.NewString(""),
		advplrt.Nil, advplrt.NewBool(false), advplrt.NewNumber(1))
	if len(arr.Elements) != 2 {
		t.Fatalf("DIRECTORY: esperava 2 entradas, obteve %d", len(arr.Elements))
	}
	var foundAlpha, foundBeta bool
	for _, e := range arr.Elements {
		row, ok := e.(*advplrt.ArrayValue)
		if !ok || len(row.Elements) != 5 {
			t.Fatalf("DIRECTORY: linha deveria ser array de 5 elementos")
		}
		name := row.Elements[0].(*advplrt.StringValue).Val
		switch name {
		case "alpha.txt":
			foundAlpha = true
			if row.Elements[1].(*advplrt.NumberValue).Val != 4 {
				t.Error("tamanho de alpha.txt deveria ser 4")
			}
		case "beta.bin":
			foundBeta = true
			if row.Elements[1].(*advplrt.NumberValue).Val != 6 {
				t.Error("tamanho de beta.bin deveria ser 6")
			}
		}
		if _, ok := row.Elements[2].(*advplrt.DateValue); !ok {
			t.Error("terceiro elemento deveria ser DateValue (data)")
		}
		if _, ok := row.Elements[3].(*advplrt.StringValue); !ok {
			t.Error("quarto elemento deveria ser StringValue (hora)")
		}
		if attr := row.Elements[4].(*advplrt.StringValue).Val; attr != "A" {
			t.Errorf("atributo deveria ser 'A', obteve %q", attr)
		}
	}
	if !foundAlpha || !foundBeta {
		t.Errorf("DIRECTORY: arquivos esperados não encontrados (alpha=%v beta=%v)", foundAlpha, foundBeta)
	}
}

func TestFRename(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()

	run := func(name string) {
		src := filepath.Join(dir, "velho_"+name+".txt")
		dst := filepath.Join(dir, "novo_"+name+".txt")
		a.writeFile(src, "rename")
		if n := a.callNum(name, advplrt.NewString(src), advplrt.NewString(dst)); n != 0 {
			t.Fatalf("%s deveria retornar 0, obteve %v", name, n)
		}
		if a.fileExists(src) {
			t.Errorf("%s: arquivo antigo deveria não existir", name)
		}
		if got := a.readFile(dst); got != "rename" {
			t.Errorf("%s: arquivo novo deveria conter 'rename', obteve %q", name, got)
		}
	}
	run("FRENAME")
	run("FRENAMEEX")

	// Falha: arquivo de origem inexistente.
	if n := a.callNum("FRENAME", advplrt.NewString(filepath.Join(dir, "inexistente.txt")),
		advplrt.NewString(filepath.Join(dir, "alvo.txt"))); n != -1 {
		t.Errorf("FRENAME de arquivo inexistente deveria retornar -1, obteve %v", n)
	}
}

func TestCpyS2T(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}

	src := filepath.Join(srcDir, "dados.dat")
	a.writeFile(src, "conteudo de copia")

	for _, name := range []string{"CPYS2T", "CPYT2S"} {
		if !a.callBool(name, advplrt.NewString(src), advplrt.NewString(dstDir)) {
			t.Fatalf("%s deveria retornar .T.", name)
		}
		got := a.readFile(filepath.Join(dstDir, "dados.dat"))
		if got != "conteudo de copia" {
			t.Errorf("%s: conteúdo copiado incorreto: %q", name, got)
		}
	}

	// CPYS2TEX copia preservando o datetime de origem (destino completo).
	exDst := filepath.Join(dstDir, "preservado.dat")
	if !a.callBool("CPYS2TEX", advplrt.NewString(src), advplrt.NewString(exDst)) {
		t.Fatal("CPYS2TEX deveria retornar .T.")
	}
	if got := a.readFile(exDst); got != "conteudo de copia" {
		t.Errorf("CPYS2TEX: conteúdo copiado incorreto: %q", got)
	}
	srcInfo, _ := os.Stat(src)
	dstInfo, _ := os.Stat(exDst)
	if !srcInfo.ModTime().Equal(dstInfo.ModTime()) {
		t.Errorf("CPYS2TEX: datetime de origem deveria ser preservado (src=%v dst=%v)",
			srcInfo.ModTime(), dstInfo.ModTime())
	}
}

func TestCpyF2Web(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "webdata.bin")
	a.writeFile(src, "para a web")

	dst := a.callStr("CPYF2WEB", advplrt.NewString(src))
	if dst == "" {
		t.Fatal("CPYF2WEB deveria retornar caminho de destino")
	}
	if got := a.readFile(dst); got != "para a web" {
		t.Errorf("CPYF2WEB: conteúdo copiado incorreto: %q", got)
	}
	os.Remove(dst)

	// Arquivo inexistente → "" sem erro.
	if s := a.callStr("CPYF2WEB", advplrt.NewString(filepath.Join(dir, "nada.txt"))); s != "" {
		t.Errorf("CPYF2WEB de arquivo inexistente deveria retornar \"\", obteve %q", s)
	}
}

func TestCpyS2TW(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "twdata.txt")
	a.writeFile(src, "s2tw")

	if n := a.callNum("CPYS2TW", advplrt.NewString(src)); n != 0 {
		t.Fatalf("CPYS2TW deveria retornar 0, obteve %v", n)
	}
	if !a.fileExists(filepath.Join(os.TempDir(), "advpp-web", "twdata.txt")) {
		t.Error("CPYS2TW: arquivo deveria existir em advpp-web")
	}
	os.Remove(filepath.Join(os.TempDir(), "advpp-web", "twdata.txt"))

	if n := a.callNum("CPYS2TW", advplrt.NewString(filepath.Join(dir, "inexistente.txt"))); n != -2 {
		t.Errorf("CPYS2TW de arquivo inexistente deveria retornar -2, obteve %v", n)
	}
}

func TestFZip(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "hello.txt")
	a.writeFile(file, "conteudo zipado")
	zipPath := filepath.Join(dir, "out.zip")

	if n := a.callNum("FZIP", advplrt.NewString(zipPath),
		advplrt.NewArray([]advplrt.Value{advplrt.NewString(file)})); n != 0 {
		t.Fatalf("FZIP deveria retornar 0, obteve %v", n)
	}
	if !a.fileExists(zipPath) {
		t.Error("FZIP: arquivo .zip deveria existir")
	}

	// Senha não suportada → -1 honesto.
	if n := a.callNum("FZIP", advplrt.NewString(zipPath),
		advplrt.NewArray([]advplrt.Value{advplrt.NewString(file)}),
		advplrt.Nil, advplrt.NewString("senha")); n != -1 {
		t.Errorf("FZIP com senha deveria retornar -1, obteve %v", n)
	}
}

func TestFListZip(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "hello.txt")
	a.writeFile(file, "listar zip")
	zipPath := filepath.Join(dir, "list.zip")

	if n := a.callNum("FZIP", advplrt.NewString(zipPath),
		advplrt.NewArray([]advplrt.Value{advplrt.NewString(file)})); n != 0 {
		t.Fatalf("FZIP de preparação falhou: %v", n)
	}

	arr := a.callArray("FLISTZIP", advplrt.NewString(zipPath))
	found := false
	for _, e := range arr.Elements {
		row, ok := e.(*advplrt.ArrayValue)
		if !ok || len(row.Elements) != 2 {
			t.Fatalf("FLISTZIP: linha deveria ser array [nome, tamanho]")
		}
		if row.Elements[0].(*advplrt.StringValue).Val == "hello.txt" {
			found = true
			if row.Elements[1].(*advplrt.NumberValue).Val == 0 {
				t.Error("FLISTZIP: tamanho de hello.txt deveria ser > 0")
			}
		}
	}
	if !found {
		t.Error("FLISTZIP: nome hello.txt deveria constar na listagem")
	}
}

func TestFUnZip(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "hello.txt")
	a.writeFile(file, "extrai de volta")
	zipPath := filepath.Join(dir, "out.zip")
	outDir := filepath.Join(dir, "dest")

	if n := a.callNum("FZIP", advplrt.NewString(zipPath),
		advplrt.NewArray([]advplrt.Value{advplrt.NewString(file)})); n != 0 {
		t.Fatalf("FZIP de preparação falhou: %v", n)
	}

	if n := a.callNum("FUNZIP", advplrt.NewString(zipPath), advplrt.NewString(outDir)); n != 0 {
		t.Fatalf("FUNZIP deveria retornar 0, obteve %v", n)
	}
	if got := a.readFile(filepath.Join(outDir, "hello.txt")); got != "extrai de volta" {
		t.Errorf("FUNZIP: conteúdo extraído incorreto: %q", got)
	}
}

func TestGzCompress(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "data.txt")
	content := "roundtrip gzip byte-a-byte: áéçõ 12345"
	a.writeFile(src, content)
	gzPath := filepath.Join(dir, "data.txt.gz")
	outDir := filepath.Join(dir, "out")

	if !a.callBool("GZCOMPRESS", advplrt.NewString(src), advplrt.NewString(gzPath)) {
		t.Fatal("GZCOMPRESS deveria retornar .T.")
	}
	if !a.fileExists(gzPath) {
		t.Fatal("GZCOMPRESS: .gz deveria existir")
	}
	if !a.callBool("GZDECOMP", advplrt.NewString(gzPath), advplrt.NewString(outDir)) {
		t.Fatal("GZDECOMP deveria retornar .T.")
	}
	if got := a.readFile(filepath.Join(outDir, "data.txt")); got != content {
		t.Errorf("GZDECOMP: conteúdo não bate byte-a-byte: %q", got)
	}
}

func TestMsCompress(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "data.txt")
	content := "roundtrip MZP byte-a-byte"
	a.writeFile(src, content)
	mzpPath := filepath.Join(dir, "data.mzp")
	outDir := filepath.Join(dir, "out")

	if got := a.callStr("MSCOMPRESS", advplrt.NewString(src), advplrt.NewString(mzpPath)); got != mzpPath {
		t.Fatalf("MSCOMPRESS deveria retornar o destino %q, obteve %q", mzpPath, got)
	}
	if !a.fileExists(mzpPath) {
		t.Fatal("MSCOMPRESS: .mzp deveria existir")
	}
	if !a.callBool("MSDECOMP", advplrt.NewString(mzpPath), advplrt.NewString(outDir)) {
		t.Fatal("MSDECOMP deveria retornar .T.")
	}
	if got := a.readFile(filepath.Join(outDir, "data.txt")); got != content {
		t.Errorf("MSDECOMP: conteúdo não bate byte-a-byte: %q", got)
	}
}

func TestTar(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "data.txt")
	content := "roundtrip tar byte-a-byte"
	a.writeFile(src, content)
	tarPath := filepath.Join(dir, "data.tar")
	outDir := filepath.Join(dir, "out")

	res := a.callStr("TARCOMPRESS",
		advplrt.NewArray([]advplrt.Value{advplrt.NewString(src)}),
		advplrt.NewString(tarPath))
	if res == "" {
		t.Fatal("TARCOMPRESS deveria retornar o caminho absoluto do .tar")
	}
	if !a.fileExists(tarPath) {
		t.Fatal("TARCOMPRESS: .tar deveria existir")
	}
	if !a.callBool("TARDECOMP", advplrt.NewString(tarPath), advplrt.NewString(outDir)) {
		t.Fatal("TARDECOMP deveria retornar .T.")
	}

	// Os nomes no tar preservam o caminho absoluto (sem a barra inicial), então
	// localizamos o arquivo por varredura recursiva no diretório de extração.
	var found bool
	filepath.Walk(outDir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && info.Name() == "data.txt" {
			if got := a.readFile(p); got == content {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Error("TARDECOMP: arquivo data.txt com conteúdo original não encontrado no destino")
	}
}

func TestFT_FUseFEOFGoto(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "ft.txt")
	content := "hello world"
	a.writeFile(p, content)

	// FT_FUSE abre; handle > 0.
	h := a.callNum("FT_FUSE", advplrt.NewString(p))
	if h <= 0 {
		t.Fatalf("FT_FUSE deveria retornar handle > 0, obteve %v", h)
	}
	// No início de arquivo não-vazio, FT_FEOF é .F.
	if a.callBool("FT_FEOF") {
		t.Error("FT_FEOF no início de arquivo não-vazio deveria ser .F.")
	}
	// FT_FGOTO move o ponteiro (retorna Nil); no meio do arquivo não é EOF.
	if res := a.call("FT_FGOTO", advplrt.NewNumber(5)); !advplrt.IsNil(res) {
		t.Error("FT_FGOTO deveria retornar Nil")
	}
	if a.callBool("FT_FEOF") {
		t.Error("FT_FEOF no meio do arquivo deveria ser .F.")
	}
	// Ao fim do arquivo, FT_FEOF é .T.
	a.call("FT_FGOTO", advplrt.NewNumber(float64(len(content))))
	if !a.callBool("FT_FEOF") {
		t.Error("FT_FEOF no fim do arquivo deveria ser .T.")
	}
	// Sem argumento, FT_FUSE fecha o arquivo corrente e retorna 0.
	if n := a.callNum("FT_FUSE"); n != 0 {
		t.Errorf("FT_FUSE sem argumento deveria retornar 0, obteve %v", n)
	}
	if !a.callBool("FT_FEOF") {
		t.Error("FT_FEOF sem arquivo aberto deveria ser .T.")
	}
	// Arquivo inexistente → -1.
	if n := a.callNum("FT_FUSE", advplrt.NewString(filepath.Join(dir, "nao_existe.txt"))); n != -1 {
		t.Errorf("FT_FUSE de arquivo inexistente deveria retornar -1, obteve %v", n)
	}
	t.Cleanup(ftCloseCurrent)
}

func TestFRead(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "read.txt")
	content := "hello world"
	a.writeFile(p, content)

	// Usa FOPEN (natives.go) para obter o handle que FREAD consome.
	fopen, ok := a.v.natives["FOPEN"]
	if !ok {
		t.Fatal("FOPEN não registrada")
	}
	hres, err := fopen.Fn([]advplrt.Value{advplrt.NewString(p), advplrt.NewNumber(0)})
	if err != nil {
		t.Fatalf("FOPEN falhou: %v", err)
	}
	h := hres.(*advplrt.NumberValue).Val

	if n := a.callNum("FREAD", advplrt.NewNumber(h), advplrt.Nil,
		advplrt.NewNumber(float64(len(content)))); n != float64(len(content)) {
		t.Errorf("FREAD deveria ler %d bytes, leu %v", len(content), n)
	}
	// EOF: próxima leitura retorna 0.
	if n := a.callNum("FREAD", advplrt.NewNumber(h), advplrt.Nil,
		advplrt.NewNumber(5)); n != 0 {
		t.Errorf("FREAD no fim do arquivo deveria retornar 0, obteve %v", n)
	}
	// Handle inválido → 0 (e lastFError=6).
	if n := a.callNum("FREAD", advplrt.NewNumber(9999), advplrt.Nil,
		advplrt.NewNumber(5)); n != 0 {
		t.Errorf("FREAD com handle inválido deveria retornar 0, obteve %v", n)
	}
	if a.v.lastFError != 6 {
		t.Errorf("FREAD com handle inválido deveria setar lastFError=6, obteve %d", a.v.lastFError)
	}

	fclose, ok := a.v.natives["FCLOSE"]
	if !ok {
		t.Fatal("FCLOSE não registrada")
	}
	if _, err := fclose.Fn([]advplrt.Value{advplrt.NewNumber(h)}); err != nil {
		t.Fatalf("FCLOSE falhou: %v", err)
	}
}

func TestListDrives(t *testing.T) {
	a := newArquivosIO(t)
	arr := a.callArray("LISTDRIVES")
	if len(arr.Elements) == 0 {
		t.Error("LISTDRIVES não deveria retornar lista vazia")
	}
	units := elemStrings(arr.Elements)
	if runtime.GOOS == "windows" {
		// No Windows as unidades têm a forma "C:", "D:", etc.
		foundDrive := false
		for _, u := range units {
			if len(u) == 2 && u[1] == ':' {
				foundDrive = true
				break
			}
		}
		if !foundDrive {
			t.Errorf("LISTDRIVES no Windows deveria conter uma letra de unidade, obteve %v", units)
		}
	} else if !containsStr(units, "/") {
		t.Errorf("LISTDRIVES no Linux deveria conter '/', obteve %v", units)
	}
}

func TestLogMsg(t *testing.T) {
	a := newArquivosIO(t)
	dir := t.TempDir()
	logPath := filepath.Join(dir, "advpp.log")

	old := os.Getenv("ADVPP_LOG")
	os.Setenv("ADVPP_LOG", logPath)
	defer func() {
		if old == "" {
			os.Unsetenv("ADVPP_LOG")
		} else {
			os.Setenv("ADVPP_LOG", old)
		}
	}()

	if !a.callBool("LOGMSG", advplrt.NewString("FUNCAO_TESTE"), advplrt.NewNumber(1),
		advplrt.NewNumber(6), advplrt.NewNumber(1), advplrt.NewString("MSG001"),
		advplrt.NewString("2026-08-11"), advplrt.NewString("texto da mensagem")) {
		t.Fatal("LOGMSG deveria retornar .T. (best-effort)")
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("arquivo de log deveria ter sido criado: %v", err)
	}
	line := string(data)
	if !strings.Contains(line, "FUNCAO_TESTE") || !strings.Contains(line, "texto da mensagem") {
		t.Errorf("linha de log deveria conter função e mensagem: %q", line)
	}
}

// helpers de comparação

func containsStr(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func elemStrings(elems []advplrt.Value) []string {
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		out = append(out, advplrt.ToString(e))
	}
	return out
}
