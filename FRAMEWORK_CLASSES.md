# Classes Complexas do Framework TOTVS

## Visão Geral

O AdvPP implementa estruturas de dados para classes complexas do framework TOTVS, permitindo que código Protheus utilize estas classes para definir interfaces de usuário e fluxos de trabalho.

## Classes Implementadas

### FWWizardControl

Classe para criação de wizards passo-a-passo.

**Estrutura:**
```go
type FWWizardControl struct {
    Name         string
    Title        string
    Steps        []*WizardStep
    CurrentStep  int
    Width        int
    Height       int
    ShowCancel   bool
    ShowHelp     bool
    FinishAction string
    CancelAction string
    HelpAction   string
}
```

**Métodos:**
- `AddStep(step *WizardStep)` - Adiciona um passo ao wizard
- `GetCurrentStep() *WizardStep` - Retorna o passo atual
- `NextStep() bool` - Avança para o próximo passo
- `PreviousStep() bool` - Volta ao passo anterior
- `CanMoveNext() bool` - Verifica se pode avançar
- `CanMovePrevious() bool` - Verifica se pode voltar
- `IsFirstStep() bool` - Verifica se é o primeiro passo
- `IsLastStep() bool` - Verifica se é o último passo
- `SetStep(stepIndex int) error` - Define o passo atual
- `GetStepCount() int` - Retorna total de passos
- `ValidateCurrentStep() error` - Valida o passo atual

**Exemplo de Uso:**
```advpl
oWizard := FWWizardControl()
oWizard:NAME := "WizardCliente"
oWizard:TITLE := "Cadastro de Cliente"
```

### FWDynDialog

Classe para criação de diálogos dinâmicos.

**Estrutura:**
```go
type FWDynDialog struct {
    Name       string
    Title      string
    Width      int
    Height     int
    Components []*Component
    Buttons    []*Component
    Modal      bool
    Resizable  bool
    Position   string // "CENTER", "TOP", "BOTTOM", etc.
}
```

**Métodos:**
- `AddComponent(comp *Component)` - Adiciona componente ao diálogo
- `AddButton(button *Component)` - Adiciona botão ao diálogo

### FWPanel

Classe para container de componentes.

**Estrutura:**
```go
type FWPanel struct {
    Name        string
    Components  []*Component
    Width       int
    Height      int
    BackColor   string
    BorderStyle string
}
```

**Métodos:**
- `AddComponent(comp *Component)` - Adiciona componente ao painel

### FWGroupBox

Classe para grupo de componentes com título.

**Estrutura:**
```go
type FWGroupBox struct {
    Name       string
    Title      string
    Components []*Component
    Width      int
    Height     int
    BackColor  string
}
```

**Métodos:**
- `AddComponent(comp *Component)` - Adiciona componente ao group box

### FWTabs

Classe para controle de abas.

**Estrutura:**
```go
type FWTabs struct {
    Name      string
    TabPages  []*TabPage
    ActiveTab int
    Width     int
    Height    int
    Position  string // "TOP", "BOTTOM", "LEFT", "RIGHT"
}
```

**Métodos:**
- `AddTabPage(tab *TabPage)` - Adiciona página de aba
- `GetActiveTab() *TabPage` - Retorna aba ativa
- `SetActiveTab(tabIndex int) error` - Define aba ativa

### FWSplitter

Classe para divisor de painéis.

**Estrutura:**
```go
type FWSplitter struct {
    Name        string
    Orientation string // "HORIZONTAL", "VERTICAL"
    Panel1      *FWPanel
    Panel2      *FWPanel
    SplitterPos int
    Width       int
    Height      int
    Resizable   bool
}
```

### FWTreeView

Classe para visualização em árvore.

**Estrutura:**
```go
type FWTreeView struct {
    Name        string
    Nodes       []*TreeNode
    Width       int
    Height      int
    ShowLines   bool
    ShowButtons bool
}
```

**Métodos:**
- `AddNode(node *TreeNode)` - Adiciona nó à árvore

### FWListView

Classe para visualização em lista.

**Estrutura:**
```go
type FWListView struct {
    Name      string
    Columns   []*ListViewColumn
    Items     []*ListViewItem
    Width     int
    Height    int
    ViewStyle string // "REPORT", "ICON", "SMALLICON", "LIST"
}
```

**Métodos:**
- `AddColumn(column *ListViewColumn)` - Adiciona coluna
- `AddItem(item *ListViewItem)` - Adiciona item

## Registro no VM

As classes complexas foram registradas na VM para permitir instanciação:

```go
case "FWWIZARDCONTROL":
    obj := advplrt.NewObject("FWWizardControl", cls)
    obj.Props["NAME"] = advplrt.NewString("")
    obj.Props["TITLE"] = advplrt.NewString("")
    obj.Props["CURRENTSTEP"] = advplrt.NewNumber(0)
    v.push(obj)
    return nil
```

Classes registradas:
- FWWizardControl
- FWDynDialog
- FWPanel
- FWGroupBox
- FWTabs
- FWSplitter
- FWTreeView
- FWListView

## Limitações Atuais

1. **Renderização Visual**: As classes são estruturas de dados e não possuem renderização visual automática
2. **Integração UI**: Para renderização completa, é necessário integrar com Fyne ou outro framework de UI
3. **Eventos**: Manipuladores de eventos não estão conectados automaticamente
4. **Validação**: Validação de campos requer implementação adicional

## Próximos Passos

1. Implementar renderização Fyne para classes complexas
2. Conectar manipuladores de eventos
3. Implementar validação de campos
4. Adicionar suporte para estilos e temas
5. Implementar persistência de estado

## Exemplo de Teste

Veja `tests/framework_classes_test.prw` para um exemplo de uso das classes do framework.

## Compatibilidade

| Classe | Status | Notas |
|--------|--------|-------|
| FWWizardControl | ✅ Estrutura de dados | Implementada, aguarda renderização |
| FWDynDialog | ✅ Estrutura de dados | Implementada, aguarda renderização |
| FWPanel | ✅ Estrutura de dados | Implementada, aguarda renderização |
| FWGroupBox | ✅ Estrutura de dados | Implementada, aguarda renderização |
| FWTabs | ✅ Estrutura de dados | Implementada, aguarda renderização |
| FWSplitter | ✅ Estrutura de dados | Implementada, aguarda renderização |
| FWTreeView | ✅ Estrutura de dados | Implementada, aguarda renderização |
| FWListView | ✅ Estrutura de dados | Implementada, aguarda renderização |
