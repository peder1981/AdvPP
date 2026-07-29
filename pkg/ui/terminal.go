package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// TerminalUIProvider implements vm.UIProvider as a real full-screen TUI —
// boxed forms, arrow-key navigation, tables — built on huh/lipgloss, as
// opposed to the Fyne (desktop window) or web/SSE providers. It's what
// turns a headless `advplc build` executable into an actual usable app
// instead of one that silently returns FWGetText's default (see
// natives.go: without any UIProvider, FWGETTEXT/FWMENUSELECT never touch
// stdin at all — they just hand back the zero value).
type TerminalUIProvider struct{}

// NewTerminalUIProvider performs a core operation.
func NewTerminalUIProvider() *TerminalUIProvider {
	// lipgloss otherwise auto-detects light/dark by querying the terminal
	// (OSC 11) and blocking for the reply. Real terminal emulators answer
	// automatically, but anything that doesn't — some multiplexers, some
	// minimal/embedded terminals, and notably any test harness driving
	// this over a raw PTY without emulating that reply — hangs forever
	// with nothing rendered, since the query blocks before the first
	// frame draws. A wrong color guess is a cosmetic problem; a
	// console app that never displays anything is not one this project
	// can afford to reintroduce after the last round of "looked done,
	// wasn't tested for real."
	lipgloss.SetHasDarkBackground(true)
	return &TerminalUIProvider{}
}

// IsTerminal reports whether f is a real interactive terminal, as opposed
// to a pipe, redirected file, or /dev/null — used to decide whether it's
// safe to attach a TerminalUIProvider (blocks on reads, needs a real tty
// for huh's raw-mode input) versus leaving the VM with no UIProvider
// (never blocks, always returns defaults).
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

var (
	tableTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).MarginBottom(1)
	tableBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	emptyStyle       = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("240"))
)

// runField wraps a single huh.Field in a one-field Form/Group so it gets
// the themed boxed rendering (border, help line with the actual
// keybindings) that a bare field.Run() doesn't — this is what makes Menu/
// InputText/MsgYesNo/MsgInfo look like the rest of a real TUI instead of
// a plain terminal prompt.
func runField(f huh.Field) error {
	return huh.NewForm(huh.NewGroup(f)).WithTheme(huh.ThemeCharm()).WithKeyMap(ptKeyMap).Run()
}

// ptKeyMap overrides huh's default y/n confirm shortcuts with s/n — the
// labels on MsgYesNo are "Sim"/"Não" (see below), and huh doesn't derive
// keybindings from custom Affirmative/Negative labels; leaving the
// default y/n bound would make the visible options and the actual
// shortcut disagree for a Portuguese-reading user pressing "s" for "Sim"
// and getting nothing.
var ptKeyMap = func() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Confirm.Accept = key.NewBinding(key.WithKeys("s", "S"), key.WithHelp("s", "Sim"))
	km.Confirm.Reject = key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n", "Não"))
	return km
}()

func (p *TerminalUIProvider) MsgInfo(msg, title string) {
	p.note(title, msg, "Information")
}

func (p *TerminalUIProvider) MsgStop(msg, title string) {
	p.note(title, msg, "Stop")
}

func (p *TerminalUIProvider) MsgAlert(msg, title string) {
	p.note(title, msg, "Alert")
}

// note blocks on an acknowledgment ("OK") — matches the Fyne/web
// providers, where MsgInfo/MsgStop/MsgAlert are real modal dialogs the
// user has to dismiss, not a fire-and-forget console print.
func (p *TerminalUIProvider) note(title, msg, fallback string) {
	if title == "" {
		title = fallback
	}
	n := huh.NewNote().Title(title).Description(msg).Next(true).NextLabel("OK")
	_ = runField(n)
}

func (p *TerminalUIProvider) MsgYesNo(msg, title string) bool {
	if title == "" {
		title = "Confirm"
	}
	var result bool
	c := huh.NewConfirm().Title(title).Description(msg).Affirmative("Sim").Negative("Não").Value(&result)
	if err := runField(c); err != nil {
		return false // Ctrl+C/Esc: never treat an aborted prompt as "yes" on a destructive action
	}
	return result
}

// Menu shows a boxed, arrow-key-navigable list and blocks for a choice.
// Returns 0 on Ctrl+C/Esc — same "closed without choosing" contract as
// the Fyne and web providers.
func (p *TerminalUIProvider) Menu(items []string, title string) int {
	if title == "" {
		title = "Menu"
	}
	opts := make([]huh.Option[int], len(items))
	for i, label := range items {
		opts[i] = huh.NewOption(label, i+1)
	}
	result := 0
	s := huh.NewSelect[int]().Title(title).Options(opts...).Value(&result)
	if err := runField(s); err != nil {
		return 0
	}
	return result
}

// InputText shows a boxed text field pre-filled with def (editable in
// place — clear it and submit empty to actually blank the value; leave it
// untouched to keep the default). Password fields mask input.
func (p *TerminalUIProvider) InputText(prompt, def string, bIsPassword bool) string {
	val := def
	i := huh.NewInput().Title(prompt).Value(&val)
	if bIsPassword {
		i = i.Password(true)
	}
	if err := runField(i); err != nil {
		return def
	}
	if val == "" {
		return def
	}
	return val
}

// --- FWMBrowse (console rendering) ---
//
// browse.go's runBrowse() drives the CRUD loop and only needs a renderer
// that satisfies vm.BrowseUI: given a JSON spec (title/columns/items),
// block until the user picks an action, and return it as JSON. All the SQL
// (columns from SX3, save, delete) already lives in browse.go, transport-
// agnostic — the web/SSE and Fyne providers each render the same spec
// their own way; this one draws a bordered table and a boxed action menu.
// browseColumn/browseSpec/browseAction (the wire types) already exist in
// browse.go (used by FyneUIProvider.Browse) — reused here as-is.

const maxColWidth = 24

// termWidth returns the terminal's column count, falling back to a
// reasonable default when it can't be read (not a TTY, or the ioctl
// fails) — used to cap the record table to what actually fits instead of
// printing every SX3 column edge to edge, which for a table like EG0
// (21 columns) produced a single unreadable box hundreds of columns wide.
func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 100
}

func fieldString(col browseColumn, v interface{}) string {
	if v == nil {
		return ""
	}
	if n, ok := v.(float64); ok {
		if col.Type == "N" {
			return strconv.FormatFloat(n, 'f', col.Decimal, 64)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}

// visibleColumns picks a prefix of cols that fits within width columns of
// screen — each one's natural width is max(label, widest value across
// items), capped at maxColWidth. Columns are added greedily in SX3 order
// (the order the table was designed in, so the first ones are usually the
// identifying/important fields — EG0_COD, EG0_NOME, ... — not whatever
// happens to be short) until the next one wouldn't fit. Returns the
// columns to show and their computed widths; len(result) < len(cols)
// means some were left out (the caller notes that — Editar always shows
// every field regardless, this only affects the list view).
func visibleColumns(cols []browseColumn, items []map[string]interface{}, budget int) ([]browseColumn, []int) {
	natural := make([]int, len(cols))
	for i, c := range cols {
		w := len(c.Label)
		if w > maxColWidth {
			w = maxColWidth
		}
		natural[i] = w
	}
	for _, item := range items {
		for i, c := range cols {
			w := len(fieldString(c, item[c.Property]))
			if w > maxColWidth {
				w = maxColWidth
			}
			if w > natural[i] {
				natural[i] = w
			}
		}
	}

	shown := []browseColumn{}
	widths := []int{}
	used := 0
	for i, c := range cols {
		add := natural[i]
		if used > 0 {
			add += 2 // separador "  " entre colunas
		}
		if used+add > budget && len(shown) > 0 {
			break
		}
		shown = append(shown, c)
		widths = append(widths, natural[i])
		used += add
	}
	return shown, widths
}

// renderTable draws the record list as a bordered table, or an empty-state
// note when there are no rows. Only as many columns as fit the terminal
// width are shown — see visibleColumns — with a note when some were left
// out, since printing all of them unconditionally (some SX3 tables, like
// EG0, have 20+ fields) produced a single box hundreds of columns wide,
// wrapped by every terminal into unreadable garbage.
func renderTable(title string, bs *browseSpec) {
	fmt.Println(tableTitleStyle.Render(title))
	if len(bs.Items) == 0 {
		fmt.Println(tableBorderStyle.Render(emptyStyle.Render("(nenhum registro)")))
		return
	}

	idxWidth := len(strconv.Itoa(len(bs.Items)))
	// orçamento: largura do terminal menos moldura (bordas+padding do
	// lipgloss, ~4) e a coluna de índice ("#  ")
	budget := termWidth() - 4 - idxWidth - 2
	if budget < 20 {
		budget = 20
	}
	cols, widths := visibleColumns(bs.Columns, bs.Items, budget)

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", idxWidth) + "  ")
	headerCells := make([]string, len(cols))
	for i, c := range cols {
		headerCells[i] = padRight(truncate(c.Label, widths[i]), widths[i])
	}
	b.WriteString(tableHeaderStyle.Render(strings.Join(headerCells, "  ")))
	for i, item := range bs.Items {
		b.WriteString("\n")
		b.WriteString(padLeft(strconv.Itoa(i+1), idxWidth) + "  ")
		cells := make([]string, len(cols))
		for j, c := range cols {
			cells[j] = padRight(truncate(fieldString(c, item[c.Property]), widths[j]), widths[j])
		}
		b.WriteString(strings.Join(cells, "  "))
	}
	fmt.Println(tableBorderStyle.Render(b.String()))
	if len(cols) < len(bs.Columns) {
		fmt.Println(emptyStyle.Render(fmt.Sprintf("(+%d campo(s) não exibido(s) — Editar mostra todos)", len(bs.Columns)-len(cols))))
	}
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func marshalBrowseAction(a browseAction) []byte {
	b, _ := json.Marshal(a)
	return b
}

// editForm shows every column as a field in one boxed, tab-navigable form
// — a real single-screen record editor, not a sequence of separate
// prompts. current is nil for Novo (all fields start blank).
func (p *TerminalUIProvider) editForm(cols []browseColumn, current map[string]interface{}) map[string]string {
	values := make([]string, len(cols))
	fields := make([]huh.Field, len(cols))
	for i, c := range cols {
		if current != nil {
			values[i] = fieldString(c, current[c.Property])
		}
		fields[i] = huh.NewInput().Title(c.Label).Value(&values[i])
	}
	if err := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeCharm()).WithKeyMap(ptKeyMap).Run(); err != nil {
		return nil // Ctrl+C/Esc: caller treats this as "nothing to save"
	}
	data := make(map[string]string, len(cols))
	for i, c := range cols {
		data[c.Property] = values[i]
	}
	return data
}

// pickRow lists items using their first few columns as a label and asks
// which one, 1-based. Returns -1 if there's nothing to pick from or the
// user backs out — the caller loops back to the main browse menu.
func (p *TerminalUIProvider) pickRow(items []map[string]interface{}, cols []browseColumn) int {
	if len(items) == 0 {
		return -1
	}
	labelCols := cols
	if len(labelCols) > 3 {
		labelCols = labelCols[:3]
	}
	labels := make([]string, len(items))
	for i, item := range items {
		parts := make([]string, len(labelCols))
		for j, c := range labelCols {
			parts[j] = truncate(fieldString(c, item[c.Property]), maxColWidth)
		}
		labels[i] = strings.Join(parts, " | ")
	}
	choice := p.Menu(labels, "Selecione o registro")
	if choice == 0 {
		return -1
	}
	return choice - 1
}

// Browse renders one turn of an FWMBrowse cycle: draws the record table,
// asks Novo/Editar/Excluir/Voltar, and — looping internally as needed
// (e.g. "Editar" with no rows, or the user backing out of a sub-prompt) —
// returns the first genuine action. runBrowse only understands three
// outcomes ("save", "delete", anything else = close), so unlike Menu()
// there's no "redisplay, no-op" action to hand back; any cancellation is
// absorbed here by looping back to the list instead.
func (p *TerminalUIProvider) Browse(spec []byte) []byte {
	var bs browseSpec
	if err := json.Unmarshal(spec, &bs); err != nil {
		return marshalBrowseAction(browseAction{Action: "close"})
	}
	title := bs.Title
	if title == "" {
		title = bs.Alias
	}

	for {
		renderTable(title, &bs)

		switch p.Menu([]string{"Novo", "Editar", "Excluir", "Voltar"}, "") {
		case 1: // Novo
			data := p.editForm(bs.Columns, nil)
			if data == nil {
				continue
			}
			return marshalBrowseAction(browseAction{Action: "save", Data: data})

		case 2: // Editar
			idx := p.pickRow(bs.Items, bs.Columns)
			if idx < 0 {
				continue
			}
			item := bs.Items[idx]
			data := p.editForm(bs.Columns, item)
			if data == nil {
				continue
			}
			recno, _ := item["recno"].(float64)
			return marshalBrowseAction(browseAction{Action: "save", Recno: int64(recno), Data: data})

		case 3: // Excluir
			idx := p.pickRow(bs.Items, bs.Columns)
			if idx < 0 {
				continue
			}
			item := bs.Items[idx]
			recno, _ := item["recno"].(float64)
			if !p.MsgYesNo(fmt.Sprintf("Confirma excluir o registro #%d?", idx+1), "Excluir") {
				continue
			}
			return marshalBrowseAction(browseAction{Action: "delete", Recno: int64(recno)})

		default: // Voltar (ou Ctrl+C/Esc)
			return marshalBrowseAction(browseAction{Action: "close"})
		}
	}
}
