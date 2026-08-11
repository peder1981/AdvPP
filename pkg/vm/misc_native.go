package vm

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerMiscNatives registra 17 funções miscelâneas das categorias TDN
// "Componentes-de-interface-visual" (13), "Conexao-TCP" (2),
// "Controle-de-erro" (1) e "Manipulacao-de-memoria" (1). Arquivo novo —
// não altera natives.go. Conflitos verificados por grep: nenhum dos nomes
// existia registrado (exceto __DeleteRmt, cujo comportamento já existia sob
// a chave "__DELETRMT" em memoria_native.go — aqui registrado com o nome
// AdvPL correto, "__DELETERMT", sem duplicar a implementação).
//
// Notas arquiteturais:
//   - Sem AppServer/SmartClient nem GUI nesta VM, as funções de UI/DLL são
//     honestas ao que o runtime consegue fazer: clipboard em buffer próprio
//     do processo (o clipboard do SO não é usado), janelas sem filhos (0),
//     repositório de resources vazio e DLLs "carregadas" apenas como
//     registros simulados (Linux não tem LoadLibrary).
//   - ExecInDLLRun/ExeDLLRun2/ExeDLLRun3 não podem chamar a função
//     ExecInClientDLL de uma DLL real: retornam "" / 0 (a DLL não produziu
//     saída) — ver docs/tdn-known-limitations.md.
//   - Saídas por referência (@lIsSSL, @lHasMPP, @cBuffer) não são graváveis
//     nesta VM — mesma limitação arquitetural das demais categorias.
//   - Em erro sempre é devolvido advplrt.Nil ou o código/.F. documentado
//     pela função, nunca erro Go como segundo retorno.
func (v *VM) registerMiscNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {

	// --- Componentes-de-interface-visual / Remote -------------------------

	// COPYTOCLIPBOARD(cTexto) -> Nil — coloca texto na área de transferência.
	// Decisão: o clipboard REAL do SO não é usado (a VM é headless e não deve
	// depender de xclip/xsel); o roundtrip COPY/PASTE é fiel em buffer próprio
	// do processo (documentado em docs/tdn-known-limitations.md).
	natives["COPYTOCLIPBOARD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		miscClipboardMu.Lock()
		miscClipboard = getArgString(args, 0, "")
		miscClipboardMu.Unlock()
		return advplrt.Nil, nil
	}

	// PASTEFROMCLIPBOARD() -> cTexto — retorna o texto da área de
	// transferência (o buffer do processo nesta VM).
	natives["PASTEFROMCLIPBOARD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		miscClipboardMu.Lock()
		s := miscClipboard
		miscClipboardMu.Unlock()
		return advplrt.NewString(s), nil
	}

	// SHELLEXECUTE(cAcao, cArquivo, cParam, cDirTrabalho, [nOpc]) -> nRet
	// Executa arquivo/comando no SO, sem aguardar (fire-and-forget), como o
	// ShellExecute real. Retorna > 32 em sucesso (HINSTANCE) ou código de
	// erro ShellExecute (0..32). Ação "open" com arquivo não-executável/URL
	// usa a associação padrão do SO (xdg-open/open/rundll32); executável é
	// disparado diretamente. nOpc (modo de janela) é aceito e ignorado (sem
	// GUI). Sem shell, para prevenir injeção de comando (CWE-78).
	natives["SHELLEXECUTE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		action := strings.ToLower(strings.TrimSpace(getArgString(args, 0, "")))
		file := getArgString(args, 1, "")
		param := getArgString(args, 2, "")
		dir := getArgString(args, 3, "")

		if file == "" {
			return advplrt.NewNumber(2), nil // SE_ERR_FNF
		}

		// startAndReturn dispara um comando sem aguardar o término; devolve
		// 33 (> 32 = sucesso) ou o código de erro de spawn.
		startAndReturn := func(argv []string, workdir string) (advplrt.Value, error) {
			cmd := exec.Command(argv[0], argv[1:]...)
			if workdir != "" {
				cmd.Dir = workdir
			}
			if err := cmd.Start(); err != nil {
				return advplrt.NewNumber(shellErrorCode(err)), nil
			}
			return advplrt.NewNumber(33), nil
		}

		if action == "open" {
			// Arquivo não-executável ou URL: abre pela associação padrão.
			if !isExecutableFile(file) {
				if !isURLFile(file) {
					if _, err := os.Stat(file); err != nil {
						return advplrt.NewNumber(2), nil // SE_ERR_FNF
					}
				}
				return startAndReturn([]string{shellOpenCommand(), file}, dir)
			}
		}
		argv := []string{file}
		if param != "" {
			argv = append(argv, strings.Fields(param)...)
		}
		return startAndReturn(argv, dir)
	}

	// WINEXEC(cExec) -> cRet — executa aplicação externa na estação, sem
	// aguardar. Retorna 0 em sucesso, ou número != 0 indicando erro de OS
	// (2 = arquivo não encontrado, 5 = acesso negado). Mesmo parsing sem
	// shell do WAITRUN (operadores do shell não são suportados).
	natives["WINEXEC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cmdStr := getArgString(args, 0, "")
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			return advplrt.NewNumber(2), nil
		}
		cmd := exec.Command(parts[0], parts[1:]...)
		if err := cmd.Start(); err != nil {
			if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
				return advplrt.NewNumber(2), nil
			}
			return advplrt.NewNumber(5), nil // ERROR_ACCESS_DENIED
		}
		return advplrt.NewNumber(0), nil
	}

	// TONE() -> lRet — dispara um sinal sonoro. Sem áudio nesta VM, o bell
	// do terminal (\a) é o sinal honesto; retorna .T. pois o comando foi
	// efetivamente emitido (spec 17.1+).
	natives["TONE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		fmt.Fprint(stdoutW, "\a")
		return advplrt.True, nil
	}

	// --- Componentes-de-interface-visual / Funcoes-genericas -------------

	// EXECINDLLOPEN(cDLLName) -> nHandle — "abre" uma DLL. Sem LoadLibrary
	// (Linux), o registro é simulado: nome (base, maiúsculo) -> handle.
	// Reabrir a mesma DLL devolve o mesmo handle (idempotente).
	natives["EXECINDLLOPEN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		name := getArgString(args, 0, "")
		if name == "" {
			return advplrt.Nil, nil
		}
		key := strings.ToUpper(filepath.Base(name))
		miscDLLsMu.Lock()
		defer miscDLLsMu.Unlock()
		if h, ok := miscDLLs[key]; ok {
			return advplrt.NewNumber(float64(h)), nil
		}
		miscNextDLL++
		miscDLLs[key] = miscNextDLL
		miscDLLHandles[miscNextDLL] = name
		return advplrt.NewNumber(float64(miscNextDLL)), nil
	}

	// EXECINDLLCLOSE(nHandle) -> Nil — encerra a "conexão" com a DLL.
	natives["EXECINDLLCLOSE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		h := int64(advplrt.ToFloat(getArg(args, 0)))
		miscDLLsMu.Lock()
		defer miscDLLsMu.Unlock()
		if name, ok := miscDLLHandles[h]; ok {
			delete(miscDLLHandles, h)
			delete(miscDLLs, strings.ToUpper(filepath.Base(name)))
		}
		return advplrt.Nil, nil
	}

	// EXECINDLLRUN(nHandle, nOpc, cStrInput) -> cRet — executa a função
	// ExecInClientDLL numa DLL carregada. Sem DLL real não há função a
	// chamar: devolve buffer vazio (a DLL não escreveu retorno — até 255
	// bytes na spec). Handle inválido -> Nil (a chamada real falharia).
	natives["EXECINDLLRUN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		h := int64(advplrt.ToFloat(getArg(args, 0)))
		miscDLLsMu.Lock()
		_, ok := miscDLLHandles[h]
		miscDLLsMu.Unlock()
		if !ok {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(""), nil
	}

	// EXEDLLRUN2(nHandle, nOpc, cBuffer) -> nRet — retorno numérico (int) de
	// ExecInClientDLL. Sem DLL real, 0. @cBuffer é input/output por referência
	// (não gravável nesta VM). Handle inválido -> Nil.
	// EXEDLLRUN3 idem, para a assinatura distinta da DLL (buffOutputLen).
	for _, fn := range []string{"EXEDLLRUN2", "EXEDLLRUN3"} {
		natives[fn] = func(args []advplrt.Value) (advplrt.Value, error) {
			h := int64(advplrt.ToFloat(getArg(args, 0)))
			miscDLLsMu.Lock()
			_, ok := miscDLLHandles[h]
			miscDLLsMu.Unlock()
			if !ok {
				return advplrt.Nil, nil
			}
			return advplrt.NewNumber(0), nil
		}
	}

	// GETCHILDCT(oWindow) -> nChildrens — número de objetos filhos de uma
	// janela TWindow/TDialog. Sem GUI/janelas nesta VM, uma janela não tem
	// filhos -> 0; parâmetro inválido (nil) -> -1 (warning no log real).
	natives["GETCHILDCT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if advplrt.IsNil(getArg(args, 0)) {
			return advplrt.NewNumber(-1), nil
		}
		return advplrt.NewNumber(0), nil
	}

	// GETCLIENTDIR() -> cRet — diretório do SmartClient; honesto = diretório
	// de trabalho do processo (os.Getwd), já que não há SmartClient nesta VM.
	natives["GETCLIENTDIR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		dir, err := os.Getwd()
		if err != nil {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(dir), nil
	}

	// GETRESARRAY(cMask, [nRPO]) -> aRet — recursos do repositório que casam
	// com a máscara (ex.: "*.png"). AdvPP não embute container de resources
	// (.PER) no bytecode nem mantém repositório indexado (ver GetApoRes em
	// rpo_native.go), então a busca honesta devolve array vazio.
	// NOTA: a função NÃO é sobre resolução de tela (isso é GetScreenRes,
	// página stub no TDN e função distinta).
	natives["GETRESARRAY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewArray([]advplrt.Value{}), nil
	}

	// --- Conexao-TCP -------------------------------------------------------

	// GETPORT(nType, [lIsSSL], [lHasMPP]) -> nPort — porta que o servidor
	// (app/license/http/https) escuta. Sem servidor nesta VM, a porta não
	// está habilitada -> -1 (spec), exceto a porta do AppServer (tipo 1),
	// que pode ser informada via env ADVPP_PORT. nType fora de 1..4 -> -1.
	// lIsSSL/lHasMPP são saídas por referência — não graváveis nesta VM.
	natives["GETPORT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nType := int(advplrt.ToFloat(getArg(args, 0)))
		if nType < 1 || nType > 4 {
			return advplrt.NewNumber(-1), nil
		}
		if nType == 1 {
			if p := os.Getenv("ADVPP_PORT"); p != "" {
				if port, err := strconv.Atoi(p); err == nil {
					return advplrt.NewNumber(float64(port)), nil
				}
			}
		}
		return advplrt.NewNumber(-1), nil
	}

	// PING([...]) -> retorno variável. Duas formas:
	//   PING(nReq) — spec TDN: latência média (ms) AppServer<->SmartClient.
	//     Nesta VM (processo único, sem hop de rede) a latência é ~0 -> 0.
	//   PING(cHost, nPort[, nTimeOut]) — sonda TCP real com net.DialTimeout:
	//     .T. se conecta, .F. caso contrário (extensão prática para testar
	//     conectividade, já que não há SmartClient remoto para medir).
	natives["PING"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if _, ok := getArg(args, 0).(*advplrt.NumberValue); ok {
			return advplrt.NewNumber(0), nil
		}
		host := getArgString(args, 0, "localhost")
		port := int(advplrt.ToFloat(getArg(args, 1)))
		timeoutMS := advplrt.ToFloat(getArg(args, 2))
		if timeoutMS <= 0 {
			timeoutMS = 2000
		}
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, time.Duration(timeoutMS)*time.Millisecond)
		if err != nil {
			return advplrt.False, nil
		}
		conn.Close()
		return advplrt.True, nil
	}

	// --- Controle-de-erro --------------------------------------------------

	// ERRORBLOCK([bErrorHandler]) -> bRet — recupera e/ou define o bloco de
	// tratamento de erro corrente. Sem infraestrutura de exceções no VM, o
	// handler é mantido em estado global do pacote: com argumento, define o
	// novo handler e devolve o anterior; sem argumento, devolve o corrente.
	natives["ERRORBLOCK"] = func(args []advplrt.Value) (advplrt.Value, error) {
		miscErrorBlockMu.Lock()
		defer miscErrorBlockMu.Unlock()
		prev := miscErrorBlock
		if len(args) > 0 {
			miscErrorBlock = args[0]
		}
		if prev == nil {
			return advplrt.Nil, nil
		}
		return prev, nil
	}

	// --- Manipulacao-de-memoria --------------------------------------------

	// __DELETERMT(cIdentificador) -> Nil — exclui a lista de conteúdo de
	// variáveis criada por __SaveRmt(). Remove do armazenamento remoto
	// (v.remoteMemory), o mesmo store da __DeleteRmt já existente em
	// memoria_native.go — registrada aqui com o nome AdvPL correto da spec
	// TDN (__DeleteRmt), evitando duplicação de comportamento.
	natives["__DELETERMT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		identifier := getArgString(args, 0, "")
		delete(v.remoteMemory, identifier)
		return advplrt.Nil, nil
	}
}

// Estado global do pacote compartilhado entre as natives acima.

// Clipboard simulado do processo (roundtrip COPYTOCLIPBOARD/PASTEFROMCLIPBOARD).
// Decisão: NÃO se usa o clipboard real do SO (xclip/xsel no Linux) — a VM é
// headless e não deve depender de utilitário externo; o roundtrip é fiel
// dentro do processo.
var (
	miscClipboardMu sync.Mutex
	miscClipboard   string
)

// DLLs simuladas: nome normalizado (base, maiúsculo) -> handle e handle -> nome.
var (
	miscDLLsMu     sync.Mutex
	miscDLLs       = map[string]int64{}
	miscDLLHandles = map[int64]string{}
	miscNextDLL    int64
)

// miscErrorBlock é o bloco de tratamento de erro corrente do ErrorBlock.
var (
	miscErrorBlockMu sync.Mutex
	miscErrorBlock   advplrt.Value
)

// shellOpenCommand devolve o comando de "abrir com a associação padrão" do SO.
func shellOpenCommand() string {
	switch runtime.GOOS {
	case "windows":
		return "rundll32"
	case "darwin":
		return "open"
	default:
		return "xdg-open"
	}
}

// isURLFile indica se cArquivo é uma URL (aberta pelo navegador padrão).
func isURLFile(s string) bool {
	l := strings.ToLower(s)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// isExecutableFile indica se o caminho é um executável (direto ou no PATH).
func isExecutableFile(path string) bool {
	if !strings.ContainsAny(path, `/\`) {
		_, err := exec.LookPath(path)
		return err == nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0
}

// shellErrorCode mapeia erro de spawn para código ShellExecute (0..32):
// 2 = SE_ERR_FNF (arquivo não encontrado), 5 = SE_ERR_ACCESSDENIED.
func shellErrorCode(err error) float64 {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return 2 // SE_ERR_FNF
	}
	return 5 // SE_ERR_ACCESSDENIED
}
