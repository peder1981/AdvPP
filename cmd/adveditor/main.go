package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/ui"
)

// version é injetada no build via -ldflags "-X main.version=v1.2.3" (make release).
var version = "dev"

// AdvEditorWindow representa a janela principal do AdvEditor
type AdvEditorWindow struct {
	window       fyne.Window
	tableManager *shared.TableManager
	treeView     *shared.TreeView
	dataGrid     *widget.Table
	statusBar    *widget.Label
	currentTable *shared.TableInfo
	records      []shared.Record // página atual de dados exibida no grid
	selectedRow  int             // índice em records da linha selecionada no grid, -1 = nenhuma
}

// NewAdvEditorWindow cria uma nova janela do AdvEditor
func NewAdvEditorWindow(a fyne.App) *AdvEditorWindow {
	w := a.NewWindow(fmt.Sprintf("AdvEditor %s - Editor de Banco de Dados", version))
	w.Resize(fyne.NewSize(1200, 800))

	ae := &AdvEditorWindow{
		window:       w,
		tableManager: shared.NewTableManager(),
		selectedRow:  -1,
	}

	ae.setupUI()
	ae.setupMenu()

	// Abre automaticamente o banco de dados padrão
	ae.openDefaultDatabase()

	return ae
}

// setupUI configura a interface do usuário
func (ae *AdvEditorWindow) setupUI() {
	// Cria tree view de tabelas
	root := &shared.TreeNode{
		ID:       "root",
		Text:     "Tabelas",
		Children: []*shared.TreeNode{},
	}

	ae.treeView = shared.NewTreeView(root)
	ae.treeView.SetOnSelect(func(node *shared.TreeNode) {
		ae.onTableSelected(node)
	})

	// Cria grid de dados: linha 0 = cabeçalho, demais = registros carregados
	ae.dataGrid = widget.NewTable(
		func() (int, int) {
			if ae.currentTable == nil {
				return 0, 0
			}
			return len(ae.records) + 1, len(ae.currentTable.Structure)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			return label
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			if ae.currentTable == nil || id.Col >= len(ae.currentTable.Structure) {
				label.SetText("")
				return
			}
			field := ae.currentTable.Structure[id.Col]

			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText(field.Name)
				return
			}
			label.TextStyle = fyne.TextStyle{}
			if id.Row-1 >= len(ae.records) {
				label.SetText("")
				return
			}
			label.SetText(formatCell(ae.records[id.Row-1].Fields[field.Name]))
		},
	)
	ae.dataGrid.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 || id.Row-1 >= len(ae.records) {
			ae.selectedRow = -1
			return
		}
		ae.selectedRow = id.Row - 1
	}
	ae.dataGrid.OnUnselected = func(id widget.TableCellID) {
		ae.selectedRow = -1
	}

	// Status bar
	ae.statusBar = widget.NewLabel("Pronto")

	// Layout principal (Border: a árvore ocupa toda a altura do painel)
	split := container.NewHSplit(
		container.NewBorder(
			widget.NewLabel("Tabelas"),
			nil, nil, nil,
			ae.treeView,
		),
		container.NewBorder(
			nil,
			ae.statusBar,
			nil,
			nil,
			ae.dataGrid,
		),
	)
	split.SetOffset(0.2)

	ae.window.SetContent(split)
}

// setupMenu configura o menu
func (ae *AdvEditorWindow) setupMenu() {
	fileMenu := fyne.NewMenu("Arquivo",
		fyne.NewMenuItem("Abrir (Ctrl+B)", ae.onOpenTable),
		fyne.NewMenuItem("Trocar Banco de Dados", ae.onChangeDatabase),
		fyne.NewMenuItem("Fechar", ae.onCloseTable),
		fyne.NewMenuItem("Sair", func() {
			ae.window.Close()
		}),
	)

	tableMenu := fyne.NewMenu("Tabela",
		fyne.NewMenuItem("Nova Tabela", ae.onNewTable),
		fyne.NewMenuItem("Excluir Tabela", ae.onDropTable),
		fyne.NewMenuItem("Estrutura", ae.onViewStructure),
		fyne.NewMenuItem("Adicionar Campo", ae.onAddColumn),
		fyne.NewMenuItem("Remover Campo", ae.onDropColumn),
	)

	editMenu := fyne.NewMenu("Editar",
		fyne.NewMenuItem("Incluir", ae.onAddRecord),
		fyne.NewMenuItem("Alterar", ae.onEditRecord),
		fyne.NewMenuItem("Excluir", ae.onDeleteRecord),
	)

	indexMenu := fyne.NewMenu("Índice",
		fyne.NewMenuItem("Criar", ae.onCreateIndex),
		fyne.NewMenuItem("Excluir", ae.onDropIndex),
	)

	helpMenu := fyne.NewMenu("Ajuda",
		fyne.NewMenuItem("Sobre", ae.onAbout),
	)

	mainMenu := fyne.NewMainMenu(
		fileMenu,
		tableMenu,
		editMenu,
		indexMenu,
		helpMenu,
	)

	ae.window.SetMainMenu(mainMenu)
}

// onOpenTable abre uma tabela
func (ae *AdvEditorWindow) onOpenTable() {
	// Primeiro, seleciona o driver
	ae.selectDriver()
}

// selectDriver seleciona o driver de banco de dados
func (ae *AdvEditorWindow) selectDriver() {
	driverOptions := []string{"SQLite", "DBF", "TopConnect", "Ctree", "BTrieve"}

	driverSelect := widget.NewSelect(driverOptions, func(selected string) {
		// Driver selecionado
	})
	driverSelect.SetSelectedIndex(0)

	sharedCheck := widget.NewCheck("Compartilhado", func(checked bool) {
		// Implementar lógica de compartilhado
	})
	sharedCheck.Checked = true

	readonlyCheck := widget.NewCheck("Somente leitura", func(checked bool) {
		// Implementar lógica de somente leitura
	})

	dialog.ShowForm("Selecionar Driver", "OK", "Cancelar", []*widget.FormItem{
		widget.NewFormItem("Driver", driverSelect),
		widget.NewFormItem("Compartilhado", sharedCheck),
		widget.NewFormItem("Somente Leitura", readonlyCheck),
	}, func(confirmed bool) {
		if !confirmed {
			return
		}
		selectedDriver := driverSelect.Selected
		ae.selectFile(selectedDriver, sharedCheck.Checked, readonlyCheck.Checked)
	}, ae.window)
}

// selectFile seleciona o arquivo de acordo com o driver
func (ae *AdvEditorWindow) selectFile(driver string, sharedMode, readonly bool) {
	// Se for SQLite, usa o banco padrão diretamente — sempre, mesmo que o
	// arquivo ainda não exista (auto-criado no open, ver openDefaultDatabase).
	if driver == "SQLite" {
		ae.openDatabasePath(shared.ResolveDatabasePath(""), "SQLITE", sharedMode, readonly, driver)
		return
	}

	// Se não for SQLite ou não encontrou banco padrão, mostra diálogo de seleção
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		// Obtém nome do arquivo
		uri := reader.URI()
		filePath := uri.Path()

		ae.openDatabasePath(filePath, ae.getDriverCode(driver), sharedMode, readonly, driver)
	}, ae.window)
	if loc := ui.CurrentDirLocation(); loc != nil {
		fd.SetLocation(loc)
	}
	fd.Show()
}

// openDatabasePath abre o banco de dados no caminho especificado
func (ae *AdvEditorWindow) openDatabasePath(filePath, driverCode string, shared, readonly bool, driverName string) {
	// Fecha tabela atual se existir
	if ae.currentTable != nil {
		ae.tableManager.CloseTable(ae.currentTable.Alias)
	}

	// Abre banco de dados
	tableInfo, err := ae.tableManager.OpenTable(filePath, driverCode, readonly, shared)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Erro ao abrir banco de dados: %w", err), ae.window)
		return
	}

	ae.currentTable = tableInfo

	// Carrega tabelas do banco
	ae.loadTablesFromDatabase()

	// Se for SQLite, mostra diálogo para selecionar tabela
	if driverCode == "SQLITE" {
		ae.selectTable()
	}

	ae.statusBar.SetText("Banco de dados aberto: " + filePath + " (" + driverName + ")")
}

// getDriverCode converte nome do driver para código interno
func (ae *AdvEditorWindow) getDriverCode(driver string) string {
	switch driver {
	case "SQLite":
		return "SQLITE"
	case "DBF":
		return "DBF"
	case "TopConnect":
		return "TOPCONNECT"
	case "Ctree":
		return "CTREECDX"
	case "BTrieve":
		return "BTVCDX"
	default:
		return "DBF"
	}
}

// selectTable seleciona uma tabela do banco de dados
func (ae *AdvEditorWindow) selectTable() {
	if ae.currentTable == nil {
		return
	}

	tables := ae.listTablesFromDB()
	if len(tables) == 0 {
		dialog.ShowInformation("Aviso", "Nenhuma tabela encontrada no banco de dados", ae.window)
		return
	}

	tableSelect := widget.NewSelect(tables, func(selected string) {
		// Tabela selecionada
	})
	tableSelect.SetSelectedIndex(0)

	dialog.ShowForm("Selecionar Tabela", "OK", "Cancelar", []*widget.FormItem{
		widget.NewFormItem("Tabela", tableSelect),
	}, func(confirmed bool) {
		if !confirmed {
			return
		}
		selectedTable := tableSelect.Selected
		ae.loadTableData(selectedTable)
	}, ae.window)
}

// onCloseTable fecha a tabela atual
func (ae *AdvEditorWindow) onCloseTable() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}

	err := ae.tableManager.CloseTable(ae.currentTable.Alias)
	if err != nil {
		dialog.ShowError(err, ae.window)
		return
	}

	ae.currentTable = nil
	ae.updateTreeView()
	ae.updateDataGrid()
	ae.statusBar.SetText("Tabela fechada")
}

// fieldTypeLabels/fieldTypeValues são as opções mostradas no Select de tipo
// de campo (Nova Tabela / Adicionar Campo) e o shared.FieldType que cada
// uma representa, na mesma ordem.
var fieldTypeLabels = []string{"Caractere (C)", "Numérico (N)", "Data (D)", "Lógico (L)", "Memo (M)"}
var fieldTypeValues = []shared.FieldType{
	shared.FieldTypeChar, shared.FieldTypeNum, shared.FieldTypeDate, shared.FieldTypeLog, shared.FieldTypeMemo,
}

// fieldTypeLabel/fieldTypeFromLabel convertem entre o FieldType interno e o
// texto mostrado no Select.
func fieldTypeLabel(t shared.FieldType) string {
	for i, v := range fieldTypeValues {
		if v == t {
			return fieldTypeLabels[i]
		}
	}
	return fieldTypeLabels[0]
}

func fieldTypeFromLabel(label string) shared.FieldType {
	for i, l := range fieldTypeLabels {
		if l == label {
			return fieldTypeValues[i]
		}
	}
	return shared.FieldTypeChar
}

// onViewStructure exibe a estrutura da tabela
func (ae *AdvEditorWindow) onViewStructure() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}

	// Monta o markdown inteiro numa string à parte antes de parsear uma
	// única vez — RichText.String() devolve o texto JÁ RENDERIZADO (sem as
	// quebras de linha do markdown fonte), então reparsear content.String()
	// a cada campo acumulava tudo numa linha só.
	md := "## Estrutura da Tabela: " + ae.currentTable.Alias + "\n\n"
	for _, field := range ae.currentTable.Structure {
		md += fmt.Sprintf("- **%s**: %s (%d,%d)\n", field.Name, fieldTypeLabel(field.Type), field.Size, field.Decimal)
	}
	content := widget.NewRichTextFromMarkdown(md)

	scroll := container.NewScroll(content)
	// container.NewScroll não tem tamanho intrínseco — sem um Resize
	// explícito, o diálogo desenha praticamente do tamanho de um pixel
	// (mesma causa-raiz do bug de árvore vazia: widgets Fyne sem
	// dica de tamanho colapsam para o mínimo).
	scroll.Resize(fyne.NewSize(500, 400))
	d := dialog.NewCustom("Estrutura", "Fechar", scroll, ae.window)
	d.Resize(fyne.NewSize(520, 440))
	d.Show()
}

// fieldRow é uma linha editável do formulário "Nova Tabela" (nome + tipo +
// tamanho + decimal + botão de remover), mantida numa lista para que o
// usuário adicione/remova campos antes de confirmar a criação.
type fieldRow struct {
	nameEntry *widget.Entry
	typeSel   *widget.Select
	sizeEntry *widget.Entry
	decEntry  *widget.Entry
	box       *fyne.Container // linha inteira, para poder remover do pai
}

func newFieldRow(rowsBox *fyne.Container, rows *[]*fieldRow) *fieldRow {
	fr := &fieldRow{
		nameEntry: widget.NewEntry(),
		typeSel:   widget.NewSelect(fieldTypeLabels, func(string) {}),
		sizeEntry: widget.NewEntry(),
		decEntry:  widget.NewEntry(),
	}
	fr.nameEntry.SetPlaceHolder("nome do campo")
	fr.typeSel.SetSelectedIndex(0)
	fr.sizeEntry.SetPlaceHolder("tamanho")
	fr.decEntry.SetPlaceHolder("decimal")
	removeBtn := widget.NewButton("Remover", nil)
	fr.box = container.NewGridWithColumns(5, fr.nameEntry, fr.typeSel, fr.sizeEntry, fr.decEntry, removeBtn)
	removeBtn.OnTapped = func() {
		for i, r := range *rows {
			if r == fr {
				*rows = append((*rows)[:i], (*rows)[i+1:]...)
				break
			}
		}
		rowsBox.Remove(fr.box)
		rowsBox.Refresh()
	}
	return fr
}

func (fr *fieldRow) toField() (shared.Field, error) {
	name := strings.TrimSpace(fr.nameEntry.Text)
	if name == "" {
		return shared.Field{}, fmt.Errorf("nome de campo em branco")
	}
	size, dec := 0, 0
	fmt.Sscanf(fr.sizeEntry.Text, "%d", &size)
	fmt.Sscanf(fr.decEntry.Text, "%d", &dec)
	return shared.Field{
		Name:    name,
		Type:    fieldTypeFromLabel(fr.typeSel.Selected),
		Size:    size,
		Decimal: dec,
	}, nil
}

// showFieldListDialog exibe um diálogo com uma lista de linhas de campo
// (nome/tipo/tamanho/decimal) que o usuário pode crescer com "Adicionar
// Campo", e chama onConfirm com os Field resultantes se confirmado.
// Reaproveitado por onNewTable (lista começa vazia) — Adicionar Campo
// (onAddColumn) usa um formulário mais simples de 1 campo só, sem precisar
// desta lista dinâmica.
func (ae *AdvEditorWindow) showFieldListDialog(title, confirmLabel string, onConfirm func([]shared.Field) error) {
	var rows []*fieldRow
	rowsBox := container.NewVBox()
	addRow := func() {
		fr := newFieldRow(rowsBox, &rows)
		rows = append(rows, fr)
		rowsBox.Add(fr.box)
	}
	addRow() // começa com uma linha em branco pronta pra preencher

	addBtn := widget.NewButton("+ Adicionar Campo", addRow)
	content := container.NewBorder(
		container.NewVBox(widget.NewLabel("Nome / Tipo / Tamanho / Decimal"), addBtn),
		nil, nil, nil,
		container.NewVScroll(rowsBox),
	)

	d := dialog.NewCustomConfirm(title, confirmLabel, "Cancelar", content, func(confirmed bool) {
		if !confirmed {
			return
		}
		fields := make([]shared.Field, 0, len(rows))
		for _, r := range rows {
			f, err := r.toField()
			if err != nil {
				continue // linha em branco deixada pra trás — ignora em vez de falhar tudo
			}
			fields = append(fields, f)
		}
		if err := onConfirm(fields); err != nil {
			dialog.ShowError(err, ae.window)
		}
	}, ae.window)
	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}

// onNewTable cria uma nova tabela no banco atual (nome + lista de campos).
func (ae *AdvEditorWindow) onNewTable() {
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Abra um banco SQLite primeiro (Arquivo > Abrir)", ae.window)
		return
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("nome da tabela")
	dialog.ShowForm("Nova Tabela — nome", "Próximo", "Cancelar", []*widget.FormItem{
		widget.NewFormItem("Nome", nameEntry),
	}, func(confirmed bool) {
		if !confirmed {
			return
		}
		tableName := strings.TrimSpace(nameEntry.Text)
		if tableName == "" {
			dialog.ShowInformation("Aviso", "Nome da tabela não pode ser vazio", ae.window)
			return
		}
		ae.showFieldListDialog("Nova Tabela — campos", "Criar", func(fields []shared.Field) error {
			if err := driver.CreateTable(tableName, fields); err != nil {
				return fmt.Errorf("erro ao criar tabela: %w", err)
			}
			ae.statusBar.SetText("Tabela criada: " + tableName)
			ae.loadTablesFromDatabase()
			return nil
		})
	}, ae.window)
}

// onDropTable exclui a tabela atualmente selecionada (com confirmação).
func (ae *AdvEditorWindow) onDropTable() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Excluir tabela só é suportado para bancos SQLite por enquanto", ae.window)
		return
	}
	tableName := ae.currentTable.Alias
	dialog.ShowConfirm("Excluir Tabela", "Confirma excluir a tabela \""+tableName+"\"? Esta ação não pode ser desfeita.", func(confirmed bool) {
		if !confirmed {
			return
		}
		if err := driver.DropTable(tableName); err != nil {
			dialog.ShowError(fmt.Errorf("erro ao excluir tabela: %w", err), ae.window)
			return
		}
		ae.currentTable = nil
		ae.records = nil
		ae.statusBar.SetText("Tabela excluída: " + tableName)
		ae.loadTablesFromDatabase()
		ae.updateDataGrid()
	}, ae.window)
}

// onAddColumn adiciona um campo à tabela atual (ALTER TABLE ADD COLUMN).
func (ae *AdvEditorWindow) onAddColumn() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Adicionar campo só é suportado para tabelas SQLite por enquanto", ae.window)
		return
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("nome do campo")
	typeSel := widget.NewSelect(fieldTypeLabels, func(string) {})
	typeSel.SetSelectedIndex(0)
	sizeEntry := widget.NewEntry()
	decEntry := widget.NewEntry()

	dialog.ShowForm("Adicionar Campo", "Adicionar", "Cancelar", []*widget.FormItem{
		widget.NewFormItem("Nome", nameEntry),
		widget.NewFormItem("Tipo", typeSel),
		widget.NewFormItem("Tamanho", sizeEntry),
		widget.NewFormItem("Decimal", decEntry),
	}, func(confirmed bool) {
		if !confirmed {
			return
		}
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			dialog.ShowInformation("Aviso", "Nome de campo não pode ser vazio", ae.window)
			return
		}
		size, dec := 0, 0
		fmt.Sscanf(sizeEntry.Text, "%d", &size)
		fmt.Sscanf(decEntry.Text, "%d", &dec)
		field := shared.Field{Name: name, Type: fieldTypeFromLabel(typeSel.Selected), Size: size, Decimal: dec}
		if err := driver.AddColumn(field); err != nil {
			dialog.ShowError(fmt.Errorf("erro ao adicionar campo: %w", err), ae.window)
			return
		}
		ae.currentTable.Structure, _ = driver.GetStructure()
		ae.statusBar.SetText("Campo adicionado: " + name)
		ae.updateDataGrid()
	}, ae.window)
}

// onDropColumn remove um campo da tabela atual, escolhido de uma lista dos
// campos existentes.
func (ae *AdvEditorWindow) onDropColumn() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Remover campo só é suportado para tabelas SQLite por enquanto", ae.window)
		return
	}
	if len(ae.currentTable.Structure) == 0 {
		dialog.ShowInformation("Aviso", "Esta tabela não tem campos para remover", ae.window)
		return
	}
	names := make([]string, len(ae.currentTable.Structure))
	for i, f := range ae.currentTable.Structure {
		names[i] = f.Name
	}
	sel := widget.NewSelect(names, func(string) {})
	sel.SetSelectedIndex(0)
	dialog.ShowForm("Remover Campo", "Remover", "Cancelar", []*widget.FormItem{
		widget.NewFormItem("Campo", sel),
	}, func(confirmed bool) {
		if !confirmed || sel.Selected == "" {
			return
		}
		if err := driver.DropColumn(sel.Selected); err != nil {
			dialog.ShowError(fmt.Errorf("erro ao remover campo: %w", err), ae.window)
			return
		}
		ae.currentTable.Structure, _ = driver.GetStructure()
		ae.statusBar.SetText("Campo removido: " + sel.Selected)
		ae.updateDataGrid()
	}, ae.window)
}

// currentDriver devolve o SQLiteDriver da conexão aberta, ou nil se a
// tabela atual não é SQLite (outros drivers ainda não têm DDL/CRUD real —
// ver comentário na struct DatabaseDriver).
func (ae *AdvEditorWindow) currentDriver() *shared.SQLiteDriver {
	if ae.currentTable == nil {
		return nil
	}
	d, _ := ae.currentTable.DriverObj.(*shared.SQLiteDriver)
	return d
}

// fieldEntry é o par (campo, widget de edição) de uma linha do formulário
// de registro — o tipo do widget depende de field.Type (Entry para C/N/D,
// Check para L).
type fieldEntry struct {
	field shared.Field
	entry *widget.Entry
	check *widget.Check
}

// value lê o valor digitado no widget e converte para o tipo Go que
// AddRecord/UpdateRecord espera (float64 para N, bool→int para L, string
// para o resto).
func (fe fieldEntry) value() interface{} {
	if fe.check != nil {
		if fe.check.Checked {
			return 1
		}
		return 0
	}
	text := fe.entry.Text
	if fe.field.Type == shared.FieldTypeNum || fe.field.Type == shared.FieldTypeDouble {
		var n float64
		fmt.Sscanf(text, "%g", &n)
		return n
	}
	return text
}

// buildRecordFormItems monta um FormItem por campo da estrutura atual,
// pré-preenchido com os valores de `initial` (nil = registro novo, campos
// em branco). Devolve os itens do form E os fieldEntry para ler os valores
// depois de confirmado.
func (ae *AdvEditorWindow) buildRecordFormItems(initial map[string]interface{}) ([]*widget.FormItem, []fieldEntry) {
	items := make([]*widget.FormItem, 0, len(ae.currentTable.Structure))
	entries := make([]fieldEntry, 0, len(ae.currentTable.Structure))
	for _, field := range ae.currentTable.Structure {
		fe := fieldEntry{field: field}
		if field.Type == shared.FieldTypeLog {
			check := widget.NewCheck("", func(bool) {})
			if initial != nil {
				check.Checked = formatCell(initial[field.Name]) == "1"
			}
			fe.check = check
			items = append(items, widget.NewFormItem(field.Name, check))
		} else {
			entry := widget.NewEntry()
			if field.Type == shared.FieldTypeDate {
				entry.SetPlaceHolder("AAAA-MM-DD")
			}
			if initial != nil {
				entry.SetText(formatCell(initial[field.Name]))
			}
			fe.entry = entry
			items = append(items, widget.NewFormItem(field.Name, entry))
		}
		entries = append(entries, fe)
	}
	return items, entries
}

// onAddRecord adiciona um registro via formulário (um campo por coluna da
// tabela atual).
func (ae *AdvEditorWindow) onAddRecord() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}
	if len(ae.currentTable.Structure) == 0 {
		dialog.ShowInformation("Aviso", "A tabela não tem campos — use Estrutura > Adicionar Campo primeiro", ae.window)
		return
	}
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Incluir só é suportado para tabelas SQLite por enquanto", ae.window)
		return
	}

	items, entries := ae.buildRecordFormItems(nil)
	dialog.ShowForm("Incluir Registro", "Incluir", "Cancelar", items, func(confirmed bool) {
		if !confirmed {
			return
		}
		fields := make(map[string]interface{}, len(entries))
		for _, fe := range entries {
			fields[fe.field.Name] = fe.value()
		}
		if _, err := driver.AddRecord(shared.Record{Fields: fields}); err != nil {
			dialog.ShowError(fmt.Errorf("erro ao incluir registro: %w", err), ae.window)
			return
		}
		ae.loadTableData(ae.currentTable.Alias)
	}, ae.window)
}

// onEditRecord edita o registro selecionado no grid.
func (ae *AdvEditorWindow) onEditRecord() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}
	if ae.selectedRow < 0 || ae.selectedRow >= len(ae.records) {
		dialog.ShowInformation("Aviso", "Selecione um registro no grid primeiro", ae.window)
		return
	}
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Alterar só é suportado para tabelas SQLite por enquanto", ae.window)
		return
	}

	record := ae.records[ae.selectedRow]
	items, entries := ae.buildRecordFormItems(record.Fields)
	dialog.ShowForm("Alterar Registro", "Salvar", "Cancelar", items, func(confirmed bool) {
		if !confirmed {
			return
		}
		fields := make(map[string]interface{}, len(entries))
		for _, fe := range entries {
			fields[fe.field.Name] = fe.value()
		}
		if err := driver.UpdateRecord(record.Recno, shared.Record{Fields: fields}); err != nil {
			dialog.ShowError(fmt.Errorf("erro ao alterar registro: %w", err), ae.window)
			return
		}
		ae.loadTableData(ae.currentTable.Alias)
	}, ae.window)
}

// onDeleteRecord marca o registro selecionado como deletado (exclusão
// lógica — D_E_L_E_T_/R_E_C_D_E_L_, ver pkg/tools/shared/database.go).
func (ae *AdvEditorWindow) onDeleteRecord() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}
	if ae.selectedRow < 0 || ae.selectedRow >= len(ae.records) {
		dialog.ShowInformation("Aviso", "Selecione um registro no grid primeiro", ae.window)
		return
	}
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Excluir só é suportado para tabelas SQLite por enquanto", ae.window)
		return
	}

	record := ae.records[ae.selectedRow]
	dialog.ShowConfirm("Excluir Registro", "Confirma excluir este registro?", func(confirmed bool) {
		if !confirmed {
			return
		}
		if err := driver.DeleteRecord(record.Recno); err != nil {
			dialog.ShowError(fmt.Errorf("erro ao excluir registro: %w", err), ae.window)
			return
		}
		ae.selectedRow = -1
		ae.loadTableData(ae.currentTable.Alias)
	}, ae.window)
}

// onCreateIndex cria um índice (nome + lista de campos separados por "+",
// convenção Clipper: "CAMPO1+CAMPO2").
func (ae *AdvEditorWindow) onCreateIndex() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Índice só é suportado para tabelas SQLite por enquanto", ae.window)
		return
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("nome do índice")
	exprEntry := widget.NewEntry()
	exprEntry.SetPlaceHolder("CAMPO1+CAMPO2")

	dialog.ShowForm("Criar Índice", "Criar", "Cancelar", []*widget.FormItem{
		widget.NewFormItem("Nome", nameEntry),
		widget.NewFormItem("Campos", exprEntry),
	}, func(confirmed bool) {
		if !confirmed {
			return
		}
		if err := driver.CreateIndex(strings.TrimSpace(nameEntry.Text), strings.TrimSpace(exprEntry.Text)); err != nil {
			dialog.ShowError(fmt.Errorf("erro ao criar índice: %w", err), ae.window)
			return
		}
		ae.statusBar.SetText("Índice criado: " + nameEntry.Text)
	}, ae.window)
}

// onDropIndex remove um índice, escolhido de uma lista dos índices
// existentes na tabela atual.
func (ae *AdvEditorWindow) onDropIndex() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}
	driver := ae.currentDriver()
	if driver == nil {
		dialog.ShowInformation("Aviso", "Índice só é suportado para tabelas SQLite por enquanto", ae.window)
		return
	}
	indexes, err := driver.GetIndexes()
	if err != nil {
		dialog.ShowError(err, ae.window)
		return
	}
	if len(indexes) == 0 {
		dialog.ShowInformation("Aviso", "Esta tabela não tem índices", ae.window)
		return
	}
	names := make([]string, len(indexes))
	for i, ix := range indexes {
		names[i] = ix.Name
	}
	sel := widget.NewSelect(names, func(string) {})
	sel.SetSelectedIndex(0)
	dialog.ShowForm("Excluir Índice", "Excluir", "Cancelar", []*widget.FormItem{
		widget.NewFormItem("Índice", sel),
	}, func(confirmed bool) {
		if !confirmed || sel.Selected == "" {
			return
		}
		if err := driver.DropIndex(sel.Selected); err != nil {
			dialog.ShowError(fmt.Errorf("erro ao excluir índice: %w", err), ae.window)
			return
		}
		ae.statusBar.SetText("Índice excluído: " + sel.Selected)
	}, ae.window)
}

// onAbout exibe informações sobre
func (ae *AdvEditorWindow) onAbout() {
	dialog.ShowInformation("Sobre", fmt.Sprintf("AdvEditor %s\nEditor de Banco de Dados AdvPL\nInspirado em APSDU", version), ae.window)
}

// onChangeDatabase troca o banco de dados
func (ae *AdvEditorWindow) onChangeDatabase() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		// Obtém nome do arquivo
		uri := reader.URI()
		filePath := uri.Path()

		// Detecta tipo de arquivo automaticamente
		driver := "DBF"
		if strings.HasSuffix(strings.ToLower(filePath), ".db") ||
			strings.HasSuffix(strings.ToLower(filePath), ".sqlite") ||
			strings.HasSuffix(strings.ToLower(filePath), ".sqlite3") {
			driver = "SQLITE"
		}

		// Fecha tabela atual se existir
		if ae.currentTable != nil {
			ae.tableManager.CloseTable(ae.currentTable.Alias)
		}

		// Abre novo banco de dados
		tableInfo, err := ae.tableManager.OpenTable(filePath, driver, false, true)
		if err != nil {
			dialog.ShowError(err, ae.window)
			return
		}

		ae.currentTable = tableInfo
		ae.loadTablesFromDatabase()
		ae.statusBar.SetText("Banco de dados alterado: " + filePath)
	}, ae.window)
	if loc := ui.CurrentDirLocation(); loc != nil {
		fd.SetLocation(loc)
	}
	fd.Show()
}

// updateTreeView atualiza a tree view
func (ae *AdvEditorWindow) updateTreeView() {
	tables := ae.tableManager.GetTables()

	root := &shared.TreeNode{
		ID:       "root",
		Text:     "Tabelas",
		Children: make([]*shared.TreeNode, len(tables)),
	}

	for i, table := range tables {
		root.Children[i] = &shared.TreeNode{
			ID:   table.Alias,
			Text: table.Alias + " (" + table.File + ")",
			Data: table,
		}
	}

	ae.treeView = shared.NewTreeView(root)
	ae.treeView.SetOnSelect(func(node *shared.TreeNode) {
		ae.onTableSelected(node)
	})
}

// updateDataGrid atualiza o grid de dados
func (ae *AdvEditorWindow) updateDataGrid() {
	ae.dataGrid.Refresh()
}

// openDefaultDatabase abre o banco de dados padrão — sempre, mesmo que o
// arquivo ainda não exista (OpenSQLite/Open agora criam na hora; ver
// pkg/tools/shared). Sem isso, a primeira execução do AdvEditor num
// diretório novo nunca chegava a criar/abrir o banco local automático.
func (ae *AdvEditorWindow) openDefaultDatabase() {
	defaultDB := shared.ResolveDatabasePath("")

	tableInfo, err := ae.tableManager.OpenTable(defaultDB, "SQLITE", false, true)
	if err != nil {
		ae.statusBar.SetText("Erro ao abrir banco padrão: " + err.Error())
		return
	}

	ae.currentTable = tableInfo
	ae.loadTablesFromDatabase()
	ae.statusBar.SetText("Banco padrão aberto: " + defaultDB)
}

// loadTablesFromDatabase carrega as tabelas do banco de dados
func (ae *AdvEditorWindow) loadTablesFromDatabase() {
	if ae.currentTable == nil {
		return
	}

	// Obtém todas as tabelas do banco de dados
	tables := ae.listTablesFromDB()

	// Cria nós para cada tabela
	root := &shared.TreeNode{
		ID:       "root",
		Text:     "Tabelas",
		Children: make([]*shared.TreeNode, len(tables)),
	}

	for i, tableName := range tables {
		root.Children[i] = &shared.TreeNode{
			ID:   tableName,
			Text: tableName,
			Data: tableName,
		}
	}

	ae.treeView = shared.NewTreeView(root)
	ae.treeView.SetOnSelect(func(node *shared.TreeNode) {
		ae.onTableSelected(node)
	})

	// Atualiza a UI
	ae.updateUIWithNewTreeView()
}

// listTablesFromDB lista as tabelas do banco de dados
func (ae *AdvEditorWindow) listTablesFromDB() []string {
	if ae.currentTable == nil {
		return []string{}
	}

	// Para SQLite, precisamos consultar o banco diretamente
	// Tenta obter o driver SQLite
	if sqliteDriver, ok := ae.currentTable.DriverObj.(*shared.SQLiteDriver); ok {
		tables, err := sqliteDriver.ListTables()
		if err != nil {
			ae.statusBar.SetText("Erro ao listar tabelas: " + err.Error())
			return []string{}
		}
		return tables
	}

	// Fallback para tabelas conhecidas
	return []string{"SX2", "SX3", "SIX", "SX7", "SX5", "SX6", "SXB"}
}

// updateUIWithNewTreeView atualiza a UI com a nova tree view
func (ae *AdvEditorWindow) updateUIWithNewTreeView() {
	// Recria o layout com a nova tree view (Border: árvore com altura total)
	split := container.NewHSplit(
		container.NewBorder(
			widget.NewLabel("Tabelas"),
			nil, nil, nil,
			ae.treeView,
		),
		container.NewBorder(
			nil,
			ae.statusBar,
			nil,
			nil,
			ae.dataGrid,
		),
	)
	split.SetOffset(0.2)

	ae.window.SetContent(split)
}

// onTableSelected callback quando tabela é selecionada
func (ae *AdvEditorWindow) onTableSelected(node *shared.TreeNode) {
	if node.Data == nil {
		return
	}

	tableName := node.Data.(string)
	ae.statusBar.SetText("Tabela selecionada: " + tableName)
	ae.loadTableData(tableName)
}

// loadTableData carrega os dados da tabela
func (ae *AdvEditorWindow) loadTableData(tableName string) {
	if ae.currentTable == nil {
		return
	}

	tableInfo := ae.currentTable
	// Reaproveita a conexão SQLite já aberta em vez de sempre passar pelo
	// TableManager.OpenTable — que trata reabrir o MESMO caminho como
	// "tabela já aberta" e devolve um erro + o snapshot antigo, sem nunca
	// reconsultar. Isso fazia todo `loadTableData` chamado depois da
	// PRIMEIRA seleção da tabela (ou seja, todo refresh depois de Incluir/
	// Alterar/Excluir/Adicionar Campo/etc.) virar um no-op silencioso — o
	// grid nunca refletia a mudança recém-feita. driver.SelectTable troca/
	// recarrega a estrutura na mesma conexão, sem esse problema.
	if driver, ok := tableInfo.DriverObj.(*shared.SQLiteDriver); ok {
		if err := driver.SelectTable(tableName); err != nil {
			ae.statusBar.SetText("Erro ao abrir tabela: " + err.Error())
			return
		}
		structure, _ := driver.GetStructure()
		tableInfo.Structure = structure
		tableInfo.Alias = tableName
	} else {
		// Drivers não-SQLite (DBF/TopConnect/...) ainda não suportam trocar
		// de tabela na mesma conexão — mantém o caminho antigo pra eles.
		tablePath := ae.currentTable.File + "/" + tableName
		opened, err := ae.tableManager.OpenTable(tablePath, "SQLITE", false, true)
		if err != nil {
			ae.statusBar.SetText("Erro ao abrir tabela: " + err.Error())
			return
		}
		tableInfo = opened
		ae.currentTable = tableInfo
	}

	// Carrega a primeira página de registros
	records, err := tableInfo.DriverObj.GetData(0, pageSize)
	if err != nil {
		ae.statusBar.SetText("Erro ao ler dados: " + err.Error())
		records = nil
	}
	ae.records = records

	// Larguras de coluna: maior entre o nome do campo e o dado real carregado.
	// ponytail: field.Size mente para tabelas de dicionário (SX2/SX3/...),
	// que declaram TEXT genérico sem tamanho — por isso usamos o conteúdo
	// realmente lido em vez do Size do driver.
	for i, field := range tableInfo.Structure {
		chars := len(field.Name)
		for _, rec := range ae.records {
			if n := len(formatCell(rec.Fields[field.Name])); n > chars {
				chars = n
			}
		}
		if chars > 40 {
			chars = 40
		}
		if chars < 4 {
			chars = 4
		}
		ae.dataGrid.SetColumnWidth(i, float32(chars)*9+28)
	}

	ae.updateDataGrid()
	ae.statusBar.SetText(fmt.Sprintf("Tabela carregada: %s (%d registros exibidos)", tableName, len(ae.records)))
}

// pageSize é o máximo de registros carregados no grid por vez.
// ponytail: sem paginação por enquanto — adicionar navegação quando alguém
// precisar de tabelas com mais de 500 linhas
const pageSize = 500

// formatCell formata um valor de célula do banco para exibição
func formatCell(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimRight(v, " ")
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Show exibe a janela
func (ae *AdvEditorWindow) Show() {
	ae.window.ShowAndRun()
}

func main() {
	a := app.New()

	ae := NewAdvEditorWindow(a)
	ae.Show()
}
