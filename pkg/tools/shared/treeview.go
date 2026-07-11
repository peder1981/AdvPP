package shared

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// TreeNode representa um nó na tree view
type TreeNode struct {
	ID       string
	Text     string
	Icon     fyne.Resource
	Children []*TreeNode
	Data     interface{}
	Expanded bool
}

// TreeView é uma tree view genérica
type TreeView struct {
	widget.BaseWidget
	tree     *widget.Tree
	root     *TreeNode
	onSelect func(node *TreeNode)
	onExpand func(node *TreeNode, expanded bool)
	selected string
}

// NewTreeView cria uma nova tree view
func NewTreeView(root *TreeNode) *TreeView {
	tv := &TreeView{
		root: root,
	}

	tv.tree = widget.NewTree(
		func(nodeID widget.TreeNodeID) []widget.TreeNodeID {
			return tv.getChildren(nodeID)
		},
		func(nodeID widget.TreeNodeID) bool {
			return tv.hasChildren(nodeID)
		},
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(nodeID widget.TreeNodeID, branch bool, item fyne.CanvasObject) {
			node := tv.findNode(nodeID)
			if node != nil {
				item.(*widget.Label).SetText(node.Text)
			}
		},
	)

	tv.tree.OpenAllBranches()
	// Obrigatório para widgets custom que embutem widget.BaseWidget — sem
	// isso o Fyne nunca aprende a usar CreateRenderer deste tipo, e o
	// widget renderiza com tamanho mínimo/errado (a raiz da árvore
	// aparecer minúscula/cortada, mesmo com dados carregados certos).
	tv.ExtendBaseWidget(tv)

	return tv
}

// CreateRenderer cria o renderer
func (tv *TreeView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(tv.tree)
}

// hasChildren verifica se nó tem filhos
func (tv *TreeView) hasChildren(nodeID widget.TreeNodeID) bool {
	node := tv.findNode(nodeID)
	if node == nil {
		return false
	}
	return len(node.Children) > 0
}

// getChildren retorna filhos do nó
func (tv *TreeView) getChildren(nodeID widget.TreeNodeID) []widget.TreeNodeID {
	node := tv.findNode(nodeID)
	if node == nil {
		return []widget.TreeNodeID{}
	}

	ids := make([]widget.TreeNodeID, len(node.Children))
	for i, child := range node.Children {
		ids[i] = widget.TreeNodeID(child.ID)
	}
	return ids
}

// findNode encontra um nó pelo ID. widget.Tree do Fyne usa "" (string
// vazia) como o ID convencional do nó raiz — não o ID literal que o
// TreeNode raiz recebeu ("root" neste pacote) — sem tratar esse caso à
// parte, getChildren("") nunca encontrava o nó raiz de verdade e a árvore
// aparecia permanentemente vazia, não importava quantos nós existissem.
func (tv *TreeView) findNode(nodeID widget.TreeNodeID) *TreeNode {
	if nodeID == "" {
		return tv.root
	}
	return tv.findNodeRecursive(tv.root, string(nodeID))
}

// findNodeRecursive busca recursivamente
func (tv *TreeView) findNodeRecursive(node *TreeNode, id string) *TreeNode {
	if node == nil {
		return nil
	}

	if node.ID == id {
		return node
	}

	for _, child := range node.Children {
		if found := tv.findNodeRecursive(child, id); found != nil {
			return found
		}
	}

	return nil
}

// SetOnSelect define callback de seleção
func (tv *TreeView) SetOnSelect(callback func(node *TreeNode)) {
	tv.onSelect = callback
	tv.tree.OnSelected = func(id widget.TreeNodeID) {
		tv.selected = string(id)
		node := tv.findNode(id)
		if node != nil && tv.onSelect != nil {
			tv.onSelect(node)
		}
	}
}

// SetOnExpand define callback de expansão
func (tv *TreeView) SetOnExpand(callback func(node *TreeNode, expanded bool)) {
	tv.onExpand = callback
	tv.tree.OnBranchOpened = func(id widget.TreeNodeID) {
		node := tv.findNode(id)
		if node != nil && tv.onExpand != nil {
			node.Expanded = true
			tv.onExpand(node, true)
		}
	}
	tv.tree.OnBranchClosed = func(id widget.TreeNodeID) {
		node := tv.findNode(id)
		if node != nil && tv.onExpand != nil {
			node.Expanded = false
			tv.onExpand(node, false)
		}
	}
}

// GetSelectedNode retorna o nó selecionado
func (tv *TreeView) GetSelectedNode() *TreeNode {
	if tv.selected == "" {
		return nil
	}
	return tv.findNode(widget.TreeNodeID(tv.selected))
}

// SelectNode seleciona um nó
func (tv *TreeView) SelectNode(nodeID string) {
	tv.tree.Select(widget.TreeNodeID(nodeID))
}

// ExpandNode expande um nó
func (tv *TreeView) ExpandNode(nodeID string) {
	tv.tree.OpenBranch(widget.TreeNodeID(nodeID))
}

// CollapseNode colapsa um nó
func (tv *TreeView) CollapseNode(nodeID string) {
	tv.tree.CloseBranch(widget.TreeNodeID(nodeID))
}

// Refresh atualiza a tree view
func (tv *TreeView) Refresh() {
	tv.tree.Refresh()
}

// AddChild adiciona filho a um nó
func (tv *TreeView) AddChild(parentID string, child *TreeNode) {
	parent := tv.findNode(parentID)
	if parent != nil {
		parent.Children = append(parent.Children, child)
		tv.tree.Refresh()
	}
}

// RemoveNode remove um nó
func (tv *TreeView) RemoveNode(nodeID string) {
	tv.removeNodeRecursive(tv.root, nodeID)
	tv.tree.Refresh()
}

// removeNodeRecursive remove recursivamente
func (tv *TreeView) removeNodeRecursive(parent *TreeNode, id string) bool {
	if parent == nil {
		return false
	}

	for i, child := range parent.Children {
		if child.ID == id {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return true
		}

		if tv.removeNodeRecursive(child, id) {
			return true
		}
	}

	return false
}

// UpdateNode atualiza um nó
func (tv *TreeView) UpdateNode(nodeID string, text string) {
	node := tv.findNode(nodeID)
	if node != nil {
		node.Text = text
		tv.tree.Refresh()
	}
}

// GetRoot retorna a raiz
func (tv *TreeView) GetRoot() *TreeNode {
	return tv.root
}

// ContainerTreeView cria uma tree view com container
func ContainerTreeView(root *TreeNode) *container.Split {
	tv := NewTreeView(root)
	return container.NewVSplit(tv, widget.NewLabel("Selecione um item"))
}
