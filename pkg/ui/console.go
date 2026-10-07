package ui

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// OutputConsole represents a core AdvPP type.
type OutputConsole struct {
	mu     sync.Mutex
	label  *widget.Label
	scroll *container.Scroll
	output []string
}

// NewOutputConsole performs a core operation.
func NewOutputConsole() *OutputConsole {
	label := widget.NewLabel("")
	label.Wrapping = fyne.TextWrapWord
	label.TextStyle = fyne.TextStyle{Monospace: true}

	scroll := container.NewScroll(label)
	scroll.SetMinSize(fyne.NewSize(0, 150))

	return &OutputConsole{
		label:  label,
		scroll: scroll,
		output: make([]string, 0),
	}
}

func (c *OutputConsole) GetWidget() fyne.CanvasObject {
	return c.scroll
}

// Append é seguro para concorrência: o botão "Perguntar ao PiG" publica
// respostas de uma goroutine (a inferência não pode travar a UI).
func (c *OutputConsole) Append(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.output = append(c.output, text)
	c.updateDisplay()
}

func (c *OutputConsole) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.output = make([]string, 0)
	c.label.SetText("")
}

func (c *OutputConsole) updateDisplay() {
	display := ""
	for _, line := range c.output {
		display += line + "\n"
	}
	c.label.SetText(display)
	c.scroll.ScrollToBottom()
}
