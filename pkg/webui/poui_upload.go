package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/advpl/compiler/pkg/tools/shared"
)

// POST /poui-upload?token=<t> — recebe o multipart do po-upload e grava
// em cDiretorioDestino do componente (registrado pela VM via
// shared.RegistrarUploadPoui). O cliente nunca escolhe o diretório:
// o token é opaco e o destino já passou por DirUploadPermitido.
//
// Resposta: 200 + JSON [{cNome, cCaminhoGravado, nBytes}, ...] — o
// po-upload repassa ao (p-success), que o ui-host converte no evento
// "upload" do AdvPL (OnUpload(aArquivos)).
//
// Erros: 404 token desconhecido, 403 destino fora da área permitida,
// 413 corpo/arquivo acima do limite, 415 extensão proibida.

// pouiUploadArquivo é um arquivo gravado (espelha o retorno do OnUpload).
type pouiUploadArquivo struct {
	Nome    string `json:"cNome"`
	Caminho string `json:"cCaminhoGravado"`
	Bytes   int64  `json:"nBytes"`
}

func (srv *Server) handlePouiUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	spec, ok := shared.SpecUploadPoui(r.URL.Query().Get("token"))
	if !ok {
		http.Error(w, "token desconhecido", http.StatusNotFound)
		return
	}
	if !shared.DirUploadPermitido(spec.Dir()) {
		http.Error(w, "diretório de destino não permitido", http.StatusForbidden)
		return
	}
	if err := os.MkdirAll(spec.Dir(), 0o755); err != nil {
		http.Error(w, "destino indisponível", http.StatusForbidden)
		return
	}
	// Limite do corpo = teto por arquivo (10–100 MB). O po-upload manda
	// um POST por arquivo (ou um multipart com N campos "files").
	r.Body = http.MaxBytesReader(w, r.Body, spec.Limite())
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "multipart inválido ou acima do limite", http.StatusRequestEntityTooLarge)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		// Alguns clientes mandam o campo com outro nome: aceita o primeiro.
		for _, fs := range r.MultipartForm.File {
			files = fs
			break
		}
	}
	if len(files) == 0 {
		http.Error(w, "nenhum arquivo", http.StatusBadRequest)
		return
	}
	gravados := []pouiUploadArquivo{}
	for _, fh := range files {
		// Nome sanitizado: só o base, sem "."/".." — o destino já é fixo.
		nome := filepath.Base(strings.ReplaceAll(fh.Filename, "\\", "/"))
		if nome == "" || nome == "." || nome == ".." || strings.ContainsAny(nome, "/\\") {
			http.Error(w, "nome de arquivo inválido", http.StatusBadRequest)
			return
		}
		ext := filepath.Ext(nome)
		if !spec.PermiteExt(ext) {
			http.Error(w, fmt.Sprintf("extensão %q não permitida", ext), http.StatusUnsupportedMediaType)
			return
		}
		if fh.Size > spec.Limite() {
			http.Error(w, "arquivo acima do limite", http.StatusRequestEntityTooLarge)
			return
		}
		src, err := fh.Open()
		if err != nil {
			http.Error(w, "falha ao ler upload", http.StatusInternalServerError)
			return
		}
		dst := filepath.Join(spec.Dir(), nome)
		out, err := os.Create(dst)
		if err != nil {
			src.Close()
			http.Error(w, "falha ao gravar", http.StatusInternalServerError)
			return
		}
		n, err := io.Copy(out, src)
		src.Close()
		cerr := out.Close()
		if err != nil || cerr != nil || n != fh.Size {
			_ = os.Remove(dst)
			http.Error(w, "falha ao gravar", http.StatusInternalServerError)
			return
		}
		gravados = append(gravados, pouiUploadArquivo{Nome: nome, Caminho: dst, Bytes: n})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(gravados)
}
