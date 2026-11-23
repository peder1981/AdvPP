package shared

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Registro de uploads PoUpload (Onda 3a.4). O componente AdvPL registra
// aqui o destino validado; a rota POST /poui-upload do webui resolve o
// token e grava o arquivo — o cliente nunca escolhe o diretório.

type pouiUploadSpec struct {
	DirAbs   string
	MaxBytes int64
	Exts     map[string]bool // minúsculas com ponto; nil = todas
}

var (
	pouiUploadsMu sync.RWMutex
	pouiUploads   = map[string]pouiUploadSpec{}
)

// NovoTokenUpload gera um token opaco para a URL do po-upload.
func NovoTokenUpload() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RegistrarUploadPoui registra o destino de um PoUpload. dir deve ser
// absoluto (o handler confere DirUploadPermitido a cada POST — destino
// fora da área permitida responde 403). maxMB é limitado a [1,100]
// (0 cai no padrão de 10 MB). Devolve o spec efetivo.
func RegistrarUploadPoui(token, dir string, maxMB int, exts []string) (pouiUploadSpec, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return pouiUploadSpec{}, false
	}
	mb := int64(maxMB)
	if mb <= 0 {
		mb = 10
	}
	if mb > 100 {
		mb = 100
	}
	var set map[string]bool
	if len(exts) > 0 {
		set = map[string]bool{}
		for _, e := range exts {
			e = strings.ToLower(strings.TrimSpace(e))
			if e == "" {
				continue
			}
			if !strings.HasPrefix(e, ".") {
				e = "." + e
			}
			set[e] = true
		}
		if len(set) == 0 {
			set = nil
		}
	}
	spec := pouiUploadSpec{DirAbs: abs, MaxBytes: mb << 20, Exts: set}
	pouiUploadsMu.Lock()
	pouiUploads[token] = spec
	pouiUploadsMu.Unlock()
	return spec, true
}

// SpecUploadPoui devolve o spec de um token (cópia) ou ok=false.
func SpecUploadPoui(token string) (pouiUploadSpec, bool) {
	pouiUploadsMu.RLock()
	s, ok := pouiUploads[token]
	pouiUploadsMu.RUnlock()
	return s, ok
}

// RemoverUploadPoui descarta o registro (app fechado).
func RemoverUploadPoui(token string) {
	pouiUploadsMu.Lock()
	delete(pouiUploads, token)
	pouiUploadsMu.Unlock()
}

// Campos do spec para o handler HTTP.
func (s pouiUploadSpec) Dir() string     { return s.DirAbs }
func (s pouiUploadSpec) Limite() int64   { return s.MaxBytes }
func (s pouiUploadSpec) ListaExts() []string {
	if s.Exts == nil {
		return nil
	}
	out := make([]string, 0, len(s.Exts))
	for e := range s.Exts {
		out = append(out, e)
	}
	return out
}
func (s pouiUploadSpec) PermiteExt(ext string) bool {
	if s.Exts == nil {
		return true
	}
	return s.Exts[strings.ToLower(ext)]
}

// DirUploadPermitido diz se dir (já absoluto) está dentro do diretório
// de trabalho do processo ou de um dos poui_upload_dirs do config.
// Fora disso -> 403 no upload. A comparação usa o caminho real (links
// simbólicos resolvidos): um link dentro da área que aponta para fora
// não serve de destino nem de origem do /poui-asset.
func DirUploadPermitido(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	abs = caminhoReal(abs)
	cwd, err := os.Getwd()
	if err == nil {
		if dentroDe(abs, caminhoReal(cwd)) {
			return true
		}
	}
	if cfg, err := LoadConfig(); err == nil {
		for _, d := range cfg.PouiUploadDirs {
			if ad, err := filepath.Abs(d); err == nil && dentroDe(abs, caminhoReal(ad)) {
				return true
			}
		}
	}
	return false
}

// caminhoReal resolve os links simbólicos do maior prefixo existente de p
// (absoluto) e reanexa o resto, que ainda não existe (ex.: o diretório de
// upload antes do MkdirAll).
func caminhoReal(p string) string {
	resto := ""
	for cur := p; ; {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, resto)
		}
		pai := filepath.Dir(cur)
		if pai == cur {
			return p
		}
		resto = filepath.Join(filepath.Base(cur), resto)
		cur = pai
	}
}

func dentroDe(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
