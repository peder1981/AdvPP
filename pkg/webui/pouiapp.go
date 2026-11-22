package webui

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PouiAppMaxDefault é o limite padrão do .app em memória (64 MB).
const PouiAppMaxDefault = 64 << 20

// PouiApp é um app PO-UI servível: pasta no disco (os.DirFS) ou pacote
// .app (zip lido inteiro em memória, nunca extraído). O index.html é
// localizado case-insensitive na abertura e guardado para o fallback de
// SPA.
type PouiApp struct {
	Nome  string
	fsys  fs.FS
	index string // caminho do index dentro de fsys ("" = sem index — não deveria acontecer)
}

// AbrePouiApp abre um app PO-UI: diretório com index.html ou arquivo .app
// (zip). maxZip <= 0 usa PouiAppMaxDefault. O nome do app é o base do
// caminho sem a extensão .app e precisa ser NomeDeAppValido.
func AbrePouiApp(caminho string, maxZip int64) (*PouiApp, error) {
	if maxZip <= 0 {
		maxZip = PouiAppMaxDefault
	}
	info, err := os.Stat(caminho)
	if err != nil {
		return nil, fmt.Errorf("app não encontrado: %s", caminho)
	}
	nome := strings.TrimSuffix(filepath.Base(caminho), filepath.Ext(caminho))
	if info.IsDir() {
		nome = filepath.Base(caminho)
	}
	if !NomeDeAppValido(nome) {
		return nil, fmt.Errorf("nome de app inválido: %q", nome)
	}

	var fsys fs.FS
	if info.IsDir() {
		fsys = os.DirFS(caminho)
	} else {
		if st, err := os.Stat(caminho); err == nil && st.Size() > maxZip {
			return nil, fmt.Errorf("app %s excede o limite de %d bytes", nome, maxZip)
		}
		fsys, err = abreZipApp(caminho, maxZip)
		if err != nil {
			return nil, err
		}
	}

	index, err := achaIndex(fsys)
	if err != nil {
		return nil, fmt.Errorf("app %s: %w", nome, err)
	}
	return &PouiApp{Nome: nome, fsys: fsys, index: index}, nil
}

// achaIndex localiza o index.html na raiz do FS, case-insensitive
// (Index.html/INDEX.HTML valem — bundlers Windows já produziram os dois).
func achaIndex(fsys fs.FS) (string, error) {
	if _, err := fs.Stat(fsys, "index.html"); err == nil {
		return "index.html", nil
	}
	ents, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return "", fmt.Errorf("index.html não encontrado: %v", err)
	}
	for _, e := range ents {
		if !e.IsDir() && strings.EqualFold(e.Name(), "index.html") {
			return e.Name(), nil
		}
	}
	return "", fmt.Errorf("index.html não encontrado")
}

// abreZipApp lê o .app inteiro em memória validando cada entrada: nada de
// caminho absoluto, ".." ou link simbólico (zip-slip). Se todas as
// entradas compartilham um único diretório de topo que contém o index, ele
// vira a raiz do app.
func abreZipApp(caminho string, maxZip int64) (fs.FS, error) {
	data, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxZip {
		return nil, fmt.Errorf("app excede o limite de %d bytes", maxZip)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("app .app inválido: %v", err)
	}
	files := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		nome := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(nome, "/") || strings.Contains(nome, "\x00") ||
			f.FileInfo().Mode()&os.ModeSymlink != 0 ||
			strings.HasPrefix(nome, "../") || nome == ".." ||
			strings.Contains(nome, "/../") {
			return nil, fmt.Errorf("entrada inválida no .app: %q", f.Name)
		}
		if strings.HasSuffix(nome, "/") {
			continue // diretório implícito
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("app .app: %v", err)
		}
		conteudo, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("app .app: %v", err)
		}
		files[nome] = conteudo
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("app .app vazio")
	}
	// Raiz aninhada: um único diretório de topo com index.html dentro.
	topo := ""
	unico := true
	for nome := range files {
		seg := strings.SplitN(nome, "/", 2)[0]
		if topo == "" {
			topo = seg
		} else if seg != topo {
			unico = false
			break
		}
	}
	if unico && strings.Contains(topo, "/") == false {
		// topo é o primeiro segmento; só rebaixa se TODO arquivo estiver sob ele
		// e existir <topo>/index.html (case-insensitive).
		temIndex := false
		for nome := range files {
			if !strings.HasPrefix(nome, topo+"/") {
				unico = false
				break
			}
			if strings.EqualFold(nome, topo+"/index.html") {
				temIndex = true
			}
		}
		if unico && temIndex {
			podados := make(map[string][]byte, len(files))
			for nome, conteudo := range files {
				podados[strings.TrimPrefix(nome, topo+"/")] = conteudo
			}
			files = podados
		}
	}
	return zipFS(files), nil
}

// zipFS é um fs.FS mínimo sobre map[string][]byte (o .app já validado).
type zipFS map[string][]byte

func (z zipFS) Open(nome string) (fs.File, error) {
	nome = strings.TrimPrefix(path.Clean("/"+nome), "/")
	if nome == "." {
		nome = ""
	}
	data, ok := z[nome]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return &zipFile{Reader: bytes.NewReader(data), name: nome, size: int64(len(data))}, nil
}

// ReadDir lista a raiz e subdiretórios para o fallback de SPA e o
// achaIndex — fs.ReadDir precisa de ReadDirFS ou de Open com ReadDirFile;
// como nosso Open devolve arquivo simples, implementamos ReadDirFS.
func (z zipFS) ReadDir(nome string) ([]fs.DirEntry, error) {
	nome = strings.TrimPrefix(path.Clean("/"+nome), "/")
	if nome != "" && nome != "." {
		nome += "/"
	}
	vistos := map[string]bool{}
	var out []fs.DirEntry
	for k := range z {
		if !strings.HasPrefix(k, nome) {
			continue
		}
		resto := strings.TrimPrefix(k, nome)
		if i := strings.IndexByte(resto, '/'); i >= 0 {
			resto = resto[:i]
		}
		if resto == "" || vistos[resto] {
			continue
		}
		vistos[resto] = true
		out = append(out, zipDirEntry{nome: resto, dir: strings.Contains(strings.TrimPrefix(k, nome), "/")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

type zipDirEntry struct {
	nome string
	dir  bool
}

func (e zipDirEntry) Name() string      { return e.nome }
func (e zipDirEntry) IsDir() bool       { return e.dir }
func (e zipDirEntry) Type() fs.FileMode { return 0 }
func (e zipDirEntry) Info() (fs.FileInfo, error) {
	return zipFileInfo{nome: e.nome, dir: e.dir}, nil
}

type zipFileInfo struct {
	nome string
	dir  bool
}

func (f zipFileInfo) Name() string { return f.nome }
func (f zipFileInfo) Size() int64  { return 0 }
func (f zipFileInfo) Mode() fs.FileMode {
	if f.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (f zipFileInfo) ModTime() time.Time { return time.Time{} }
func (f zipFileInfo) IsDir() bool        { return f.dir }
func (f zipFileInfo) Sys() any           { return nil }

type zipFile struct {
	*bytes.Reader
	name string
	size int64
}

func (f *zipFile) Stat() (fs.FileInfo, error) { return zipFileInfo{nome: f.name}, nil }
func (f *zipFile) Close() error               { return nil }

// ServeHTTP serve o app: caminho relativo à raiz. "/" e rotas de SPA sem
// extensão devolvem o index.html; arquivo inexistente COM extensão dá 404.
// Content-Type pela extensão (mime.TypeByExtension).
func (a *PouiApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	if strings.Contains(p, "..") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	nome := strings.TrimPrefix(p, "/")
	if nome == "" || strings.HasSuffix(nome, "/") {
		nome += a.index
	}
	data, err := readFSFile(a.fsys, nome)
	if err != nil {
		// SPA fallback: caminho sem extensão -> index.html.
		if path.Ext(nome) == "" {
			data, err = readFSFile(a.fsys, a.index)
		}
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}
	if ct := mime.TypeByExtension(path.Ext(nome)); ct != "" {
		if strings.HasPrefix(ct, "text/") || strings.Contains(ct, "javascript") || strings.Contains(ct, "json") || strings.Contains(ct, "svg") {
			ct += "; charset=utf-8"
		}
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// readFSFile lê um arquivo do fs.FS (ReadFileFS ou Open+ReadAll).
func readFSFile(fsys fs.FS, nome string) ([]byte, error) {
	if rfs, ok := fsys.(fs.ReadFileFS); ok {
		return rfs.ReadFile(nome)
	}
	f, err := fsys.Open(nome)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// LocalizaPouiApp procura o app nos diretórios, nesta ordem e para cada
// dir: <dir>/<nome>.app primeiro, depois <dir>/<nome>/ com index.html.
// Devolve o caminho encontrado ou erro.
func LocalizaPouiApp(nome string, dirs []string) (string, error) {
	if !NomeDeAppValido(nome) {
		return "", fmt.Errorf("nome de app inválido: %q", nome)
	}
	for _, dir := range dirs {
		app := filepath.Join(dir, nome+".app")
		if st, err := os.Stat(app); err == nil && !st.IsDir() {
			return app, nil
		}
		pasta := filepath.Join(dir, nome)
		if st, err := os.Stat(pasta); err == nil && st.IsDir() {
			if _, err := achaIndex(os.DirFS(pasta)); err == nil {
				return pasta, nil
			}
		}
	}
	return "", fmt.Errorf("app %q não encontrado", nome)
}
