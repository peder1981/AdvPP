/*/{Protheus.doc} monitor.prw
App de exemplo: o monitor do sistema de examples/poui/fase2.prw, servido
como app de `advplc serve --apps`.
@type file
@author Peder Munksgaard
@since 20/11/2026
/*/

User Function Main()
	Local oApp  := PoApp():New("AdvPP — componentes PO-UI")
	Local oMenu := PoMenu():New(oApp)
	Local oSis  := PoPage():New(oApp, "Sistema")
	Local oCmp  := PoPage():New(oApp, "Componentes")
	Local oWMem := PoWidget():New(oSis, "Memória")
	Local oMem  := PoGauge():New(oWMem, "Em uso")
	Local oWCpu := PoWidget():New(oSis, "Carga do sistema")
	Local oCh   := PoChart():New(oWCpu, "Load average (1 min)", "line")
	Local oWDir := PoWidget():New(oSis, "Diretório atual")
	Local oBtn  := PoButton():New(oWDir, "Listar diretório")
	Local oLs   := PoList():New(oWDir)

	oWMem:SetCols(6)
	oWCpu:SetCols(6)
	oCh:SetSeries({{"carga", {}}})
	oCh:SetMaxPoints(30)
	oBtn:OnClick({|| Listar(oApp, oLs)})

	MontaComponentes(oApp, oCmp)

	oMenu:AddItem("Sistema", "an an-gauge", {|| oApp:Navigate(oSis)})
	oMenu:AddItem("Componentes", "an an-squares-four", {|| oApp:Navigate(oCmp)})

	Atualiza(oMem, oCh)
	oApp:SetTimer(1000, {|| Atualiza(oMem, oCh)})
	oApp:Notify("Bem-vindo! Use o menu à esquerda.", "success")
	oApp:Activate()
Return

/*/{Protheus.doc} MontaComponentes
Página com abas: campos tipados, árvore e indicadores.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function MontaComponentes(oApp, oCmp)
	Local oTabs  := PoTabs():New(oCmp)
	Local oTCamp := PoTab():New(oTabs, "Campos")
	Local oTArv  := PoTab():New(oTabs, "Árvore")
	Local oTInd  := PoTab():New(oTabs, "Indicadores")
	Local oSenha := PoPassword():New(oTCamp, "Senha")
	Local oQtd   := PoNumber():New(oTCamp, "Quantidade", 1)
	Local oCor   := PoSelect():New(oTCamp, "Cor", {"Azul", "Verde", {"Vermelho", "vm"}}, "Azul")
	Local oAceit := PoCheckbox():New(oTCamp, "Aceito os termos")
	Local oWifi  := PoSwitch():New(oTCamp, "Wi-Fi", .T.)
	Local oObs   := PoTextarea():New(oTCamp, "Observação")
	Local oMostr := PoButton():New(oTCamp, "Mostrar valores")
	Local oArv   := PoTree():New(oTArv)
	Local oInfo  := PoInfo():New(oTInd, "Compilador", "AdvPP")
	Local oProg  := PoProgress():New(oTInd, "Progresso")
	Local oMais  := PoButton():New(oTInd, "+10%")

	oQtd:SetCols(4)
	oCor:SetCols(4)
	oSenha:SetCols(4)
	oMostr:OnClick({|| oApp:Notify("Qtd " + cValToChar(oQtd:GetValue()) + ", cor " + oCor:GetValue() + ;
		", Wi-Fi " + cValToChar(oWifi:GetValue()) + ", termos " + cValToChar(oAceit:GetValue()), "information")})
	oWifi:OnChange({|o, l| oApp:Notify("Wi-Fi " + LigaDesliga(l), "warning")})

	oArv:SetItems({{"Projetos", "proj", {{"AdvPP", "advpp"}, {"AdvOS", "advos"}}}, {"Documentos", "docs"}})
	oArv:OnSelect({|o, c| oApp:Notify("Selecionado: " + c)})

	oMais:OnClick({|| oProg:SetValue(oProg:GetValue() + 10)})
Return

/*/{Protheus.doc} Listar
Roda `ls -la` sem shell e mostra a saída na lista ao vivo.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Listar(oApp, oLs)
	Local aItens := {} as Array

	oApp:Loading(.T., "Listando...")
	oApp:Spawn("ls", {"-la"}, ;
		{|c, lErro| aAdd(aItens, {c, Origem(lErro)}), oLs:SetItems(aItens)}, ;
		{|n| oApp:Loading(.F.), oApp:Notify("processo terminou: " + cValToChar(n), TipoFim(n))})
Return

/*/{Protheus.doc} Atualiza
Memória em uso (gauge) e carga (gráfico), lidas de /proc.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Atualiza(oMem, oCh)
	Local cMem   := MemoRead("/proc/meminfo") as Character
	Local nTotal := ValorMem(cMem, "MemTotal:") as Numeric
	Local nLivre := ValorMem(cMem, "MemAvailable:") as Numeric
	Local cLoad  := MemoRead("/proc/loadavg") as Character

	If nTotal > 0
		oMem:SetValue(Round((nTotal - nLivre) / nTotal * 100, 1))
		oMem:SetDescription(cValToChar(Round((nTotal - nLivre) / 1048576, 1)) + " de " + cValToChar(Round(nTotal / 1048576, 1)) + " GB")
	EndIf
	oCh:AddPoint(Time(), {Val(cLoad)})
Return

/*/{Protheus.doc} ValorMem
Valor em kB de uma chave de /proc/meminfo.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function ValorMem(cMem, cChave)
	Local nPos := At(cChave, cMem) as Numeric

	If nPos == 0
		Return 0
	EndIf
Return Val(SubStr(cMem, nPos + Len(cChave), 20))

/*/{Protheus.doc} LigaDesliga
Texto do estado de um switch.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function LigaDesliga(lLigado)
	Local cRet := "desligado" as Character

	If lLigado
		cRet := "ligado"
	EndIf
Return cRet

/*/{Protheus.doc} Origem
Detalhe da linha: vazia para stdout, "erro" para stderr.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function Origem(lErro)
	Local cRet := "" as Character

	If lErro
		cRet := "erro"
	EndIf
Return cRet

/*/{Protheus.doc} TipoFim
Tipo da notificação conforme o código de saída.
@type function
@author Peder Munksgaard
@since 20/11/2026
/*/
Static Function TipoFim(nCodigo)
	Local cRet := "error" as Character

	If nCodigo == 0
		cRet := "success"
	EndIf
Return cRet
