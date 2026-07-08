package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	
	"github.com/advpl/compiler/pkg/tools/shared"
)

// AdvCfgWindow representa a janela principal do AdvCfg
type AdvCfgWindow struct {
	window    fyne.Window
	treeView  *shared.TreeView
	dataGrid  *widget.Table
	statusBar *widget.Label
	current  string
}

// NewAdvCfgWindow cria uma nova janela do AdvCfg
func NewAdvCfgWindow(a fyne.App) *AdvCfgWindow {
	w := a.NewWindow("AdvCfg - Configurador de Tabelas")
	w.Resize(fyne.NewSize(1200, 800))
	
	ac := &AdvCfgWindow{
		window: w,
	}
	
	ac.setupUI()
	ac.setupMenu()
	
	return ac
}

// setupUI configura a interface do usuário
func (ac *AdvCfgWindow) setupUI() {
	// Cria tree view do dicionário
	root := &shared.TreeNode{
		ID:   "root",
		Text: "Dicionário de Dados",
		Children: []*shared.TreeNode{
			{
				ID:   "SX2",
				Text: "Tabelas (SX2)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:   "SX3",
				Text: "Campos (SX3)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:   "SIX",
				Text: "Índices (SIX)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:   "SX7",
				Text: "Triggers (SX7)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:   "SX5",
				Text: "Genéricas (SX5)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:   "SX6",
				Text: "Parâmetros (SX6)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:   "SXB",
				Text: "Perguntas (SXB)",
				Children: []*shared.TreeNode{},
			},
		},
	}
	
	ac.treeView = shared.NewTreeView(root)
	ac.treeView.SetOnSelect(func(node *shared.TreeNode) {
		ac.onNodeSelected(node)
	})
	
	// Cria grid de dados
	ac.dataGrid = widget.NewTable(
		func() (int, int) {
			return 10, 5 // 10 linhas por página, 5 colunas
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			label.SetText("")
		},
	)
	
	// Status bar
	ac.statusBar = widget.NewLabel("Pronto")
	
	// Layout principal
	split := container.NewHSplit(
		container.NewVBox(
			widget.NewLabel("Dicionário"),
			ac.treeView,
		),
		container.NewBorder(
			nil,
			ac.statusBar,
			nil,
			nil,
			ac.dataGrid,
		),
	)
	split.SetOffset(0.2)
	
	ac.window.SetContent(split)
}

// setupMenu configura o menu
func (ac *AdvCfgWindow) setupMenu() {
	fileMenu := fyne.NewMenu("Arquivo",
		fyne.NewMenuItem("Nova Tabela", ac.onNewTable),
		fyne.NewMenuItem("Importar Dicionário", ac.onImportDictionary),
		fyne.NewMenuItem("Exportar Dicionário", ac.onExportDictionary),
		fyne.NewMenuItem("Sair", func() {
			ac.window.Close()
		}),
	)
	
	editMenu := fyne.NewMenu("Editar",
		fyne.NewMenuItem("Incluir", ac.onAddRecord),
		fyne.NewMenuItem("Alterar", ac.onEditRecord),
		fyne.NewMenuItem("Excluir", ac.onDeleteRecord),
	)
	
	toolsMenu := fyne.NewMenu("Ferramentas",
		fyne.NewMenuItem("Validar Dicionário", ac.onValidateDictionary),
		fyne.NewMenuItem("Gerar Código", ac.onGenerateCode),
	)
	
	helpMenu := fyne.NewMenu("Ajuda",
		fyne.NewMenuItem("Sobre", ac.onAbout),
	)
	
	mainMenu := fyne.NewMainMenu(
		fileMenu,
		editMenu,
		toolsMenu,
		helpMenu,
	)
	
	ac.window.SetMainMenu(mainMenu)
}

// onNewTable cria uma nova tabela
func (ac *AdvCfgWindow) onNewTable() {
	dialog.ShowInformation("Info", "Funcionalidade de nova tabela será implementada", ac.window)
}

// onImportDictionary importa dicionário
func (ac *AdvCfgWindow) onImportDictionary() {
	dialog.ShowInformation("Info", "Funcionalidade de importar dicionário será implementada", ac.window)
}

// onExportDictionary exporta dicionário
func (ac *AdvCfgWindow) onExportDictionary() {
	dialog.ShowInformation("Info", "Funcionalidade de exportar dicionário será implementada", ac.window)
}

// onAddRecord adiciona um registro
func (ac *AdvCfgWindow) onAddRecord() {
	dialog.ShowInformation("Info", "Funcionalidade de adicionar registro será implementada", ac.window)
}

// onEditRecord edita um registro
func (ac *AdvCfgWindow) onEditRecord() {
	dialog.ShowInformation("Info", "Funcionalidade de editar registro será implementada", ac.window)
}

// onDeleteRecord deleta um registro
func (ac *AdvCfgWindow) onDeleteRecord() {
	dialog.ShowInformation("Info", "Funcionalidade de deletar registro será implementada", ac.window)
}

// onValidateDictionary valida dicionário
func (ac *AdvCfgWindow) onValidateDictionary() {
	dialog.ShowInformation("Info", "Funcionalidade de validar dicionário será implementada", ac.window)
}

// onGenerateCode gera código
func (ac *AdvCfgWindow) onGenerateCode() {
	dialog.ShowInformation("Info", "Funcionalidade de gerar código será implementada", ac.window)
}

// onAbout exibe informações sobre
func (ac *AdvCfgWindow) onAbout() {
	dialog.ShowInformation("Sobre", "AdvCfg v1.0\nConfigurador de Tabelas AdvPL\nInspirado em SIGACFG", ac.window)
}

// onNodeSelected callback quando nó é selecionado
func (ac *AdvCfgWindow) onNodeSelected(node *shared.TreeNode) {
	ac.current = node.ID
	ac.statusBar.SetText("Selecionado: " + node.Text)
	ac.updateDataGrid()
}

// updateDataGrid atualiza o grid de dados
func (ac *AdvCfgWindow) updateDataGrid() {
	ac.dataGrid.Refresh()
}

// Show exibe a janela
func (ac *AdvCfgWindow) Show() {
	ac.window.ShowAndRun()
}

func main() {
	a := app.New()
	
	ac := NewAdvCfgWindow(a)
	ac.Show()
}
