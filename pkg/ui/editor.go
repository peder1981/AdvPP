package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type CodeEditor struct {
	entry    *widget.Entry
	filename string
	modified bool
}

var advplKeywords = []string{
	"USER FUNCTION", "STATIC FUNCTION", "FUNCTION", "RETURN",
	"IF", "ELSEIF", "ELSE", "ENDIF",
	"FOR", "TO", "STEP", "NEXT",
	"WHILE", "ENDDO",
	"DO CASE", "CASE", "OTHERWISE", "ENDCASE",
	"BEGIN SEQUENCE", "RECOVER", "END SEQUENCE",
	"TRY", "CATCH", "FINALLY", "ENDTRY",
	"LOCAL", "PRIVATE", "PUBLIC", "STATIC",
	"CLASS", "ENDCLASS", "DATA", "METHOD", "CONSTRUCTOR",
	"NAMESPACE", "USING", "INTERFACE", "ENDINTERFACE",
	"IMPLEMENTS", "FROM", "AS",
	"NEW", "SELF", "NIL",
	"AND", "OR", "NOT",
	"EXIT", "LOOP", "BREAK", "THROW",
}

var advplTypes = []string{
	"CHARACTER", "NUMERIC", "LOGICAL", "DATE", "ARRAY",
	"INTEGER", "DOUBLE", "DECIMAL", "CODEBLOCK", "OBJECT",
	"JSON", "VARIANT", "VARIADIC", "STRING", "BOOLEAN",
}

func NewCodeEditor() *CodeEditor {
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder("Enter your AdvPL/TLPP code here...")
	entry.Wrapping = fyne.TextWrapWord

	return &CodeEditor{
		entry:    entry,
		filename: "",
		modified: false,
	}
}

func (e *CodeEditor) GetContent() string {
	return e.entry.Text
}

func (e *CodeEditor) SetContent(text string) {
	e.entry.SetText(text)
	e.modified = false
}

func (e *CodeEditor) GetWidget() fyne.CanvasObject {
	return e.entry
}

func (e *CodeEditor) SetFilename(name string) {
	e.filename = name
}

func (e *CodeEditor) GetFilename() string {
	return e.filename
}

func (e *CodeEditor) IsModified() bool {
	return e.modified
}

func (e *CodeEditor) SetModified(modified bool) {
	e.modified = modified
}
