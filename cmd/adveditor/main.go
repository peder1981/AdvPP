package main

import (
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
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		// Obtém nome do arquivo
		uri := reader.URI()
		filePath := uri.Path()

		// Abre tabela (usando DBF por padrão)
		tableInfo, err := ae.tableManager.OpenTable(filePath, "DBF", false, true)
		if err != nil {
			dialog.ShowError(err, ae.window)
			return
		}

		ae.currentTable = tableInfo
		ae.updateTreeView()
		ae.updateDataGrid()
		ae.statusBar.SetText("Tabela aberta: " + tableInfo.Alias)
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

// onTableSelected callback quando tabela é selecionada
func (ae *AdvEditorWindow) onTableSelected(node *shared.TreeNode) {
	// Implementar seleção de tabela
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

// Show exibe a janela
func (ae *AdvEditorWindow) Show() {
	ae.window.ShowAndRun()
}

func main() {
	a := app.New()

	ae := NewAdvEditorWindow(a)
	ae.Show()
}
