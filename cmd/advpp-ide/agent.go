package main

// Integração "Perguntar ao PiG" (Passo 3 do plano pig-advpp).
//
// Fluxo: menu Tools -> diálogo com a pergunta -> executa
// `pig -p --mode json [--model ...] <prompt>` em goroutine (para não
// congelar a UI durante a inferência) com o arquivo atual como contexto,
// extrai a resposta final do stream JSONL e anexa ao console de saída.
//
// Configuração por ambiente (nenhum segredo vai para o repo):
//
//	ADVPP_PIG_BIN      binário pig (default: "pig", resolvido ao lado do
//	                   advpp-ide e depois no PATH, como adveditorPath)
//	ADVPP_PIG_MODEL    ex. "ollama-native/granite-bash-ninja-shadow-3b:latest"
//	                   (opcional; sem ele vale o default do pig)
//	ADVPP_PIG_TIMEOUT_SECS  default 300
//
// Notas de segurança: nunca passa -a/--approve sozinho (a decisão de trust
// do projeto é do usuário); roda com --no-session para não acumular
// sessões de Q&A; usa Dir = pasta do arquivo para que MCP/settings do
// projeto se apliquem quando o projeto for confiável.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	agentMaxContextChars = 12000
	agentDefaultTimeout  = 300 * time.Second
)

// askPigDialog abre o diálogo da pergunta na thread da UI.
func (ide *IDE) askPigDialog() {
	filename := ide.editor.GetFilename()
	if filename == "" {
		filename = "untitled.prw"
	}
	info := widget.NewLabel("Arquivo em contexto: " + filename)
	info.Wrapping = fyne.TextWrapWord
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder("Ex.: explique este erro do compilador / revise esta rotina / gere um teste para…")
	entry.SetMinRowsVisible(4)

	form := dialog.NewForm("Perguntar ao PiG", "Enviar", "Cancelar",
		[]*widget.FormItem{
			widget.NewFormItem("Contexto", info),
			widget.NewFormItem("Pergunta", entry),
		}, func(confirmed bool) {
			if !confirmed {
				return
			}
			question := strings.TrimSpace(entry.Text)
			if question == "" {
				dialog.ShowError(fmt.Errorf("digite uma pergunta"), ide.window)
				return
			}
			ide.output.Append("PiG: consultando (" + filename + ")…")
			go ide.askPig(question, filename, ide.editor.GetContent())
		}, ide.window)
	form.Resize(fyne.NewSize(560, 320))
	form.Show()
}

// askPig executa o pig fora da thread da UI e publica o resultado no console
// (OutputConsole.Append agora é seguro para concorrência).
func (ide *IDE) askPig(question, filename, content string) {
	answer, err := runPigAgent(question, filename, content)
	if err != nil {
		ide.output.Append("PiG: erro: " + err.Error())
		return
	}
	ide.output.Append("PiG respondeu (" + filename + "):")
	for _, line := range strings.Split(answer, "\n") {
		ide.output.Append("  " + line)
	}
}

// pigSettings resolve binário/modelo/timeout a partir do ambiente.
func pigSettings() (bin, model string, timeout time.Duration) {
	bin = os.Getenv("ADVPP_PIG_BIN")
	if bin == "" {
		bin = "pig"
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), exeName("pig"))
			if _, statErr := os.Stat(candidate); statErr == nil {
				bin = candidate
			}
		}
		if _, err := exec.LookPath(bin); err != nil {
			if p, lerr := exec.LookPath("pig"); lerr == nil {
				bin = p
			}
		}
	}
	model = os.Getenv("ADVPP_PIG_MODEL")
	timeout = agentDefaultTimeout
	if s := os.Getenv("ADVPP_PIG_TIMEOUT_SECS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	return bin, model, timeout
}

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// buildAgentPrompt monta o prompt com o arquivo em contexto (truncado).
func buildAgentPrompt(question, filename, content string) string {
	var sb strings.Builder
	sb.WriteString(question)
	sb.WriteString("\n\n[Contexto do IDE AdvPP]\nArquivo: ")
	sb.WriteString(filename)
	sb.WriteString("\n")
	if content != "" {
		if len(content) > agentMaxContextChars {
			content = content[:agentMaxContextChars] + "\n[...trecho truncado pelo IDE...]"
		}
		sb.WriteString("Conteúdo:\n```\n")
		sb.WriteString(content)
		sb.WriteString("\n```\n")
	}
	return sb.String()
}

// runPigAgent invoca `pig -p --mode json` e devolve o texto final.
func runPigAgent(question, filename, content string) (string, error) {
	bin, model, timeout := pigSettings()
	if _, err := exec.LookPath(bin); err != nil {
		// bin pode ser caminho absoluto já resolvido.
		if _, serr := os.Stat(bin); serr != nil {
			return "", fmt.Errorf("binário pig não encontrado (ADVPP_PIG_BIN=%q): %w", os.Getenv("ADVPP_PIG_BIN"), err)
		}
	}
	args := []string{"-p", "--mode", "json", "--no-session"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, buildAgentPrompt(question, filename, content))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir := filepath.Dir(filename); dir != "." && dir != "" {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			cmd.Dir = dir
		}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("pig falhou: %s", firstLines(msg, 4))
	}
	answer, err := extractFinalAnswer(stdout.Bytes())
	if err != nil {
		return "", err
	}
	return answer, nil
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// agentMessage espelha o envelope mínimo do --mode json que nos interessa.
type agentMessage struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// extractFinalAnswer devolve o texto final do stream JSONL: o registro
// message_end do assistant é autoritativo (docs json.md); na ausência dele,
// usa o turn_end; sem nenhum dos dois, erro (não inventa resposta).
func extractFinalAnswer(stream []byte) (string, error) {
	var lastTurnText string
	turns := 0
	sc := bufio.NewScanner(bytes.NewReader(stream))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var m agentMessage
		if err := json.Unmarshal(line, &m); err != nil {
			continue // ignora linhas fora do protocolo (ruído)
		}
		if m.Type == "message_end" && m.Message != nil && m.Message.Role == "assistant" {
			if text := joinTextParts(m.Message.Content); text != "" {
				return text, nil
			}
		}
		if m.Type == "turn_end" {
			turns++
			// turn_end pode carregar a mensagem final; guarda como fallback.
			var envelope struct {
				Message *struct {
					Role    string `json:"role"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			}
			if err := json.Unmarshal(line, &envelope); err == nil && envelope.Message != nil {
				if text := joinTextParts(envelope.Message.Content); text != "" {
					lastTurnText = text
				}
			}
		}
	}
	if lastTurnText != "" {
		return lastTurnText, nil
	}
	return "", fmt.Errorf("pig não retornou resposta final (%d turnos observados)", turns)
}

func joinTextParts(parts []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) string {
	var sb strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			sb.WriteString(p.Text)
		}
	}
	return strings.TrimSpace(sb.String())
}
