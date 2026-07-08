package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type OutputConsole struct {
	label   *widget.Label
	scroll  *container.Scroll
	output  []string
}

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

func (c *OutputConsole) Append(text string) {
	c.output = append(c.output, text)
	c.updateDisplay()
}

func (c *OutputConsole) Clear() {
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
