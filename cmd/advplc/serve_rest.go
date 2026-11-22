package main

import (
	"fmt"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/rest"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/vm"
	"github.com/advpl/compiler/pkg/webui"
)

// serve_rest.go — montagem das rotas REST TOTVS no `advplc serve`.
//
// O bytecode compilado (fonte único ou diretório-projeto) é varrido por
// uma VM de REGISTRO que nunca executa Run(): ela só chama
// RegisterRestRoutes para descobrir @Get/@Post/... e classes
// WSRESTFUL/WSMETHOD e registrá-las num rest.Server. Cada requisição HTTP
// despacha numa VM-filha dessa VM de registro (newChildVM), com banco e
// estado isolados por requisição — mesmo modelo do WSRestServer nativo.
//
// O rest.Server fica atrás de um ponteiro atômico: com --watch a
// recompilação troca o servidor inteiro de uma vez, sem reiniciar o
// processo nem derrubar requisições em voo.

// montaRest registra as rotas REST do bytecode e devolve o servidor (nil
// quando não há nenhuma rota) e os erros de registro — impressos um por
// linha no console, sem abortar as rotas válidas.
func montaRest(bc *compiler.Bytecode, opts *Options) (*rest.Server, []error) {
	regVM := vm.NewVM(bc, false)
	attachDatabase(regVM, opts)
	srv := rest.NewServer("advpl-serve", "1.0")
	errs := regVM.RegisterRestRoutes(srv)
	if len(srv.Routes()) == 0 {
		return nil, errs
	}
	return srv, errs
}

// registraRest monta (ou remonta, no --watch) o rest.Server do bytecode
// corrente e guarda no ponteiro. Erros de registro vão para stderr.
func registraRest(bc *compiler.Bytecode, opts *Options, restSrv *atomic.Pointer[rest.Server]) {
	srv, errs := montaRest(bc, opts)
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "rest: %v\n", err)
	}
	restSrv.Store(srv)
}

// handlerRest delega cada requisição para o rest.Server corrente. Quando
// não há rotas (ou o bytecode não tem REST) responde 404 — montado ou não,
// o comportamento fora do prefixo REST não muda.
func handlerRest(restSrv *atomic.Pointer[rest.Server]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := restSrv.Load()
		if s == nil {
			http.NotFound(w, r)
			return
		}
		s.Handler().ServeHTTP(w, r)
	})
}

// montaRESTNoServe registra o REST inicial e, se houver prefixo
// configurado, monta no servidor web. Devolve o ponteiro atômico para o
// --watch trocar o servidor a cada recompilação.
func montaRESTNoServe(bc *compiler.Bytecode, opts *Options, srv *webui.Server) *atomic.Pointer[rest.Server] {
	var restSrv atomic.Pointer[rest.Server]
	registraRest(bc, opts, &restSrv)

	if prefix := shared.ResolveRestPrefix(); prefix != "" {
		srv.MountREST(prefix, handlerRest(&restSrv))
	}
	return &restSrv
}
