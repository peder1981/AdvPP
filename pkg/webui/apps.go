package webui

import (
	"errors"
	"regexp"
)

// AppResolver devolve o programa de um app ("" = página inicial). Erro
// ErrAppNaoEncontrado vira 404; outros erros, 500.
type AppResolver func(app string) (RunFunc, error)

// ErrAppNaoEncontrado: não há programa com esse nome.
var ErrAppNaoEncontrado = errors.New("app não encontrado")

var reNomeApp = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// NomeDeAppValido: só letras, dígitos, '_' e '-' (até 64): o nome nunca vira
// caminho de arquivo (sem '/', '.', NUL).
func NomeDeAppValido(nome string) bool { return reNomeApp.MatchString(nome) }

// NewApps cria o servidor de vários programas: cada sessão roda o app pedido
// em /events?app=<nome>, e /app/<nome> serve a página.
func NewApps(titulo string, resolve AppResolver) *Server {
	return &Server{sourceName: titulo, resolve: resolve, sessions: map[string]*session{}}
}
