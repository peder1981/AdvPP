package ui

import (
	"image/color"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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

// keywordSet/typeSet: multi-word entries above (e.g. "DO CASE") are split into
// individual words since the highlighter tokenizes one identifier at a time.
var keywordSet = wordSet(advplKeywords)
var typeSet = wordSet(advplTypes)

func wordSet(entries []string) map[string]bool {
	set := make(map[string]bool)
	for _, entry := range entries {
		for _, word := range strings.Fields(entry) {
			set[word] = true
		}
	}
	return set
}

// highlightPattern tokenizes AdvPL/TLPP source for syntax coloring. It is
// intentionally lexer-light (no preprocessor/string-escape awareness) since
// it only drives a cosmetic preview, not compilation.
var highlightPattern = regexp.MustCompile(`(?sm)` +
	`(?P<comment>//[^\n]*|/\*.*?\*/)` +
	`|(?P<string>"[^"\n]*"|'[^'\n]*'|\[[^\]\n]*\])` +
	`|(?P<directive>^[ \t]*#[A-Za-z]+[^\n]*)` +
	`|(?P<number>\b\d+(\.\d+)?\b)` +
	`|(?P<ident>\b[A-Za-z_]\w*\b)`)

var (
	styleComment   = widget.RichTextStyle{ColorName: theme.ColorNamePlaceHolder, Inline: true, SizeName: theme.SizeNameText, TextStyle: fyne.TextStyle{Monospace: true, Italic: true}}
	styleString    = widget.RichTextStyle{ColorName: theme.ColorNameSuccess, Inline: true, SizeName: theme.SizeNameText, TextStyle: fyne.TextStyle{Monospace: true}}
	styleDirective = widget.RichTextStyle{ColorName: theme.ColorNameError, Inline: true, SizeName: theme.SizeNameText, TextStyle: fyne.TextStyle{Monospace: true}}
	styleNumber    = widget.RichTextStyle{ColorName: theme.ColorNameWarning, Inline: true, SizeName: theme.SizeNameText, TextStyle: fyne.TextStyle{Monospace: true}}
	styleKeyword   = widget.RichTextStyle{ColorName: theme.ColorNamePrimary, Inline: true, SizeName: theme.SizeNameText, TextStyle: fyne.TextStyle{Monospace: true, Bold: true}}
	styleType      = widget.RichTextStyle{ColorName: theme.ColorNameHyperlink, Inline: true, SizeName: theme.SizeNameText, TextStyle: fyne.TextStyle{Monospace: true}}
	stylePlain     = widget.RichTextStyle{ColorName: theme.ColorNameForeground, Inline: true, SizeName: theme.SizeNameText, TextStyle: fyne.TextStyle{Monospace: true}}
)

// highlightSegments converts source text into colored RichText segments.
func highlightSegments(text string) []widget.RichTextSegment {
	if text == "" {
		return []widget.RichTextSegment{&widget.TextSegment{Text: "", Style: stylePlain}}
	}

	segments := make([]widget.RichTextSegment, 0, 64)
	names := highlightPattern.SubexpNames()
	pos := 0

	appendPlain := func(s string) {
		if s == "" {
			return
		}
		segments = append(segments, &widget.TextSegment{Text: s, Style: stylePlain})
	}

	for _, m := range highlightPattern.FindAllStringSubmatchIndex(text, -1) {
		start, end := m[0], m[1]
		appendPlain(text[pos:start])

		matched := text[start:end]
		style := stylePlain
		switch {
		case groupMatched(m, names, "comment"):
			style = styleComment
		case groupMatched(m, names, "string"):
			style = styleString
		case groupMatched(m, names, "directive"):
			style = styleDirective
		case groupMatched(m, names, "number"):
			style = styleNumber
		case groupMatched(m, names, "ident"):
			upper := strings.ToUpper(matched)
			if keywordSet[upper] {
				style = styleKeyword
			} else if typeSet[upper] {
				style = styleType
			}
		}
		segments = append(segments, &widget.TextSegment{Text: matched, Style: style})
		pos = end
	}
	appendPlain(text[pos:])

	return segments
}

func groupMatched(m []int, names []string, name string) bool {
	for i, n := range names {
		if n == name {
			return m[2*i] != -1
		}
	}
	return false
}

// highlightEntry is a widget.Entry that reports focus loss so the editor can
// swap to the colorized preview. Overriding FocusLost only takes effect
// because ExtendBaseWidget(e) is called with the subclass, not the embedded
// Entry, in newHighlightEntry.
type highlightEntry struct {
	widget.Entry
	onFocusLost func()
}

func newHighlightEntry(onFocusLost func()) *highlightEntry {
	e := &highlightEntry{onFocusLost: onFocusLost}
	e.ExtendBaseWidget(e)
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.TextStyle = fyne.TextStyle{Monospace: true}
	return e
}

func (e *highlightEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onFocusLost != nil {
		e.onFocusLost()
	}
}

// tapOverlay is an invisible widget placed over the colorized preview so a
// single click switches the editor back into edit mode.
type tapOverlay struct {
	widget.BaseWidget
	onTapped func()
}

func newTapOverlay(onTapped func()) *tapOverlay {
	o := &tapOverlay{onTapped: onTapped}
	o.ExtendBaseWidget(o)
	return o
}

func (o *tapOverlay) Tapped(*fyne.PointEvent) {
	if o.onTapped != nil {
		o.onTapped()
	}
}

func (o *tapOverlay) CreateRenderer() fyne.WidgetRenderer {
	// Transparent: this sits on top of the colorized preview in the stack
	// (later objects paint over earlier ones) purely to catch the tap that
	// switches back to edit mode — an opaque rectangle here would visually
	// cover the very text it's supposed to let you click through to.
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

// CodeEditor is a syntax-highlighting AdvPL/TLPP source editor. It shows a
// colorized read-only preview (widget.RichText) while unfocused and swaps to
// a plain, fully-native widget.Entry for actual editing — Fyne's Entry has
// no per-token color support, so live-colored keystrokes aren't possible
// without reimplementing text editing from scratch; toggling on focus keeps
// editing behavior 100% native (cursor, selection, IME, clipboard) while
// still giving a real colorized view whenever the user isn't actively typing
// (right after opening a file, or after tabbing/clicking away).
type CodeEditor struct {
	entry    *highlightEntry
	preview  *widget.RichText
	overlay  *tapOverlay
	stack    *fyne.Container
	filename string
	modified bool
}

func NewCodeEditor() *CodeEditor {
	e := &CodeEditor{}

	e.preview = widget.NewRichText()
	e.preview.Wrapping = fyne.TextWrapWord
	// RichText only wires its internal BaseWidget.impl lazily, on its first
	// CreateRenderer()/MinSize() call. Forcing that here (rather than
	// leaving it to happen whenever the widget first gets painted) avoids
	// depending on paint-order timing once it starts getting swapped in and
	// out of e.stack.Objects below.
	e.preview.MinSize()

	e.overlay = newTapOverlay(e.showEditor)

	e.entry = newHighlightEntry(e.showPreview)
	e.entry.SetPlaceHolder("Enter your AdvPL/TLPP code here...")
	e.entry.OnChanged = func(string) {
		e.modified = true
	}

	// e.stack.Objects is swapped directly (rather than using Show()/Hide()
	// on permanent members) because a container.Stack child that is Hidden
	// before its first paint never gets a repaint queued for it later by
	// Show(). Container.Refresh() alone is also not enough after a swap: it
	// re-runs layout and calls each child's own Refresh(), but never calls
	// the canvas's SetDirty() (fyne.io/fyne/v2@v2.4.4 container.go only
	// does that from Container.Move()/Hide(), not Refresh()) — so nothing
	// actually gets repainted on screen. swapStack() below forces it with a
	// harmless no-op Move() to the container's own current position.
	e.stack = container.NewStack(e.entry)
	return e
}

// swapStack replaces the editor's visible content and forces an actual
// repaint — see the comment in NewCodeEditor for why Refresh() alone is
// insufficient here.
func (e *CodeEditor) swapStack(objects ...fyne.CanvasObject) {
	e.stack.Objects = objects
	e.stack.Refresh()
	e.stack.Move(e.stack.Position())
}

func (e *CodeEditor) showPreview() {
	e.preview.Segments = highlightSegments(e.entry.Text)
	e.preview.Refresh()
	e.swapStack(e.preview, e.overlay)
}

func (e *CodeEditor) showEditor() {
	e.swapStack(e.entry)
	if cv := fyne.CurrentApp().Driver().CanvasForObject(e.entry); cv != nil {
		cv.Focus(e.entry)
	}
}

func (e *CodeEditor) GetContent() string {
	return e.entry.Text
}

func (e *CodeEditor) SetContent(text string) {
	e.entry.SetText(text)
	e.modified = false
	if text == "" {
		// New/empty file: stay in edit mode so the entry's placeholder text
		// is visible and typing can start immediately, no extra click.
		e.showEditor()
		return
	}
	e.showPreview()
}

func (e *CodeEditor) GetWidget() fyne.CanvasObject {
	return e.stack
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
