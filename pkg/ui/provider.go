package ui

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// menuIconRules espelha a heurística do frontend web (web/src/app/app.ts,
// MENU_ICON_RULES) — mesmo critério de palavra-chave em português comum,
// só trocando os ícones AnDes (web) pelos ícones embutidos do Fyne
// (conjunto bem mais limitado, sem prédio/carteira/recibo/calendário
// próprios — usa o mais próximo semanticamente disponível). O ícone fica
// como func() fyne.Resource, não fyne.Resource já resolvido: os
// theme.XxxIcon() do Fyne chamam fyne.CurrentApp() por baixo, e este é um
// var de pacote — avaliado antes de app.New() rodar em main(), sem app
// nenhum "current" ainda, o que derrubava com "Attempt to access current
// Fyne app when none is started" a cada standalone build.
var menuIconRules = []struct {
	re   *regexp.Regexp
	icon func() fyne.Resource
}{
	// Checado primeiro: itens de relatório mencionam palavras (unidade,
	// despesa) que também disparariam as regras de tela CRUD abaixo —
	// "Extrato por Unidade" precisa do ícone de grid, não de casa.
	{regexp.MustCompile(`(?i)relat[oó]rio|balancete|inadimpl[eê]ncia|extrato|por categoria`), theme.GridIcon},
	{regexp.MustCompile(`(?i)unidade|apartamento|im[oó]vel`), theme.HomeIcon},
	{regexp.MustCompile(`(?i)cond[oô]m[ií]?n|cliente|usu[aá]rio|pessoa`), theme.AccountIcon},
	{regexp.MustCompile(`(?i)despesa|financeiro|custo`), theme.DocumentCreateIcon},
	{regexp.MustCompile(`(?i)cobran[çc]a|fatura|boleto|conta`), theme.DocumentIcon},
	{regexp.MustCompile(`(?i)fechamento|compet[eê]ncia|m[eê]s`), theme.HistoryIcon},
	{regexp.MustCompile(`(?i)mala|e-?mail|correio|mensagem`), theme.MailComposeIcon},
	{regexp.MustCompile(`(?i)senha`), theme.LoginIcon},
	{regexp.MustCompile(`(?i)voltar`), theme.NavigateBackIcon},
	{regexp.MustCompile(`(?i)sair|encerrar|fechar`), theme.LogoutIcon},
}

func menuItemIcon(label string) fyne.Resource {
	for _, rule := range menuIconRules {
		if rule.re.MatchString(label) {
			return rule.icon()
		}
	}
	return theme.ListIcon()
}

// FyneUIProvider represents a core AdvPP type.
type FyneUIProvider struct {
	window fyne.Window
}

// NewFyneUIProvider performs a core operation.
func NewFyneUIProvider(window fyne.Window) *FyneUIProvider {
	return &FyneUIProvider{
		window: window,
	}
}

// blockingDialog mostra um dialog.Dialog e bloqueia até ele ser
// fechado (botão OK ou X) — dialog.ShowInformation/ShowError só
// disparam a exibição e retornam na hora, sem esperar o usuário
// reconhecer nada. Sem isso, MsgInfo/MsgAlert/MsgStop chamados em
// sequência (ex.: MsgAlert de erro seguido por um novo FWMenuSelect
// no loop do GesCon) empilhavam um diálogo por cima do outro — a VM
// já tinha seguido em frente antes do usuário sequer ver o primeiro.
// Mesma restrição de goroutine do MsgYesNo abaixo.
func blockingDialog(d dialog.Dialog) {
	done := make(chan struct{}, 1)
	sent := false
	d.SetOnClosed(func() {
		if !sent {
			sent = true
			done <- struct{}{}
		}
	})
	d.Show()

	// Add timeout to detect if dialog fails to show or callback never fires
	select {
	case <-done:
		// Dialog closed normally
	case <-time.After(30 * time.Second):
		// Dialog timeout — user never closed it, or dialog failed to show
		if !sent {
			sent = true
		}
	}
}

func (p *FyneUIProvider) MsgInfo(msg, title string) {
	if title == "" {
		title = "Information"
	}
	blockingDialog(dialog.NewInformation(title, msg, p.window))
}

func (p *FyneUIProvider) MsgStop(msg, title string) {
	blockingDialog(dialog.NewError(&fyneError{msg: msg}, p.window))
}

func (p *FyneUIProvider) MsgAlert(msg, title string) {
	if title == "" {
		title = "Alert"
	}
	blockingDialog(dialog.NewInformation(title, msg, p.window))
}

// MsgYesNo blocks its calling goroutine until the user answers — same
// constraint as Dialog (see msdialog.go): dialog.ShowConfirm's callback
// only fires once Fyne's own event loop processes the click, so this must
// never be called from that same event loop goroutine, or it deadlocks.
// Safe here because the VM (the only caller) always runs on its own
// goroutine (see cmd/advpp-ide's run()), never directly on a menu handler.
func (p *FyneUIProvider) MsgYesNo(msg, title string) bool {
	if title == "" {
		title = "Confirm"
	}

	result := make(chan bool, 1)
	dialog.ShowConfirm(title, msg, func(confirmed bool) {
		result <- confirmed
	}, p.window)

	return <-result
}

// Menu mostra um botão por item numa caixa de diálogo custom e bloqueia
// até o usuário clicar um deles (ou fechar sem escolher, retornando 0).
// Mesma restrição de goroutine do MsgYesNo acima.
func (p *FyneUIProvider) Menu(items []string, title string) int {
	fmt.Fprintf(os.Stderr, "[FYNE] Menu: %d items, title=%q\n", len(items), title)
	if title == "" {
		title = "Menu"
	}
	result := make(chan int, 1)
	sent := false

	buttons := make([]fyne.CanvasObject, len(items))
	var dlg dialog.Dialog
	for i, label := range items {
		idx := i + 1 // 1-based, mesmo padrão de FWMenuSelect
		btn := widget.NewButtonWithIcon(label, menuItemIcon(label), func() {
			fmt.Fprintf(os.Stderr, "[FYNE] Menu button clicked: idx=%d\n", idx)
			if !sent {
				sent = true
				result <- idx
			}
			dlg.Hide()
		})
		btn.Alignment = widget.ButtonAlignLeading
		buttons[i] = btn
	}
	box := container.NewVBox(buttons...)
	// largura mínima — sem isso o diálogo encolhe pro tamanho do título
	// quando os itens são curtos, ficando espremido.
	content := container.NewGridWrap(fyne.NewSize(320, box.MinSize().Height), box)
	dlg = dialog.NewCustomWithoutButtons(title, content, p.window)
	fmt.Fprintf(os.Stderr, "[FYNE] Menu dialog created\n")
	dlg.SetOnClosed(func() {
		fmt.Fprintf(os.Stderr, "[FYNE] Menu dialog closed (sent=%v)\n", sent)
		if !sent {
			sent = true
			result <- 0
		}
	})
	fmt.Fprintf(os.Stderr, "[FYNE] Menu calling dlg.Show()\n")
	dlg.Show()
	fmt.Fprintf(os.Stderr, "[FYNE] Menu waiting for result...\n")

	selection := <-result
	fmt.Fprintf(os.Stderr, "[FYNE] Menu returning: %d\n", selection)
	return selection
}

// InputText pede um texto ao usuário e bloqueia até a resposta (ou def, se
// cancelado). Se bIsPassword for true, o entry exibe asteriscos/bolinhas ao
// digitar. Mesma restrição de goroutine acima.
func (p *FyneUIProvider) InputText(prompt, def string, bIsPassword bool) string {
	result := make(chan string, 1)
	sent := false

	content := widget.NewEntry()
	content.SetText(def)
	content.PlaceHolder = prompt

	if bIsPassword {
		content.Password = true
	}

	dlg := dialog.NewCustomWithoutButtons(prompt, content, p.window)
	dlg.SetOnClosed(func() {
		if !sent {
			sent = true
			val := content.Text
			if val == "" {
				val = def
			}
			result <- val
		}
	})

	btnOk := widget.NewButton("OK", func() {
		if !sent {
			sent = true
			val := content.Text
			if val == "" {
				val = def
			}
			result <- val
			dlg.Hide()
		}
	})
	dlg.SetButtons([]fyne.CanvasObject{btnOk})

	dlg.Show()
	return <-result
}

type fyneError struct {
	msg string
}

func (e *fyneError) Error() string {
	return e.msg
}
