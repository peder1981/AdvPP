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
)

// AdvCfgWindow representa a janela principal do AdvCfg
type AdvCfgWindow struct {
	window       fyne.Window
	treeView     *shared.TreeView
	dataGrid     *widget.Table
	statusBar    *widget.Label
	current      string
	dictionary   *shared.Dictionary
	currentTable string
	currentData  []map[string]interface{}
}

// fieldColumns são as colunas do SX3 exibidas no grid de campos.
// ponytail: SX3 tem 82 colunas; mostrar só as relevantes para edição.
var fieldColumns = []string{"X3_CAMPO", "X3_TITULO", "X3_TIPO", "X3_TAMANHO", "X3_DECIMAL", "X3_DESCRIC"}

// formatCell formata um valor de célula do dicionário para exibição
func formatCell(val interface{}) string {
	switch v := val.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimRight(v, " ")
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// NewAdvCfgWindow cria uma nova janela do AdvCfg
func NewAdvCfgWindow(a fyne.App) *AdvCfgWindow {
	w := a.NewWindow("AdvCfg - Configurador de Tabelas")
	w.Resize(fyne.NewSize(1200, 800))

	ac := &AdvCfgWindow{
		window: w,
	}

	// Carrega dicionário
	dict, err := shared.NewDictionary(shared.ResolveDatabasePath(""))
	if err != nil {
		dialog.ShowError(err, w)
		return nil
	}
	ac.dictionary = dict

	ac.setupUI()
	ac.setupMenu()
	ac.loadDictionaryData()

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
				ID:       "SX2",
				Text:     "Tabelas (SX2)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:       "SX3",
				Text:     "Campos (SX3)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:       "SIX",
				Text:     "Índices (SIX)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:       "SX7",
				Text:     "Triggers (SX7)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:       "SX5",
				Text:     "Genéricas (SX5)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:       "SX6",
				Text:     "Parâmetros (SX6)",
				Children: []*shared.TreeNode{},
			},
			{
				ID:       "SXB",
				Text:     "Perguntas (SXB)",
				Children: []*shared.TreeNode{},
			},
		},
	}

	ac.treeView = shared.NewTreeView(root)
	ac.treeView.SetOnSelect(func(node *shared.TreeNode) {
		ac.onNodeSelected(node)
	})

	// Cria grid de dados: linha 0 = cabeçalho, demais = campos da tabela selecionada
	ac.dataGrid = widget.NewTable(
		func() (int, int) {
			return len(ac.currentData) + 1, len(fieldColumns)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			return label
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			col := fieldColumns[id.Col]

			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText(col)
				return
			}
			label.TextStyle = fyne.TextStyle{}
			if id.Row-1 >= len(ac.currentData) {
				label.SetText("")
				return
			}
			label.SetText(formatCell(ac.currentData[id.Row-1][col]))
		},
	)

	// Status bar
	ac.statusBar = widget.NewLabel("Pronto")

	// Layout principal
	split := container.NewHSplit(
		container.NewBorder(
			widget.NewLabel("Dicionário"),
			nil, nil, nil,
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
		fyne.NewMenuItem("Trocar Dicionário", ac.onChangeDictionary),
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

// onChangeDictionary troca o dicionário
func (ac *AdvCfgWindow) onChangeDictionary() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		// Obtém nome do arquivo
		uri := reader.URI()
		filePath := uri.Path()

		// Fecha dicionário atual se existir
		if ac.dictionary != nil {
			ac.dictionary.Close()
		}

		// Abre novo dicionário
		dict, err := shared.NewDictionary(filePath)
		if err != nil {
			dialog.ShowError(err, ac.window)
			return
		}
		ac.dictionary = dict

		// Recarrega dados
		ac.loadDictionaryData()
		ac.statusBar.SetText("Dicionário alterado: " + filePath)
	}, ac.window)
}

// loadDictionaryData carrega dados do dicionário e popula a tree view
func (ac *AdvCfgWindow) loadDictionaryData() {
	if ac.dictionary == nil {
		return
	}

	// Obtém tabelas do dicionário
	tables, err := ac.dictionary.GetTables()
	if err != nil {
		ac.statusBar.SetText("Erro ao carregar dicionário: " + err.Error())
		return
	}

	// Cria nós de tabelas
	sx2Node := &shared.TreeNode{
		ID:       "SX2",
		Text:     "Tabelas (SX2)",
		Children: make([]*shared.TreeNode, len(tables)),
	}

	for i, table := range tables {
		chave := table["X2_CHAVE"].(string)
		alias := table["X2_ALIAS"].(string)
		nome := table["X2_NOMEUSR"].(string)

		sx2Node.Children[i] = &shared.TreeNode{
			ID:   chave,
			Text: alias + " - " + nome,
			Data: table,
		}
	}

	// Atualiza tree view
	root := ac.treeView.GetRoot()
	for i, child := range root.Children {
		if child.ID == "SX2" {
			root.Children[i] = sx2Node
			break
		}
	}

	ac.treeView.Refresh()
	ac.statusBar.SetText("Dicionário carregado: " + fmt.Sprintf("%d", len(tables)) + " tabelas")
}

// onNodeSelected callback quando nó é selecionado
func (ac *AdvCfgWindow) onNodeSelected(node *shared.TreeNode) {
	ac.current = node.ID
	ac.statusBar.SetText("Selecionado: " + node.Text)

	// Se for uma tabela, carrega campos
	if ac.current == "SX2" || node.Data != nil {
		if tableData, ok := node.Data.(map[string]interface{}); ok {
			ac.currentTable = tableData["X2_ALIAS"].(string)
			ac.loadTableFields(ac.currentTable)
		}
	} else {
		ac.currentTable = ""
		ac.currentData = nil
	}

	ac.updateDataGrid()
}

// loadTableFields carrega campos de uma tabela
func (ac *AdvCfgWindow) loadTableFields(table string) {
	if ac.dictionary == nil {
		return
	}

	fields, err := ac.dictionary.GetFields(table)
	if err != nil {
		ac.statusBar.SetText("Erro ao carregar campos: " + err.Error())
		return
	}

	ac.currentData = fields
	ac.statusBar.SetText("Tabela: " + table + " - " + fmt.Sprintf("%d", len(fields)) + " campos")
}

// updateDataGrid atualiza o grid de dados
func (ac *AdvCfgWindow) updateDataGrid() {
	// Larguras de coluna: maior entre o nome da coluna e o dado real carregado
	for i, col := range fieldColumns {
		chars := len(col)
		for _, rec := range ac.currentData {
			if n := len(formatCell(rec[col])); n > chars {
				chars = n
			}
		}
		if chars > 40 {
			chars = 40
		}
		if chars < 4 {
			chars = 4
		}
		ac.dataGrid.SetColumnWidth(i, float32(chars)*9+28)
	}
	ac.dataGrid.Refresh()
}

// Show exibe a janela
func (ac *AdvCfgWindow) Show() {
	ac.window.ShowAndRun()
}

func main() {
	a := app.New()

	ac := NewAdvCfgWindow(a)
	if ac != nil {
		ac.Show()
	}
}
