package ui

import (
	"regexp"

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
	{regexp.MustCompile(`(?i)unidade|apartamento|im[oó]vel`), theme.HomeIcon},
	{regexp.MustCompile(`(?i)condom[ií]nio|condom[ií]no|cliente|usu[aá]rio|pessoa`), theme.AccountIcon},
	{regexp.MustCompile(`(?i)despesa|financeiro|custo`), theme.DocumentCreateIcon},
	{regexp.MustCompile(`(?i)cobran[çc]a|fatura|boleto|conta`), theme.DocumentIcon},
	{regexp.MustCompile(`(?i)fechamento|compet[eê]ncia|m[eê]s`), theme.HistoryIcon},
	{regexp.MustCompile(`(?i)mala|e-?mail|correio|mensagem`), theme.MailComposeIcon},
	{regexp.MustCompile(`(?i)sair|encerrar|fechar|voltar`), theme.LogoutIcon},
}

func menuItemIcon(label string) fyne.Resource {
	for _, rule := range menuIconRules {
		if rule.re.MatchString(label) {
			return rule.icon()
		}
	}
	return theme.ListIcon()
}

type FyneUIProvider struct {
	window fyne.Window
}

func NewFyneUIProvider(window fyne.Window) *FyneUIProvider {
	return &FyneUIProvider{
		window: window,
	}
}

func (p *FyneUIProvider) MsgInfo(msg, title string) {
	if title == "" {
		title = "Information"
	}
	dialog.ShowInformation(title, msg, p.window)
}

func (p *FyneUIProvider) MsgStop(msg, title string) {
	if title == "" {
		title = "Error"
	}
	dialog.ShowError(&fyneError{msg: msg}, p.window)
}

func (p *FyneUIProvider) MsgAlert(msg, title string) {
	if title == "" {
		title = "Alert"
	}
	dialog.ShowInformation(title, msg, p.window)
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
	dlg.SetOnClosed(func() {
		if !sent {
			sent = true
			result <- 0
		}
	})
	dlg.Show()

	return <-result
}

// InputText pede um texto ao usuário via EntryDialog e bloqueia até a
// resposta (ou def, se cancelado). Mesma restrição de goroutine acima.
func (p *FyneUIProvider) InputText(prompt, def string) string {
	result := make(chan string, 1)
	sent := false
	entry := dialog.NewEntryDialog(prompt, "", func(text string) {
		if !sent {
			sent = true
			if text == "" {
				text = def
			}
			result <- text
		}
	}, p.window)
	entry.SetText(def)
	entry.SetOnClosed(func() {
		if !sent {
			sent = true
			result <- def
		}
	})
	entry.Show()

	return <-result
}

type fyneError struct {
	msg string
}

func (e *fyneError) Error() string {
	return e.msg
}
