// Fixture da Onda 3a.6 — templates dinamicos PO-UI ligados ao REST da
// onda 1. PoPageDynamicTable/Edit/Detail/Search consomem
// /rest/api/est/v1/armazens (contrato {items,hasNext}); OnBeforeSave/
// OnBeforeRemove fazem o round-trip sincrono browser->AdvPL.
// O endpoint da fixture e somente-leitura: a gravacao real depende de um
// repositorio com POST/PUT/DELETE — aqui validamos mount, campos,
// filtros e os hooks (o POST falha com erro HTTP, como esperado).

User Function CrudDemo()
	Local oApp, oMenu, oTable, oEdit, oDetail, oSearch

	oApp := PoApp():New("CRUD Armazens")
	oMenu := PoMenu():New(oApp)

	oTable := PoPageDynamicTable():New(oApp, "Armazens", "/rest/api/est/v1/armazens")
	oTable:SetFields({{"codigo", "Codigo", "string", .T., .T., .T.}, ;
		{"descri", "Descricao", "string", .F., .T., .T.}, ;
		{"estoque", "Estoque", "number", .F., .F., .T.}})
	oTable:SetQuickSearchField("codigo")
	oTable:OnQuickSearch({|oSelf, cTexto| ConOut("[dyn] quicksearch: " + cTexto)})
	oTable:OnCustomAction({|oSelf, cAcao, oLinha| ConOut("[dyn] acao " + cAcao)})

	oEdit := PoPageDynamicEdit():New(oApp, "Editar Armazem", "/rest/api/est/v1/armazens")
	oEdit:SetFields({{"codigo", "Codigo", "string", .T., .T., .T.}, ;
		{"descri", "Descricao", "string", .F., .T., .T.}, ;
		{"estoque", "Estoque", "number", .F., .F., .T.}})
	oEdit:SetDuplicates({"codigo"})
	oEdit:OnBeforeSave({|oSelf, oRegistro|
		ConOut("[dyn] beforesave codigo=" + oRegistro["codigo"])
		Return .T.
	})

	oDetail := PoPageDynamicDetail():New(oApp, "Armazem", "/rest/api/est/v1/armazens")
	oDetail:SetFields({{"codigo", "Codigo", "string", .T., .T., .T.}, ;
		{"descri", "Descricao", "string", .F., .T., .T.}, ;
		{"estoque", "Estoque", "number", .F., .F., .T.}})
	oDetail:OnBeforeRemove({|oSelf, oRegistro|
		ConOut("[dyn] beforeremove codigo=" + oRegistro["codigo"])
		Return .T.
	})

	oSearch := PoPageDynamicSearch():New(oApp, "Pesquisa", "/rest/api/est/v1/armazens")
	oSearch:SetFields({{"codigo", "Codigo", "string", .T., .F., .T.}, ;
		{"descri", "Descricao", "string", .F., .T., .T.}})
	oSearch:SetQuickFilter("codigo")

	oMenu:AddItem("Listagem", "an an-list", {|| oApp:Navigate(oTable)})
	oMenu:AddItem("Editar", "an an-edit", {|| oApp:Navigate(oEdit)})
	oMenu:AddItem("Detalhe", "an an-eye", {|| oApp:Navigate(oDetail)})
	oMenu:AddItem("Pesquisa", "an an-magnifying-glass", {|| oApp:Navigate(oSearch)})

	oApp:Navigate(oTable)
	oApp:Activate()
Return Nil

#include "../rest_totvs/armazens_repo.tlpp"
#include "../rest_totvs/armazens_api.tlpp"
