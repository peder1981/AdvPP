// Fixture da Onda 3c — serviços e diretivas PO-UI: tema, idioma,
// mídia, storage (localStorage), rede, guia, tooltip e drag & drop.
// Botões disparam as APIs; os handlers ConOut logam o que voltou do
// browser (storage, guideend) ou o que o servidor aplicou.

User Function ServicosDemo()
	Local oApp, oMenu, oPg, oBox, oB1, oB2, oTema

	oApp := PoApp():New("Servicos 3c")
	oMenu := PoMenu():New(oApp)
	oPg := PoPage():New(oApp, "Principal")

	// 3c.1 — tema e idioma globais
	oApp:SetTheme("light")
	oApp:SetLanguage("pt")

	// 3c.1/3c.2 — eventos de app
	oApp:OnMediaChange({|oSelf, cM| ConOut("[3c] media=" + cM)})
	oApp:OnNetworkChange({|oSelf, lOn| ConOut("[3c] rede=" + cValToChar(lOn))})

	// 3c.3 — guia (passos apontam para componentes reais)
	oApp:OnGuideEnd({|oSelf, bOk| ConOut("[3c] guia fim=" + cValToChar(bOk))})

	// 3c.2 — storage: grava, lê de volta (round-trip síncrono) e limpa
	oApp:StorageSet("fixture", "onda3c")

	// 3c.3 — tooltip em qualquer componente
	oTema := PoButton():New(oPg, "Tema escuro", {|| oApp:SetTheme("dark")})
	oTema:SetTooltip("Alterna para o tema escuro", "top")

	Local oStorage := PoButton():New(oPg, "Storage", {||
		Local x := oApp:StorageGet("fixture")
		ConOut("[3c] storage=" + cValToChar(x))
		oApp:StorageRemove("fixture")
	})
	oStorage:SetTooltip("Lê e remove a chave 'fixture'", "bottom")

	Local oGuia := PoButton():New(oPg, "Guia", {||
		oApp:StartGuide({{oTema, "Tema", "Este botão troca o tema."}, ;
			{oStorage, "Storage", "Lê o valor gravado no navegador."}})
	})

	// 3c.3 — drag & drop: reordena os filhos do container
	oBox := PoContainer():New(oPg, "Itens arrastaveis")
	oB1 := PoButton():New(oBox, "Item 1")
	oB2 := PoButton():New(oBox, "Item 2")
	oB1:SetDraggable(.T.)
	oB2:SetDraggable(.T.)
	oBox:OnDrop({|oSelf, oArr, nPos|
		ConOut("[3c] drop " + cValToChar(oArr:Get("label")) + " pos=" + cValToChar(nPos))
	})

	oMenu:AddItem("Principal", "an an-gear", {|| oApp:Navigate(oPg)})
	oApp:Navigate(oPg)
	oApp:Activate()
Return Nil
