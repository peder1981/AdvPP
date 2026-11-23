package webui

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/advpl/compiler/pkg/tools/shared"
)

// GET /poui-asset?sid=<sessão>&p=<caminho relativo> — serve imagens e
// demais assets referenciados por componentes Po* (PoAvatar/PoImage/
// PoSlide) com src relativa. O caminho é resolvido contra o diretório
// de trabalho do serve e precisa passar pela mesma checagem do upload
// (DirUploadPermitido): nada fora da área permitida é servido, e ".."
// é rejeitado antes de qualquer I/O.

const pouiAssetMaxBytes = 10 << 20 // 10 MB por asset

func (srv *Server) handlePouiAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	// O sid precisa ser de uma sessão viva — asset não é endpoint público.
	sid := r.URL.Query().Get("sid")
	srv.mu.Lock()
	_, ok := srv.sessions[sid]
	srv.mu.Unlock()
	if !ok {
		http.Error(w, "sessão desconhecida", http.StatusNotFound)
		return
	}
	rel := filepath.FromSlash(r.URL.Query().Get("p"))
	if rel == "" || strings.Contains(rel, "\x00") {
		http.Error(w, "caminho inválido", http.StatusBadRequest)
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		http.Error(w, "indisponível", http.StatusInternalServerError)
		return
	}
	abs := filepath.Join(cwd, rel)
	// Checagem de caminho idêntica ao upload: dentro do cwd ou de um
	// poui_upload_dirs configurado — e nunca acima da raiz permitida.
	if !shared.DirUploadPermitido(abs) {
		http.Error(w, "fora da área permitida", http.StatusForbidden)
		return
	}
	// Garante que o arquivo real (após symlinks) continua permitido.
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
		if !shared.DirUploadPermitido(abs) {
			http.Error(w, "fora da área permitida", http.StatusForbidden)
			return
		}
	}
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() {
		http.Error(w, "não encontrado", http.StatusNotFound)
		return
	}
	if st.Size() > pouiAssetMaxBytes {
		http.Error(w, "asset acima do limite", http.StatusRequestEntityTooLarge)
		return
	}
	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(abs))); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	http.ServeFile(w, r, abs)
}
