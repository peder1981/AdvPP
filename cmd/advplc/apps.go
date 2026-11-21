package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/vm"
	"github.com/advpl/compiler/pkg/webui"
)

// compilado guarda o bytecode (ou o erro de compilação) de um app para a
// data de modificação do fonte; muda a data, recompila.
type compilado struct {
	mtime time.Time
	bc    *compiler.Bytecode
	err   error
}

// catalogo resolve "advplc serve --apps <dir>": cada <dir>/<nome>.prw|.tlpp
// é o app <nome>, compilado sob demanda.
type catalogo struct {
	dir, home string
	opts      *Options
	mu        sync.Mutex
	cache     map[string]compilado
}

func novoCatalogo(dir, home string, opts *Options) *catalogo {
	return &catalogo{dir: dir, home: home, opts: opts, cache: map[string]compilado{}}
}

// fonte acha o arquivo do app ("" = página inicial, o --home).
func (c *catalogo) fonte(app string) (string, error) {
	if app == "" {
		if c.home == "" {
			return "", webui.ErrAppNaoEncontrado
		}
		app = c.home
	}
	if !webui.NomeDeAppValido(app) {
		return "", webui.ErrAppNaoEncontrado
	}
	for _, ext := range []string{".prw", ".tlpp"} {
		p := filepath.Join(c.dir, app+ext)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", webui.ErrAppNaoEncontrado
}

// bytecode compila o fonte, ou devolve o resultado guardado se o arquivo não mudou.
func (c *catalogo) bytecode(path string) (*compiler.Bytecode, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if k, ok := c.cache[path]; ok && k.mtime.Equal(st.ModTime()) {
		return k.bc, k.err
	}
	bc, err := loadAndCompile(path, c.opts)
	c.cache[path] = compilado{mtime: st.ModTime(), bc: bc, err: err}
	return bc, err
}

// resolver liga o catálogo ao servidor: cada sessão, uma VM nova.
func (c *catalogo) resolver() webui.AppResolver {
	return func(app string) (webui.RunFunc, error) {
		path, err := c.fonte(app)
		if err != nil {
			return nil, err
		}
		nome := app
		if nome == "" {
			nome = c.home
		}
		return func(ui *webui.Provider, console *webui.OutWriter) error {
			bc, err := c.bytecode(path)
			if err != nil {
				return fmt.Errorf("erro de compilação em %s: %v", nome, err)
			}
			v := vm.NewVM(bc, true)
			v.SetUIProvider(ui)
			attachDatabase(v, c.opts)
			v.SetOutputWriter(console)
			defer v.CloseOwnedConnections()
			_, err = v.Run()
			return err
		}, nil
	}
}

// serveAppsPrepara valida o diretório e define ADVPP_APPS_DIR (caminho
// absoluto), que os programas usam para listar os apps com Directory().
func serveAppsPrepara(opts *Options) error {
	abs, err := filepath.Abs(opts.apps)
	if err != nil {
		return err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return fmt.Errorf("--apps: %s não é um diretório", opts.apps)
	}
	opts.apps = abs
	vm.DefinirIconesValidos(webui.Icones())
	return os.Setenv("ADVPP_APPS_DIR", abs)
}

// serveApps: advplc serve --apps <dir> [--home <nome>] [--port n].
func serveApps(opts *Options) error {
	if err := serveAppsPrepara(opts); err != nil {
		return err
	}
	if opts.watch || opts.debugPort != "" {
		fmt.Fprintln(os.Stderr, "aviso: --watch e --debug-port não se aplicam a --apps (os apps já recompilam ao mudar)")
	}
	c := novoCatalogo(opts.apps, opts.home, opts)
	port := shared.ResolveWebUIPort(opts.port)
	host := os.Getenv("ADVPL_SERVE_HOST")
	if host == "" {
		host = "localhost"
	}
	fmt.Printf("AdvPP apps: %s (início: %q)\n", opts.apps, opts.home)
	return webui.NewApps(filepath.Base(opts.apps), c.resolver()).Serve(host + ":" + port)
}
