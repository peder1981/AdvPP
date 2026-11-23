// Vitrine da Onda 3b — app de navegação completo: chrome (PoHeader,
// PoNavbar, PoMenuPanel), PoMenu, PoBreadcrumb, PoContextTabs, PoStepper,
// PoModal e componentes de exibição (3b.4). Compila com advplc check.

User Function Vitrine3B()
	Local oApp := PoApp():New("Vitrine 3b")
	Local oPg1 := PoPage():New(oApp, "Principal")
	Local oPg2 := PoPage():New(oApp, "Detalhes")
	Local oMenu := PoMenu():New(oApp)
	Local oNav  := PoNavbar():New(oApp)
	Local oHead := PoHeader():New(oApp)
	Local oPan  := PoMenuPanel():New(oApp)

	// chrome do app
	oNav:SetLogo("logo.png")
	oNav:AddItem("Principal", {|| oApp:Navigate(oPg1)})
	oNav:AddIconAction("an an-cog", "Config", {|| oApp:Notify("config")})
	oHead:SetUser("Maria Silva", "maria@empresa.com")
	oHead:AddAction("Sair", {|| oApp:Close()})
	oPan:AddItem("Ajuda", "an an-question", {|| oApp:Notify("ajuda")})

	// menu lateral
	oMenu:AddItem("Principal", "an an-home", {|| oApp:Navigate(oPg1)})
	oMenu:AddItem("Detalhes", "an an-list", {|| oApp:Navigate(oPg2)})

	// página 1: navegação + agrupamento + exibição
	Local oBc := PoBreadcrumb():New(oPg1, {{"Home", oPg1}, {"Principal", "https://doc"}})
	Local oCt := PoContextTabs():New(oPg1)
	oCt:AddTab("Aba 1", oPg1)
	oCt:AddTab("Aba 2", oPg2)
	Local oSt := PoStepper():New(oPg1)
	Local oSt1 := PoStep():New(oSt, "Dados")
	Local oSt2 := PoStep():New(oSt, "Revisão")
	Local oMd := PoModal():New(oPg1, "Confirmação")
	Local oBtn := PoButton():New(oPg1, "Abrir modal")
	oBtn:OnClick({|| oMd:Open()})
	Local oModalBtn := PoButton():New(oMd, "Fechar")
	oModalBtn:OnClick({|| oMd:Close()})

	// exibição
	Local oAv := PoAvatar():New(oPg1, "avatar.png", "medium")
	Local oBd := PoBadge():New(oPg1, 3, "caption-tag-01")
	Local oSk := PoSkeleton():New(oPg1, "text", "medium")
	Local oHp := PoHelper():New(oPg1, "Texto de ajuda", "Ajuda")
	Local oDg := PoDisclaimerGroup():New(oPg1, "Avisos", {"Termo 1", "Termo 2"})
	Local oFc := PoFilterChip():New(oPg1, "Importante")
	oFc:OnChange({|o, l| ConOut("chip=" + cValToChar(l))})
	Local oIm := PoImage():New(oPg1, "foto.png", "foto")

	// página 2: conteúdo simples
	Local oLbl := PoTag():New(oPg2, "Detalhes do registro", "info")

	oApp:Notify("Bem-vindo à vitrine 3b", "success", 4000, "top")
	oApp:Activate()
Return

Static Function cValToChar(x)
	If ValType(x) == "L"
		If x
			Return "T"
		EndIf
		Return "F"
	EndIf
Return Str(x)
