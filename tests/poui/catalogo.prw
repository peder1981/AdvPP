// Catálogo completo PO-UI: todos os componentes Po* numa página, mais uma
// navegação para cada página especial. Usado por tests/poui/ao-vivo.sh
// catalogo, que confere no navegador se cada um vira o elemento PO-UI
// certo, com conteúdo, e sem erros no console.

User Function Catalogo()
	Local oApp, oPg, oMenu, oNav, oHead, oPan
	Local oBtn, oTagClique, oInp, oTagInp, oAlvoPop
	Local oCt, oWg, oTabs, oAc, oSt, oMd, oTb
	Local oDd, oBg, oCm, oTl, oCh, oTr, oDv
	Local jDv := JsonObject():New()
	Local oRc, oFm
	Local aPaginas := {}
	Local nI

	oApp := PoApp():New("Catalogo")
	oPg  := PoPage():New(oApp, "Componentes")
	oMenu := PoMenu():New(oApp)
	oNav  := PoNavbar():New(oApp)
	oHead := PoHeader():New(oApp, "Catálogo")
	oPan  := PoMenuPanel():New(oApp)
	oMenu:AddItem("Componentes", "an an-house", {|| oApp:Navigate(oPg)})
	oNav:AddItem("Componentes", {|| oApp:Navigate(oPg)})
	oHead:SetUser("Operador", "operador@exemplo.com")
	oPan:AddItem("Componentes", "an an-house", {|| oApp:Navigate(oPg)})

	// interação ao vivo: clique e digitação voltam ao AdvPL
	oBtn := PoButton():New(oPg, "Clicar")
	oTagClique := PoTag():New(oPg, "sem clique", "neutral")
	oBtn:OnClick({|| oTagClique:SetValue("clicado") })
	oInp := PoInput():New(oPg, "Digite", "")
	oTagInp := PoTag():New(oPg, "vazio", "neutral")
	oInp:OnChange({|o, c| oTagInp:SetValue("eco:" + c) })

	// estrutura
	oCt := PoContainer():New(oPg, "Caixa")
	oWg := PoWidget():New(oCt, "Cartão")
	PoDivider():New(oWg, "Seção")
	oTabs := PoTabs():New(oWg)
	PoTag():New(PoTab():New(oTabs, "Aba A"), "conteúdo A", "info")
	PoTab():New(oTabs, "Aba B")
	oAc := PoAccordion():New(oPg)
	PoInfo():New(PoAccordionItem():New(oAc, "Seção 1"), "Chave", "Valor")
	oSt := PoStepper():New(oPg)
	PoStep():New(oSt, "Passo 1")
	PoStep():New(oSt, "Passo 2")
	oMd := PoModal():New(oPg, "Modal")
	PoButton():New(oMd, "Dentro do modal")
	oCt := PoContextTabs():New(oPg)
	oCt:AddTab("Contexto", oPg)

	// dados
	PoInfo():New(oPg, "Kernel", "6.12")
	PoProgress():New(oPg, "Andamento")
	PoGauge():New(oPg, "Medidor")
	oCh := PoChart():New(oPg, "CPU", "line")
	oCh:SetCategories({"jan", "fev", "mar"})
	oCh:SetSeries({{"cpu", {10, 20, 15}}})
	oTb := PoTable():New(oPg)
	oTb:SetColumns({{"cod", "Código"}, {"nome", "Nome"}})
	oTb:SetItems({{"1", "Ana"}, {"2", "Bia"}})
	oTr := PoTree():New(oPg)
	oTr:SetItems({{"Raiz", "r", {{"Filho 1", "f1"}, {"Filho 2", "f2"}}}})
	PoList():New(oPg)
	PoTag():New(oPg, "ok", "success")

	// campos básicos
	PoPassword():New(oPg, "Senha")
	PoTextarea():New(oPg, "Obs", "linha")
	PoNumber():New(oPg, "Qtd", 3)
	PoSelect():New(oPg, "Cor", {"azul", {"Verde", "g"}}, "g")
	PoCheckbox():New(oPg, "Aceito", .T.)
	PoSwitch():New(oPg, "Wi-Fi")

	// 3a: seleção, data/hora, texto, upload
	PoCombo():New(oPg, "UF", {{"SP", "São Paulo"}, {"RJ", "Rio"}}, "SP")
	PoMultiselect():New(oPg, "UFs", {{"SP", "São Paulo"}, {"RJ", "Rio"}}, {"SP"})
	PoRadioGroup():New(oPg, "Sexo", {{"M", "Masc"}, {"F", "Fem"}}, "M")
	PoCheckboxGroup():New(oPg, "Tipos", {{"A", "Tipo A"}, {"B", "Tipo B"}}, {"A"})
	PoLookup():New(oPg, "Armazém", "/rest/api/est/v1/armazens", "codigo", "descri")
	PoDatepicker():New(oPg, "Emissão", CToD("10/05/2026"))
	PoDatepickerRange():New(oPg, "Período", CToD("01/01/2026"), CToD("31/12/2026"))
	PoDatetimepicker():New(oPg, "Timestamp", "2026-05-10T14:30:00")
	PoTimepicker():New(oPg, "Hora", "08:30")
	PoDecimal():New(oPg, "Valor", 1.5, 2)
	PoEmail():New(oPg, "Email", "a@b.c")
	PoUrl():New(oPg, "Site", "https://exemplo.com")
	PoRichText():New(oPg, "Html", "<b>negrito</b>")
	PoSearch():New(oPg, "Buscar")
	PoUpload():New(oPg, "Anexo", "uploads-catalogo")

	// 3b: ações, navegação e exibição
	oBg := PoButtonGroup():New(oPg, "single")
	oBg:AddButton("Um", "", {|| .T.})
	oBg:AddButton("Dois", "", {|| .T.})
	oDd := PoDropdown():New(oPg, "Menu")
	oDd:AddAction("Ação", {|| .T.})
	oAlvoPop := PoButton():New(oPg, "Alvo")
	PoPopup():New(oPg, oAlvoPop)
	PoPopover():New(oPg, oAlvoPop, "Dica")
	oCm := PoContextMenu():New(oPg, "Contexto")
	oCm:AddItem("Item", {|| .T.})
	oTl := PoToolbar():New(oPg, "Barra")
	PoBreadcrumb():New(oPg, {{"Home", oPg}, {"Docs", "https://exemplo.com"}})
	PoLink():New(oPg, "Site", "https://exemplo.com")
	PoSlide():New(oPg, {{"a.png", "Slide A", "/x"}, {"b.png"}})
	PoAvatar():New(oPg, "avatar.png", "medium")
	PoBadge():New(oPg, 7, "caption-tag-01")
	PoImage():New(oPg, "foto.png", "foto")
	PoSkeleton():New(oPg, "text", "medium")
	PoHelper():New(oPg, "Texto de ajuda", "Ajuda")
	PoCalendar():New(oPg, CToD("01/02/2026"))
	oDv := PoDynamicView():New(oPg)
	oDv:SetFields({{"nome", "Nome", "string"}})
	jDv["nome"] := "Ana"
	oDv:SetValue(jDv)
	PoDisclaimerGroup():New(oPg, "Avisos", {"Termo 1", "Termo 2"})
	PoFilterChip():New(oPg, "Importante")

	// complementos do catálogo do llms.txt
	oFm := PoDynamicForm():New(oPg, {{"nome", "Nome", "string"}, {"idade", "Idade", "number"}}, jDv)
	oFm:OnChange({|o, cCampo, xValor| oTagInp:SetValue("form:" + cCampo + "=" + xValor) })
	PoLogin():New(oPg, "Usuário", "ana")
	oRc := PoModalPasswordRecovery():New(oPg, "email")
	oRc:SetContactEmail("suporte@exemplo.com")
	// abre por botão: aberto desde o início, o modal prenderia o foco
	PoButton():New(oPg, "recuperar"):OnClick({|| oRc:Open() })

	// páginas especiais (uma navegação para cada)
	AAdd(aPaginas, PoPageList():New(oApp, "Lista"))
	AAdd(aPaginas, PoPageEdit():New(oApp, "Editar"))
	AAdd(aPaginas, PoPageDetail():New(oApp, "Detalhe"))
	AAdd(aPaginas, PoPageDynamicTable():New(oApp, "Armazéns", "/rest/api/est/v1/armazens"))
	AAdd(aPaginas, PoPageDynamicEdit():New(oApp, "Editar armazém", "/rest/api/est/v1/armazens"))
	AAdd(aPaginas, PoPageDynamicDetail():New(oApp, "Armazém", "/rest/api/est/v1/armazens"))
	ATail(aPaginas):SetId("00001")
	AAdd(aPaginas, PoPageDynamicSearch():New(oApp, "Busca"))
	AAdd(aPaginas, PoPageLogin():New(oApp, "MeuERP"))
	AAdd(aPaginas, PoPageChangePassword():New(oApp))
	AAdd(aPaginas, PoPageBlockedUser():New(oApp, "tentativas"))
	AAdd(aPaginas, PoPageSlide():New(oApp, "Ajuda"))
	ATail(aPaginas):Open() // só existe no DOM quando aberto
	AAdd(aPaginas, PoPageJobScheduler():New(oApp, "Jobs", "/rest/api/jobs"))
	For nI := 1 To Len(aPaginas)
		IrPara(oApp, oPg, aPaginas[nI], nI)
	Next nI

	oApp:Activate()
Return

// IrPara cria o botão "ir:N" que navega para a página N (um bloco por
// chamada, para cada botão capturar a sua página).
Static Function IrPara(oApp, oPg, oDestino, nI)
	Local oB := PoButton():New(oPg, "ir:" + AllTrim(Str(nI)))
	oB:OnClick({|| oApp:Navigate(oDestino) })
Return
