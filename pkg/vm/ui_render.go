package vm

import (
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerUiRenderNatives expõe primitivas visuais (lipgloss) para TUIs de
// terminal escritas em AdvPL/TLPP e compiladas com `advplc build` — o mesmo
// motivo de existir do pkg/ui (TerminalUIProvider dos diálogos legados),
// só que aqui como natives de baixo nível que o próprio programa AdvPL
// compõe, em vez de widgets prontos (MSDIALOG/FWGetText/Menu).
func (v *VM) registerUiRenderNatives(natives map[string]func([]advplrt.Value) (advplrt.Value, error)) {
	// UiBox(cTitle, cBody, cColor[, nWidth]) As Character: caixa com borda
	// arredondada (estilo opencode/Claude Code), título em negrito na
	// primeira linha. cColor é um código ANSI 256 ("39"=ciano, "212"=rosa,
	// "240"=cinza). nWidth omitido/0 = largura automática pelo conteúdo.
	// Só RENDERIZA a string (com os códigos ANSI já embutidos); quem chama
	// decide se imprime com ConOut/ConOutRaw.
	natives["UIBOX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		title := getArgString(args, 0, "")
		body := advplrt.ToString(getArg(args, 1))
		color := getArgString(args, 2, "39")
		width := int(advplrt.ToFloat(getArg(args, 3)))

		content := body
		if title != "" {
			titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color))
			content = titleStyle.Render(title) + "\n" + body
		}
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(color)).
			Padding(0, 1)
		if width > 0 {
			style = style.Width(width)
		}
		return advplrt.NewString(style.Render(content)), nil
	}

	// UiStreamBox(cTitle, cBodySoFar, cColor[, nWidth]): igual ao UiBox, mas
	// AUTO-REDESENHA — antes de imprimir a caixa nova, apaga com ANSI as
	// linhas da caixa anterior desenhada pela última chamada (a VM guarda a
	// altura em v.lastBoxLines). Uso: chamar de novo a cada delta de texto
	// do LLM, sempre com o texto acumulado até agora (não só o delta) — dá
	// o efeito de "cartão crescendo ao vivo" que opencode/Claude Code têm,
	// sem precisar de raw-mode de teclado nem redesenho de tela inteira.
	// Chame UiStreamReset() ao terminar o turno, senão o PRÓXIMO turno tenta
	// apagar por cima da caixa deste.
	natives["UISTREAMBOX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		title := getArgString(args, 0, "")
		body := advplrt.ToString(getArg(args, 1))
		color := getArgString(args, 2, "39")
		width := int(advplrt.ToFloat(getArg(args, 3)))

		content := body
		if title != "" {
			titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color))
			content = titleStyle.Render(title) + "\n" + body
		}
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(color)).
			Padding(0, 1)
		if width > 0 {
			style = style.Width(width)
		}
		rendered := style.Render(content)

		if v.lastBoxLines > 0 {
			// sobe N linhas, volta pra coluna 0, apaga da posição atual até
			// o fim da tela — remove a caixa anterior inteira de uma vez.
			fmt.Fprintf(stdoutW, "\x1b[%dA\r\x1b[0J", v.lastBoxLines)
		}
		fmt.Fprintln(stdoutW, rendered)
		v.lastBoxLines = strings.Count(rendered, "\n") + 1
		return advplrt.Nil, nil
	}

	// UiStreamReset(): zera o rastreador de altura do UiStreamBox — chamar
	// ao fechar um turno de streaming, antes do próximo elemento de tela
	// (senão o próximo UiStreamBox apaga a caixa errada).
	natives["UISTREAMRESET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		v.lastBoxLines = 0
		return advplrt.Nil, nil
	}

	// UiMarkdown(cMarkdown[, nWidth]) As Character: renderiza markdown
	// (negrito, itálico, listas, blocos de código com fundo destacado,
	// títulos) para texto ANSI de terminal via glamour — mesmo tratamento
	// visual que opencode/Claude Code dão para a resposta de um LLM. Estilo
	// fixo "dark" (não usa WithAutoStyle): auto-detecção de tema consulta o
	// terminal via OSC 11 e PODE TRAVAR em multiplexers/terminais que não
	// respondem a essa query — o mesmo problema que pkg/ui/terminal.go já
	// documentou e evitou para o lipgloss (SetHasDarkBackground(true)); aqui
	// a mesma cautela, sem query bloqueante. Em erro de parse (raro — texto
	// realmente malformado), devolve cMarkdown sem alteração, nunca falha.
	natives["UIMARKDOWN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		body := advplrt.ToString(getArg(args, 0))
		width := int(advplrt.ToFloat(getArg(args, 1)))
		if width <= 0 {
			width = 80
		}
		r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(width),
		)
		if err != nil {
			return advplrt.NewString(body), nil
		}
		out, err := r.Render(body)
		if err != nil {
			return advplrt.NewString(body), nil
		}
		return advplrt.NewString(strings.TrimRight(out, "\n")), nil
	}

	// UiAltScreenEnter(): entra na tela alternativa do terminal (o mesmo
	// buffer que vim/less/htop usam) — a saída normal do shell fica
	// preservada e volta exatamente como estava ao sair, dando o efeito de
	// "aplicativo cheio" em vez de um scroll de terminal comum. Instala
	// também um handler de Ctrl+C (os.Interrupt — portátil em
	// Linux/macOS/Windows, ao contrário de syscall.SIGTERM) que restaura a
	// tela normal antes de encerrar; sem isso, um Ctrl+C durante o app
	// deixaria o terminal do usuário preso na tela alternativa.
	natives["UIALTSCREENENTER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		fmt.Fprint(stdoutW, "\x1b[?1049h\x1b[H")
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		go func() {
			<-ch
			fmt.Fprint(stdoutW, "\x1b[?1049l")
			os.Exit(130)
		}()
		return advplrt.Nil, nil
	}

	// UiAltScreenExit(): sai da tela alternativa, restaurando o conteúdo
	// normal do terminal — chamar na saída normal (não Ctrl+C, que já tem
	// seu próprio restore via UiAltScreenEnter).
	natives["UIALTSCREENEXIT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		fmt.Fprint(stdoutW, "\x1b[?1049l")
		return advplrt.Nil, nil
	}

	// UiTermWidth([nDefault]) As Numeric: largura do terminal em colunas.
	// nDefault (padrão 80) é usado quando stdout não é um tty real (pipe,
	// redirecionamento) — term.GetSize falha nesse caso.
	natives["UITERMWIDTH"] = func(args []advplrt.Value) (advplrt.Value, error) {
		def := 80.0
		if len(args) > 0 {
			def = advplrt.ToFloat(getArg(args, 0))
		}
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			return advplrt.NewNumber(float64(w)), nil
		}
		return advplrt.NewNumber(def), nil
	}
}
