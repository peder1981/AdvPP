package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/rest"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/webui"
)

// serve_pouiapp.go — `advplc serve --poui-app <dir|arquivo.app> [fonte]`.
//
// Hospeda um app PO-UI externo (pasta ou .app) sem fonte AdvPL própria:
// a raiz "/" redireciona para /poui/<nome>/?sid=<sid> com uma sessão de
// servidor (sem VM) — jsToAdvpl aceitos e descartados, close desregistra.
// Com fonte/diretório informado, o REST da onda 1 e os endpoints de
// framework (Task 2.5) também sobem, como no serveFile.

// extraiPouiAppApp reconhece `--poui-app <app>` (ou --poui-app=<app>) como
// primeiro argumento do serve e devolve o app e o fonte posicional
// opcional. O restante dos argumentos segue para parseOptions.
func extraiPouiApp(args []string) (app, fonte string, ok bool) {
	if len(args) == 0 {
		return "", "", false
	}
	var i int
	if strings.HasPrefix(args[0], "--poui-app=") {
		app = strings.TrimPrefix(args[0], "--poui-app=")
		i = 1
	} else if args[0] == "--poui-app" {
		if len(args) < 2 {
			return "", "", true // app vazio: erro adiante
		}
		app = args[1]
		i = 2
	} else {
		return "", "", false
	}
	if i < len(args) && !strings.HasPrefix(args[i], "-") {
		fonte = args[i]
	}
	return app, fonte, true
}

// servePouiApp sobe o servidor do app externo.
func servePouiApp(appPath, fonte string, opts *Options) error {
	if appPath == "" {
		return fmt.Errorf("--poui-app requer um diretório ou arquivo .app")
	}
	maxZip := int64(64 << 20)
	if cfg, err := shared.LoadConfig(); err == nil && cfg.PouiAppMaxMB > 0 {
		maxZip = cfg.PouiAppMaxMB << 20
	}
	app, err := webui.AbrePouiApp(appPath, maxZip)
	if err != nil {
		return err
	}

	srv := webui.New("poui-app:"+app.Nome,
		func(ui *webui.Provider, console *webui.OutWriter) error {
			// Sessões SPA não são usadas no modo --poui-app (a raiz
			// redireciona para o app); se alguém conectar /events
			// direto, a sessão termina imediatamente.
			return nil
		})

	// Fonte opcional: mesmo pipeline do serveFile — compila e monta o REST
	// (framework + rotas do usuário) atrás do ponteiro atômico do --watch.
	// Sem fonte, só os endpoints de framework sobem (apps externos ainda
	// precisam de companies/users/genericList).
	var restPtr *atomic.Pointer[rest.Server]
	if fonte != "" {
		var deps atomic.Value
		compile := func() (*compiler.Bytecode, error) {
			bc, d, err := compilaAlvoDeps(fonte, opts)
			if err == nil {
				deps.Store(d)
			}
			return bc, err
		}
		bc, err := compile()
		if err != nil {
			return err
		}
		var current atomic.Pointer[compiler.Bytecode]
		current.Store(bc)
		restPtr = montaRESTNoServe(bc, opts, srv)
		if opts.watch {
			go watchSource(fonte, compile, &deps, &current, srv, func(bc *compiler.Bytecode) {
				registraRest(bc, opts, restPtr)
			})
		}
	} else {
		var p atomic.Pointer[rest.Server]
		empty := rest.NewServer("advpl-serve", "1.0")
		registraFrameworkRoutes(empty, opts)
		p.Store(empty)
		restPtr = &p
		if prefix := shared.ResolveRestPrefix(); prefix != "" {
			srv.MountREST(prefix, handlerRest(restPtr))
		}
	}

	inner := srv.Handler()
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			sid, err := srv.NovaSessaoPoui(app, "http://"+r.Host)
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/poui/%s/?sid=%s&app=%s", app.Nome, sid, app.Nome), http.StatusFound)
			return
		}
		inner.ServeHTTP(w, r)
	})

	port := shared.ResolveWebUIPort(opts.port)
	host := os.Getenv("ADVPL_SERVE_HOST")
	if host == "" {
		host = "localhost"
	}
	return srv.ServeHandler(host+":"+port, root)
}
