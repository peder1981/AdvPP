package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

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

type fyneError struct {
	msg string
}

func (e *fyneError) Error() string {
	return e.msg
}
