package main

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/advpl/compiler/pkg/tools/shared"
)

// AdvEditorWindow representa a janela principal do AdvEditor
type AdvEditorWindow struct {
	window       fyne.Window
	tableManager *shared.TableManager
	treeView     *shared.TreeView
	dataGrid     *widget.Table
	statusBar    *widget.Label
	currentTable *shared.TableInfo
}

// NewAdvEditorWindow cria uma nova janela do AdvEditor
func NewAdvEditorWindow(a fyne.App) *AdvEditorWindow {
	w := a.NewWindow("AdvEditor - Editor de Banco de Dados")
	w.Resize(fyne.NewSize(1200, 800))

	ae := &AdvEditorWindow{
		window:       w,
		tableManager: shared.NewTableManager(),
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

	// Cria grid de dados
	ae.dataGrid = widget.NewTable(
		func() (int, int) {
			if ae.currentTable == nil {
				return 0, 0
			}
			return 10, len(ae.currentTable.Structure) // 10 linhas por página
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			if ae.currentTable == nil {
				label.SetText("")
				return
			}

			if id.Row == 0 {
				// Cabeçalho
				if id.Col < len(ae.currentTable.Structure) {
					label.SetText(ae.currentTable.Structure[id.Col].Name)
				}
			} else {
				// Dados
				label.SetText("")
			}
		},
	)

	// Status bar
	ae.statusBar = widget.NewLabel("Pronto")

	// Layout principal
	split := container.NewHSplit(
		container.NewVBox(
			widget.NewLabel("Tabelas Abertas"),
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
		fyne.NewMenuItem("Estrutura", ae.onViewStructure),
		fyne.NewMenuItem("Sair", func() {
			ae.window.Close()
		}),
	)

	editMenu := fyne.NewMenu("Editar",
		fyne.NewMenuItem("Incluir", ae.onAddRecord),
		fyne.NewMenuItem("Alterar", ae.onEditRecord),
		fyne.NewMenuItem("Excluir", ae.onDeleteRecord),
	)

	indexMenu := fyne.NewMenu("Índice",
		fyne.NewMenuItem("Abrir", ae.onOpenIndex),
		fyne.NewMenuItem("Criar", ae.onCreateIndex),
		fyne.NewMenuItem("Fechar", ae.onCloseIndex),
	)

	helpMenu := fyne.NewMenu("Ajuda",
		fyne.NewMenuItem("Sobre", ae.onAbout),
	)

	mainMenu := fyne.NewMainMenu(
		fileMenu,
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

	dialog.ShowForm("Selecionar Driver", "Cancelar", "OK", []*widget.FormItem{
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
func (ae *AdvEditorWindow) selectFile(driver string, shared, readonly bool) {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		// Obtém nome do arquivo
		uri := reader.URI()
		filePath := uri.Path()

		// Mapeia driver para código interno
		driverCode := "DBF"
		switch driver {
		case "SQLite":
			driverCode = "SQLITE"
		case "DBF":
			driverCode = "DBF"
		case "TopConnect":
			driverCode = "TOPCONNECT"
		case "Ctree":
			driverCode = "CTREECDX"
		case "BTrieve":
			driverCode = "BTVCDX"
		}

		// Fecha tabela atual se existir
		if ae.currentTable != nil {
			ae.tableManager.CloseTable(ae.currentTable.Alias)
		}

		// Abre banco de dados
		tableInfo, err := ae.tableManager.OpenTable(filePath, driverCode, readonly, shared)
		if err != nil {
			dialog.ShowError(err, ae.window)
			return
		}

		ae.currentTable = tableInfo

		// Carrega tabelas do banco
		ae.loadTablesFromDatabase()

		// Se for SQLite, mostra diálogo para selecionar tabela
		if driverCode == "SQLITE" {
			ae.selectTable()
		}

		ae.statusBar.SetText("Banco de dados aberto: " + filePath)
	}, ae.window)
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

	dialog.ShowForm("Selecionar Tabela", "Cancelar", "OK", []*widget.FormItem{
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

// onViewStructure exibe a estrutura da tabela
func (ae *AdvEditorWindow) onViewStructure() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}

	content := widget.NewRichTextFromMarkdown("## Estrutura da Tabela: " + ae.currentTable.Alias + "\n\n")

	for _, field := range ae.currentTable.Structure {
		content.ParseMarkdown(content.String() +
			"- **" + field.Name + "**: " + string(field.Type) +
			"(" + string(rune(field.Size)) + "," + string(rune(field.Decimal)) + ")\n")
	}

	dialog.ShowCustom("Estrutura", "Fechar", container.NewScroll(content), ae.window)
}

// onAddRecord adiciona um registro
func (ae *AdvEditorWindow) onAddRecord() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}

	dialog.ShowInformation("Info", "Funcionalidade de adicionar registro será implementada", ae.window)
}

// onEditRecord edita um registro
func (ae *AdvEditorWindow) onEditRecord() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}

	dialog.ShowInformation("Info", "Funcionalidade de editar registro será implementada", ae.window)
}

// onDeleteRecord deleta um registro
func (ae *AdvEditorWindow) onDeleteRecord() {
	if ae.currentTable == nil {
		dialog.ShowInformation("Aviso", "Nenhuma tabela selecionada", ae.window)
		return
	}

	dialog.ShowInformation("Info", "Funcionalidade de deletar registro será implementada", ae.window)
}

// onOpenIndex abre um índice
func (ae *AdvEditorWindow) onOpenIndex() {
	dialog.ShowInformation("Info", "Funcionalidade de abrir índice será implementada", ae.window)
}

// onCreateIndex cria um índice
func (ae *AdvEditorWindow) onCreateIndex() {
	dialog.ShowInformation("Info", "Funcionalidade de criar índice será implementada", ae.window)
}

// onCloseIndex fecha um índice
func (ae *AdvEditorWindow) onCloseIndex() {
	dialog.ShowInformation("Info", "Funcionalidade de fechar índice será implementada", ae.window)
}

// onAbout exibe informações sobre
func (ae *AdvEditorWindow) onAbout() {
	dialog.ShowInformation("Sobre", "AdvEditor v1.0\nEditor de Banco de Dados AdvPL\nInspirado em APSDU", ae.window)
}

// onChangeDatabase troca o banco de dados
func (ae *AdvEditorWindow) onChangeDatabase() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
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

// openDefaultDatabase abre o banco de dados padrão
func (ae *AdvEditorWindow) openDefaultDatabase() {
	defaultDB := "./data/advpl_dictionary.db"

	// Tenta abrir o banco de dados padrão
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
	// Recria o layout com a nova tree view
	split := container.NewHSplit(
		container.NewVBox(
			widget.NewLabel("Tabelas"),
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

	// Abre a tabela específica
	tablePath := ae.currentTable.File + "/" + tableName
	tableInfo, err := ae.tableManager.OpenTable(tablePath, "SQLITE", false, true)
	if err != nil {
		ae.statusBar.SetText("Erro ao abrir tabela: " + err.Error())
		return
	}

	ae.currentTable = tableInfo
	ae.updateDataGrid()
	ae.statusBar.SetText("Tabela carregada: " + tableName)
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
