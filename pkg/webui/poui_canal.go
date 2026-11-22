package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/advpl/compiler/pkg/tools/shared"
)

// pouiMsg é um jsToAdvpl recebido da aba do app (POST /poui-ui).
type pouiMsg struct {
	tipo, conteudo string
}

// pouiCanal é o canal de UM app PO-UI aberto por FwCallApp dentro de uma
// sessão do modo web: inbox recebe jsToAdvpl da aba, tab carrega os
// eventos advpltojs para o SSE dela, dead marca a aba desconectada por
// mais que o timeout configurado.
type pouiCanal struct {
	app    *PouiApp
	ctx    *PouiCtx
	inbox  chan pouiMsg
	tab    chan event
	closed chan struct{}

	closeOnce sync.Once
	mu        sync.Mutex
	conns     int           // SSE canal=poui abertas (a aba do app)
	gen       int           // geração do timer de aba morta
	dead      chan struct{} // fechado quando a aba some > PouiDesconexaoSeg
}

func novaPouiCanal(app *PouiApp, ctx *PouiCtx) *pouiCanal {
	return &pouiCanal{
		app: app, ctx: ctx,
		inbox:  make(chan pouiMsg, 64),
		tab:    make(chan event, 64),
		dead:   make(chan struct{}), // nunca fecha enquanto a aba vive
		closed: make(chan struct{}),
	}
}

// fecha marca o canal como encerrado (close do app ou fim da sessão).
func (c *pouiCanal) fecha() {
	c.closeOnce.Do(func() { close(c.closed) })
}

// desconexaoSeg lê o timeout configurado (padrão 10 s):
// ADVPP_POUI_DESCONEXAO_SEG (teste/override) → poui_desconexao_seg do
// config → 10 s.
func desconexaoSeg() time.Duration {
	if n, err := strconv.Atoi(os.Getenv("ADVPP_POUI_DESCONEXAO_SEG")); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if cfg, err := shared.LoadConfig(); err == nil && cfg.PouiDesconexaoSeg > 0 {
		return time.Duration(cfg.PouiDesconexaoSeg) * time.Second
	}
	return 10 * time.Second
}

// tabConecta registra uma conexão SSE da aba do app. Devolve false se já
// houver uma aba viva (segunda aba -> 409). Ao reconectar dentro do prazo,
// o timer de morte é cancelado pela nova geração.
func (c *pouiCanal) tabConecta() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conns > 0 {
		return false
	}
	c.conns++
	c.gen++
	c.dead = make(chan struct{}) // geração nova: timer antigo perde a referência
	return true
}

// tabDesconecta desfaz o registro; quando zera, arma o timer que fecha
// dead após desconexaoSeg sem reconexão.
func (c *pouiCanal) tabDesconecta() {
	c.mu.Lock()
	c.conns--
	if c.conns < 0 {
		c.conns = 0
	}
	if c.conns == 0 {
		gen := c.gen
		dead := c.dead
		go func() {
			time.Sleep(desconexaoSeg())
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.gen == gen && c.conns == 0 {
				close(dead)
			}
		}()
	}
	c.mu.Unlock()
}

// deadCh devolve o canal fechado quando a aba estiver morta (geração atual).
func (c *pouiCanal) deadCh() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dead
}

// ---------------------------------------------------------------------------
// Provider — interface usada pela VM (pkg/vm implementa PouiAppUI sobre isto)
// ---------------------------------------------------------------------------

// AbrePouiAppPorNome é o PouiAppUI do lado webui (implementa a interface
// que a VM consome): localiza o app em dirs + poui_apps_dirs do config,
// monta o contexto da sessão e abre o canal.
func (p *Provider) AbrePouiAppPorNome(nome string, dirs []string) (string, error) {
	cfg, _ := shared.LoadConfig()
	for _, d := range cfg.PouiAppsDirs {
		dirs = append(dirs, d)
	}
	caminho, err := LocalizaPouiApp(nome, dirs)
	if err != nil {
		return "", err
	}
	maxZip := int64(64 << 20)
	if cfg.PouiAppMaxMB > 0 {
		maxZip = cfg.PouiAppMaxMB << 20
	}
	app, err := AbrePouiApp(caminho, maxZip)
	if err != nil {
		return "", err
	}
	usuario := os.Getenv("USER")
	if usuario == "" {
		usuario = os.Getenv("USERNAME")
	}
	if usuario == "" {
		usuario = "admin"
	}
	ctx, err := NovoPouiCtx(app.Nome, "01", "01", "CUSTOM", usuario, "pt-br",
		shared.ResolveRestPrefix(), p.s.origin)
	if err != nil {
		return "", err
	}
	return p.AbrePouiApp(app, ctx)
}

// AbrePouiApp registra o app na sessão e avisa a SPA para abrir a aba.
// Devolve a URL que a aba carrega (/?sid=&app= no shim).
func (p *Provider) AbrePouiApp(app *PouiApp, ctx *PouiCtx) (string, error) {
	s := p.s
	s.mu.Lock()
	if s.poui != nil {
		s.mu.Unlock()
		return "", fmt.Errorf("app já aberto nesta sessão")
	}
	canal := novaPouiCanal(app, ctx)
	s.poui = canal
	s.pouiGone = false
	s.mu.Unlock()

	url := fmt.Sprintf("/poui/%s/?sid=%s&app=%s", app.Nome, s.sid, app.Nome)
	data, _ := json.Marshal(map[string]string{"url": url})
	s.send(event{Type: "open-tab", Data: data})
	return url, nil
}

// PouiNext bloqueia até chegar um jsToAdvpl, o app fechar ("close" ou aba
// morta além do timeout) ou a sessão terminar. timeout é o teto de espera:
// expirado sem evento, devolve ("", "", false) para o chamador reavaliar.
func (p *Provider) PouiNext(timeout time.Duration) (tipo, conteudo string, fechou bool) {
	s := p.s
	s.mu.Lock()
	canal := s.poui
	s.mu.Unlock()
	if canal == nil {
		return "", "", true
	}
	var timer <-chan time.Time
	if timeout > 0 {
		t := time.NewTimer(timeout)
		defer t.Stop()
		timer = t.C
	}
	select {
	case m := <-canal.inbox:
		if m.tipo == "close" {
			return m.tipo, m.conteudo, true
		}
		return m.tipo, m.conteudo, false
	case <-canal.deadCh():
		return "", "", true
	case <-canal.closed:
		return "", "", true
	case <-s.quit:
		return "", "", true
	case <-timer:
		return "", "", false
	}
}

// PouiEnvia entrega um evento advpltojs à aba do app (SSE canal=poui).
func (p *Provider) PouiEnvia(tipo, conteudo string) {
	s := p.s
	s.mu.Lock()
	canal := s.poui
	s.mu.Unlock()
	if canal == nil {
		return
	}
	data, _ := json.Marshal(map[string]string{"type": tipo, "content": conteudo})
	select {
	case canal.tab <- event{Type: "advpltojs", Data: data}:
	case <-canal.closed:
	case <-s.quit:
	}
}

// FechaPouiApp desregistra o app: o laço do FwCallApp deve chamá-lo ao
// sair, e requisições seguintes a /poui/{app} passam a responder 410.
func (p *Provider) FechaPouiApp() {
	s := p.s
	s.mu.Lock()
	canal := s.poui
	s.poui = nil
	s.pouiGone = true
	s.mu.Unlock()
	if canal != nil {
		canal.fecha()
	}
}

// ---------------------------------------------------------------------------
// Handlers HTTP das rotas /poui*
// ---------------------------------------------------------------------------

// sidDaRequisicao resolve a sessão do app: ?sid= na primeira carga, depois
// o cookie poui_sid (a aba recarrega sem query).
func (srv *Server) sessaoDoApp(w http.ResponseWriter, r *http.Request, sidQuery string) (*session, bool) {
	sid := sidQuery
	if sid == "" {
		if c, err := r.Cookie("poui_sid"); err == nil {
			sid = c.Value
		}
	}
	if sid == "" {
		return nil, false
	}
	srv.mu.Lock()
	s := srv.sessions[sid]
	srv.mu.Unlock()
	return s, s != nil
}

// handlePouiApp serve GET /poui/{app}/{resto...}: valida a sessão e o app
// abertos e delega ao PouiApp com o caminho relativo.
func (srv *Server) handlePouiApp(w http.ResponseWriter, r *http.Request, appNome, resto string) {
	s, ok := srv.sessaoDoApp(w, r, r.URL.Query().Get("sid"))
	if !ok {
		http.Error(w, "sessão não encontrada", http.StatusNotFound)
		return
	}
	s.mu.Lock()
	canal := s.poui
	gone := s.pouiGone
	s.mu.Unlock()
	if canal == nil {
		if gone {
			http.Error(w, "app fechado", http.StatusGone)
			return
		}
		http.Error(w, "app não aberto nesta sessão", http.StatusNotFound)
		return
	}
	if canal.app.Nome != appNome {
		http.Error(w, "app não aberto nesta sessão", http.StatusNotFound)
		return
	}
	// Primeira carga (?sid=): fixa o cookie para o F5 sem query.
	if r.URL.Query().Get("sid") != "" {
		http.SetCookie(w, &http.Cookie{
			Name: "poui_sid", Value: s.sid, Path: "/poui/" + appNome + "/",
			HttpOnly: true, SameSite: http.SameSiteStrictMode,
		})
	}
	r2 := r.Clone(r.Context())
	u := *r.URL
	u.Path = "/" + resto
	r2.URL = &u
	canal.app.ServeHTTP(w, r2)
}

// handlePouiCtx devolve o SessionStorage() do app como JSON.
func (srv *Server) handlePouiCtx(w http.ResponseWriter, r *http.Request) {
	s, ok := srv.sessaoDoApp(w, r, r.URL.Query().Get("sid"))
	if !ok {
		http.Error(w, "sessão não encontrada", http.StatusNotFound)
		return
	}
	s.mu.Lock()
	canal := s.poui
	s.mu.Unlock()
	if canal == nil {
		http.Error(w, "app não aberto nesta sessão", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(canal.ctx.SessionStorage())
}

// handlePouiUI recebe o jsToAdvpl da aba (POST /poui-ui?sid=).
func (srv *Server) handlePouiUI(w http.ResponseWriter, r *http.Request) {
	s, ok := srv.sessaoDoApp(w, r, r.URL.Query().Get("sid"))
	if !ok {
		http.Error(w, "sessão não encontrada", http.StatusNotFound)
		return
	}
	s.mu.Lock()
	canal := s.poui
	s.mu.Unlock()
	if canal == nil {
		http.Error(w, "app não aberto nesta sessão", http.StatusNotFound)
		return
	}
	var body struct {
		App     string `json:"app"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.App != "" && body.App != canal.app.Nome {
		http.Error(w, "app divergente", http.StatusBadRequest)
		return
	}
	select {
	case canal.inbox <- pouiMsg{tipo: body.Type, conteudo: body.Content}:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "fila cheia", http.StatusServiceUnavailable)
	}
}

// handlePouiSSE é o stream da aba do app: /events?s=<sid>&canal=poui.
// Devolve false quando a requisição não é do canal poui.
func (srv *Server) handlePouiSSE(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Query().Get("canal") != "poui" {
		return false
	}
	s, ok := srv.sessaoDoApp(w, r, r.URL.Query().Get("s"))
	if !ok {
		http.Error(w, "sessão não encontrada", http.StatusNotFound)
		return true
	}
	s.mu.Lock()
	canal := s.poui
	s.mu.Unlock()
	if canal == nil {
		http.Error(w, "app não aberto nesta sessão", http.StatusNotFound)
		return true
	}
	if !canal.tabConecta() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "app já aberto nesta sessão"})
		return true
	}
	defer canal.tabDesconecta()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return true
	}
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	enc := json.NewEncoder(w)
	for {
		select {
		case ev := <-canal.tab:
			fmt.Fprintf(w, "data: ")
			_ = enc.Encode(ev)
			fmt.Fprintf(w, "\n")
			flusher.Flush()
		case <-canal.closed:
			return true
		case <-s.quit:
			return true
		case <-r.Context().Done():
			return true
		}
	}
}
