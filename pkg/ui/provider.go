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

func (p *FyneUIProvider) MsgYesNo(msg, title string) bool {
	if title == "" {
		title = "Confirm"
	}
	
	result := false
	dialog.ShowConfirm(title, msg, func(confirmed bool) {
		result = confirmed
	}, p.window)
	
	return result
}

type fyneError struct {
	msg string
}

func (e *fyneError) Error() string {
	return e.msg
}
