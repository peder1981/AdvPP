#Include "PROTHEUS.CH"
#Include "ADMXFUN.CH"

STATIC lAs400
STATIC __lReport
STATIC cAdmArq

STATIC lCTBDCVLD := .F.
Static lFWCodFil := FindFunction("FWCodFil")

// 09/03/10 - Comentario incluido para gerar pacote (so amarracao nao gera pacote)
/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÃÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³         ATUALIZACOES SOFRIDAS DESDE A CONSTRU€AO INICIAL.             ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Programador ³Data    ³ BOPS     ³ Motivo da Alteracao                  ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³M.Camargo   ³01/03/17³MMI-4130  |Se abre punto de entrada ADMSELFIL en ³±±
±±³            ³        ³          ³la función ADMGETFIL                  ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³ SelDados   ³ Autor ³ Wagner Mobile Costa   ³ Data ³ 18/03/02 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³Monta Query (RECNO) ou IndRegua de acordo com ambiente e monta³±±
±±³          ³Regua de Progresso                                            ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³ cAlias   - > Alias para montagem da selecao                  ³±±
±±³          ³ cFiltro  - > Condicao de selecao dos dados                   ³±±
±±³          ³ lReport  - > Indica se chamada a partir de relatorio         ³±±
±±³          ³ cAliCmpQ - > Alias complementares na query para uso WHERE    ³±±
±±³          ³ aCpoSelQ - > Campos complementares/selecionados na Query     ³±±
±±³          ³ cFilTop  - > Condicao de selecao dos dados somente em TOP    ³±±
±±³          ³ cFilCod  - > Condicao de selecao dos dados somente CodeBase  ³±±
±±³          ³ cOrdem   - > Chave de indice utilizada para ordenar a tabela ³±±
±±³          ³             ou Query (Order BY)                              ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Retorno   ³ cAlias  - > Alias para montagem da selecao                   ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                     ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function SelDados(cAlias, cFiltro, lReport, cAliCmpQ, aCpoSelQ, cFilTop, cFilCod, cOrdem)

Local cIndex
Local cAliasQry
#IFDEF TOP
	Local nPosAt, cCpoDtos
	Local cAliasCmpQ := ""
	Local cCpoSelQ, nCpoSelQ
	Local nOrdSx3 := SX3->(IndexOrd())
	Local aCposTbl := {}
	Local nCpoTbl  := 2
#ENDIF

DEFAULT cAlias := Alias()
DEFAULT cOrdem := (cAlias)->(IndexKey())
DEFAULT lReport:= .T.
DEFAULT cFilTop:= ""
DEFAULT cFilCod:= ""

cAliasQry := cAlias

#IFDEF TOP
	If lAs400 = Nil
		lAs400 := TcSrvType() = "AS/400"
	Endif
#ELSE
	lAs400 := .F.
#ENDIF

__lReport := lReport
If ValType("__lReport") = "L"
	If lReport
		SetRegua( (cAlias)->( RecCount() ) )
	Else
		ProcRegua( (cAlias)->( RecCount() ) )
	Endif
Endif

#IFDEF TOP
	If ! lAs400
		If cAliCmpQ <> Nil
			While Len(cAliCmpQ) > 0
				cAliasCmpQ 	+= ", " + RetSqlName(Left(cAliCmpQ, 3)) + " " + Left(cAliCmpQ, 3)
				cAliCmpQ	:= Subs(cAliCmpQ, 4)
			EndDo
		Else
			cAliasCmpQ := ""
		Endif

		cFiltro := StrTran(UPPER(cFiltro), ".OR.", " OR ")
		cFiltro := StrTran(UPPER(cFiltro), ".AND.", " AND ")
		cFiltro := StrTran(cFiltro, "==", "=")

		While At("DTOS", Upper(cFiltro)) > 0
			nPosAt 		:= At("DTOS(", Upper(cFiltro))
			cCpoDtos    := Subs(cFiltro, nPosAt + 5, Len(cFiltro))
			cCpoDtos	:= Left(cCpoDtos, At(")", cCpoDtos) - 1)
			cFiltro 	:= 	StrTran(Upper(cFiltro), "DTOS(" + cCpoDtos + ")", cCpoDtos)
		EndDo

		If ! Empty(cFilTop) .And. Right(AllTrim(cFilTop), 3) <> "AND"
			cFilTop := " AND " + cFilTop
		Endif
		cFilTop += " AND " + cAlias + ".D_E_L_E_T_ = ' '"
		dbSelectArea(cAlias)
		cAliasQry := cAlias + "QRY"
		If aCpoSelQ = Nil
			aCpoSelQ := {}
		Endif
		If Len(aCpoSelQ) > 0 .And. ValType(aCpoSelQ[1]) <> "A" .And. aCpoSelQ[1] = "*"
			cCpoSelQ := "*"
		ElseIf Len(aCpoSelQ) > 0 .And. ValType(aCpoSelQ[1]) = "A"
			cCpoSelQ := ""
			For nCpoSelQ := 1 To Len(aCpoSelQ)
				If Len(aCpoSelq[nCpoSelQ]) > 4
					cCpoSelQ += aCpoSelq[nCpoSelQ][5] + " " +;
								aCpoSelq[nCpoSelQ][1] + If(nCpoSelQ <> Len(aCpoSelQ), ", ", "")
				Else
					cCpoSelQ += aCpoSelq[nCpoSelQ] + If(nCpoSelQ <> Len(aCpoSelQ), ", ", "")
				Endif
			Next
		Else
			cCpoSelQ := ""
			For nCpoSelQ := 1 To Len(aCpoSelQ)
				cCpoSelQ += aCpoSelq[nCpoSelQ] + If(nCpoSelQ <> Len(aCpoSelQ), ", ", "")
			Next
		Endif

        If ! "SUM" $ cCpoSelQ .And. ! "MAX" $ cCpoSelQ .And. ! "R_E_C_N_O_" $ cCpoSelQ
			_cGetDB := TcGetDb()
			If cCpoSelQ == "*" .and. ( Upper(_cGetDB) == "ORACLE" .Or. Upper(_cGetDB) == "DB2") /// ORACLE NAO ACEITA SELECT *,campo
				aCposTbl := (cAlias)->(dbStruct())
				If Len(aCposTbl) > 0								/// SE NAO RETORNOU NADA NA ESTRUTURA NAO MEXE NOS CAMPOS
					cCpoSelQ := aCposTbl[1,1]						/// PRIMEIRO CAMPO
					If Len(aCposTbl) > 1							/// SE HA MAIS DE UM CAMPO
						For nCpoTbl := 2 to Len(aCposTbl)			/// ADICIONA OS DEMAIS CAMPOS
							cCpoSelQ += ","+aCposTbl[nCpoTbl,1]		/// DO 2º CAMPO EM DIANTE (SEPARADO POR VIRGULA)
						Next
					Endif
				Endif
			Endif
			cCpoSelQ += If(Empty(cCpoSelQ), "", ", ") + cAlias + ".R_E_C_N_O_ RECNO"
		Endif

		cFiltro := 	"SELECT " + cCpoSelQ + " FROM " +;
					RetSqlName(cAlias) + " " + cAlias + cAliasCmpQ + " WHERE " +;
					cFiltro + cFilTop
        If ! "SUM" $ cCpoSelQ .And. ! "MAX" $ cCpoSelQ
			cFiltro += 	" ORDER BY "+ SqlOrder(cOrdem) + If(Empty(cAliasCmpQ), ",R_E_C_N_O_", "")
		Endif
		cFiltro := ChangeQuery(cFiltro)

		If Upper(TcGetDb()) $ "ORACLE,POSTGRES,DB2,INFORMIX"		// Sinal de concatencao nesses ambientes
			cFiltro := StrTran(cFiltro, "+", "||")
		Endif
		dbUseArea(.T., "TOPCONN", TCGenQry(,,cFiltro), cAliasQry, .F., .T.)
		SX3->(DbSetOrder(2))
		aCposTbl := (cAlias)->(dbStruct())
		For nCpoSelQ := 1 To Len(aCposTbl)
			// Avalia a estrutura do arquivo a ser aberto. Se o campo estiver na query e for diferente
			// de caracter, altera o atributo do campo.
			If aCposTbl[nCpoSelQ][2] <> "C" .And. (cAliasQry)->(FieldPos(aCposTbl[nCpoSelQ][1])) > 0
				TCSetField(cAliasQry, 	aCposTbl[nCpoSelQ][1], aCposTbl[nCpoSelQ][2],aCposTbl[nCpoSelQ][3], aCposTbl[nCpoSelQ][4])
			Endif
		Next
		SX3->(DbSetOrder(nOrdSx3))
	Else
#ENDIF

dbSelectArea(cAlias)
#IFDEF AXS
If InTransact()
	Set Filter To &(cFiltro+cFilCod)
	DbGoTop()
Else
#ENDIF
	cIndex := CriaTrab(nil,.f.)
	IndRegua(cAlias,cIndex,cOrdem,,cFiltro+cFilCod,STR0001)
	nIndex := RetIndex(cAlias)
	#IFNDEF TOP
	dbSetIndex(cIndex+OrdBagExt())
	#Endif
	dbSetOrder(nIndex+1)
	dbSelectArea(cAlias)
	dbGoTop()

#IFDEF AXS
Endif
#ENDIF

#IFDEF TOP
	Endif
#Endif

Return cAliasQry

/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³ CarregaSel ³ Autor ³ Wagner Mobile Costa   ³ Data ³ 21/03/02 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³Posiciona Registro de Query de RECNO de acordo com ambiente   ³±±
±±³          ³e atualiza regra de progresso                                 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³ cAlias    - > Alias para montagem da selecao                 ³±±
±±³          ³ bCondicao - > Condicao de avaliacao se registro valido       ³±±
±±³          ³               Somente CodeBase ou AS400                      ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                     ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function CarregaSel(cAlias, bCondicao)

If lAs400 = Nil
	#IFDEF TOP
		lAs400 := TcSrvType() = "AS/400"
	#ELSE
		lAs400 := .F.
	#Endif
Endif

If ValType("__lReport") = "L"
	If __lReport
		IncRegua()
	Else
		IncProc()
	Endif
Endif

#IFDEF TOP

If ! lAS400
	DbSelectArea(cAlias)
	If Recno() <> (cAlias + "Qry")->RECNO
		If (cAlias + "Qry")->RECNO = 0
			DbGoBottom()
			DbSkip()
		Else
			DbGoto((cAlias + "Qry")->RECNO)
		Endif
	Endif
Else

#ENDIF

	DbSelectArea(cAlias)
	If bCondicao <> Nil .And. ! Eval(bCondicao)
		DbSkip()
		Return .F.
	Endif

#IFDEF TOP
Endif
#ENDIF

Return .T.

/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³ RemoveSel  ³ Autor ³ Wagner Mobile Costa   ³ Data ³ 18/03/02 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³Monta Query ou IndRegua de acordo com ambiente utilizado      ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Utilizacao³ Cposusuario(aAlter,cAlias)                                   ³±±
±±³Parametros³ aAlter  - > Campos que podem ser alterados                   ³±±
±±³          ³ cAlias  - > Arquivo a ser lido                               ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                     ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function RemoveSel(cAlias)

#IfDef TOP
	If TcSrvType() != "AS/400"
		(cAlias + "QRY")->(DbCloseArea())
		dbSelectArea(cAlias)
		dbSetOrder(1)
	Else
#Endif

#IFDEF AXS
If InTransact()
	DbSelectArea(cAlias)
	Set Filter To
	DbSetOrder(1)
Else
#ENDIF

RetIndex(cAlias)
DbSetOrder(1)

#IFDEF AXS
Endif
#ENDIF

#IfDef TOP
	Endif
#Endif

Return .T.

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡„o    ³AdmModelo ³ Autor ³ Wagner Mobile Costa   ³ Data ³ 20/06/01 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡„o ³ Funcao para visualizacao em formato Modelo 1/2/3           ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Sintaxe   ³ AdmModelo(ExpC1,ExpN1,ExpN2)                               ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³ ExpC1 = Alias do arquivo                                   ³±±
±±³          ³ ExpN1 = Numero do registro                                 ³±±
±±³          ³ ExpN2 = Numero da opcao selecionada                        ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                   ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/

Function AdmModelo(cAlias,nReg,nOpc,lAutomato)
Local lPanelFin := If (FindFunction("IsPanelFin"),IsPanelFin(),.F.)
Local nOpca := 0, nCnt := 0, nSavReg
Local oDlg, oGetD, cContador, lModelo2 := .F., nCpoTela
Local nOrdSx3 	:= Sx3->(IndexOrd())
Local cCpoMod2  := ""
Local lOk
Local nHeader	:= 1
Local nSaveSx8Len := GetSx8Len()
Local aTam		:= {}
Local aObjeto 	:= {}
Local aTamAut 	:= {}
Local aInfo		:= {}
Local aCabAuto	:= {}

Private Inclui	:= .F.
Private Altera	:= .F.
Private Exclui	:= .F.
Private Visual	:= .F.

DEFAULT lAutomato := .F.


If !(lAutomato)
	If Type('cChave') = "U" .Or. cChave = Nil
		cChave  := StrTran((cAlias)->(IndexKey(1)), " ", "")
		cChave  := Subs(cChave, At("_FILIAL+", cChave) + 8)
	Endif

	cAlias1  := If(Type("cAlias1") = "U" .Or. cAlias1 = Nil, cAlias, cAlias1)
	lModelo1 := If(cAlias1 = cAlias .And. Type("aPosTela") = "U", .T., .F.)
	lModelo2 := If(cAlias1 = cAlias, .T., .F.)

	If At("_", cChave) = 3		// Campos com duas finais do alias (C5_, N1_ ...)
		cCpoIni1 := Right(cAlias, 2)
		cCpoIni2 := Right(cAlias1, 2)
	Else
		cCpoIni1 := cAlias
		cCpoIni2 := cAlias1
	Endif

	If Type("aSize") = "U" .Or. aSize = Nil
		If (Type("lUsaBes") <> "L" .Or. ! lUsaBes) .And. lModelo1
			If lPanelFin
				oPanelDados := FinWindow:GetVisPanel()
				oPanelDados:FreeChildren()
				aDim := DLGinPANEL(oPanelDados)
				aPosEnch := {,,,}
				DEFINE MSDIALOG oDlg OF oPanelDados:oWnd FROM 0, 0 TO 0, 0 PIXEL STYLE nOR( WS_VISIBLE, WS_POPUP )
		   Else
				If SetMDIChild()
					oMainWnd:ReadClientCoors()
					DEFINE MSDIALOG oDlg TITLE cCadastro FROM 40,30 TO oMainWnd:nBottom-80,oMainWnd:nRight-70 PIXEL OF oMainWnd
				Else
					DEFINE MSDIALOG oDlg TITLE cCadastro FROM 9,0 TO TranslateBottom(.F.,28),80 OF oMainWnd
				EndIf
	      Endif

			aTam			:= MsAdvSize(,.F.,430)
			aTamAut 		:= MsAdvSize()

			AAdd( aObjeto, { 100, 100, .T., .T. } )
			aInfo := { aTamAut[ 1 ], aTamAut[ 2 ], aTamAut[ 3 ], aTamAut[ 4 ], 3, 3 }

			aPosObj := MsObjSize( aInfo, aObjeto, lModelo2 )

		Else
			Private aSize		:= MsAdvSize(,.F.,430)
			Private aObjects 	:= {}
			Private aPosObj  	:= {}
			Private aSizeAut 	:= MsAdvSize()

			If lModelo1
				AAdd( aObjects, { 315, 20, .T., .T. } )
			ElseIf lModelo2
				AAdd( aObjects, { 315, aPosTela[1][2] + 20, .T., .T. } )
				AAdd( aObjects, { 100, 430 - aPosTela[1][2] - 20, .T., .T. } )
			Else
				AAdd( aObjects, { 100, 100, .T., .T. } )
				AAdd( aObjects, { 315,  70, .T., .T. } )
			Endif

			aInfo := { aSizeAut[ 1 ], aSizeAut[ 2 ], aSizeAut[ 3 ], aSizeAut[ 4 ], 3, 3 }

			aPosObj := MsObjSize( aInfo, aObjects, lModelo2 )
		Endif
	Endif


	Do Case
		Case nOpc = 2
			Visual 	:= .T.
		Case nOpc = 3
			Inclui 	:= .T.
			Altera 	:= .F.
		Case nOpc = 4
			Inclui 	:= .F.
			Altera 	:= .T.
		Case nOpc = 5
			Exclui	:= .T.
			Visual	:= .T.
	EndCase

	//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	//³ Salva a integridade dos campos de Bancos de Dados    ³
	//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
	dbSelectArea(cAlias)

	IF ! INCLUI .And. LastRec() == 0
		Help(" ",1,"A000FI")
		Return (.T.)
	Endif
	//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	//³ Verifica se esta' na filial correta                          ³
	//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
	If ! INCLUI .And. xFilial(cAlias) != &(cCpoIni1 + "_FILIAL")
		Help(" ",1,"A000FI")
		Return
	Endif

	//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	//³ Monta a entrada de dados do arquivo                  ³
	//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
	PRIVATE aTELA[0][0],aGETS[0],aHeader[0]
	PRIVATE nUsado:=0,lTab   := .F.

	If lModelo1 .Or. ! lModelo2
		//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		//³ Salva a integridade dos campos de Bancos de Dados    ³
		//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
		If ALTERA .Or. EXCLUI
			SoftLock(cAlias)
		Endif

		AdmMemory(cAlias)
	ElseIf Type("aPosTela") <> "U"
		Private aChaves	:= {}
		SX3->(DbSetOrder(2))
		For nCpoTela := 1 to Len(aPosTela)
			cCpoMod2 += aPosTela[nCpoTela][1] + ";"
			Aadd(aChaves, { aPosTela[nCpoTela][1], "M->" + aPosTela[nCpoTela][1] })

	// Bloco CdaMemory

			SX3->(DbSeek(aPosTela[nCpoTela][1]))

			If SX3->X3_CONTEXT = "V" 	// Campo virtual
				If ! Empty(SX3->X3_INIBRW)
					_SetOwnerPrvt(Trim(SX3->X3_CAMPO), &(AllTrim(SX3->X3_INIBRW)))
				Else
					_SetOwnerPrvt(Trim(SX3->X3_CAMPO), CriaVar(Trim(SX3->X3_CAMPO)))
				Endif
			Else
				If INCLUI
					_SetOwnerPrvt(Trim(SX3->X3_CAMPO), CriaVar(Trim(SX3->X3_CAMPO)))
				Else
					_SetOwnerPrvt(Trim(SX3->X3_CAMPO), (cAlias)->&(Trim(SX3->X3_CAMPO)))
				EndIf
			Endif
		Next
	Endif

	//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	//³ Monta cabecalho.                                     ³
	//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ

	If ! lModelo1
		dbSelectArea("SX3")
		dbSetOrder(1)
		MsSeek( cAlias1 )
		While !Eof() .And. x3_arquivo == cAlias1
			IF X3USO(x3_usado) .And. cNivel >= x3_nivel .And. ! X3_CAMPO $ cCpoMod2
				nUsado++
				Aadd(aHeader,{ TRIM(x3_titulo), x3_campo, x3_picture,;
				x3_tamanho, x3_decimal, x3_valid, x3_usado, x3_tipo, x3_arquivo,;
				x3_context, x3_nivel, x3_relacao, Trim(x3_inibrw) } )
			Endif
			dbSkip()
		End
	Endif

	If ! lModelo1 .And. (Type('cChave1') = "U" .Or. cChave1 = Nil)
		If lModelo2
			cChave1 := cChave
		Else
			If At("_", cChave) = 3		// Campos com duas finais do alias (C5_, N1_ ...)
				cChave1 := StrTran(cChave, Right(cAlias, 2), Right(cAlias1, 2))
			Else
				cChave1 := StrTran(cChave, cAlias, cAlias1)
			Endif
		Endif
	Endif

	If ! lModelo1 .And. ! lModelo2 .And. Len(cChave1) # Len(cChave)
		cChave := Left(cChave, Len(cChave1))
	Endif

	If ! lModelo1
		dbSelectArea(cAlias1)
		dbSetOrder(1)
		MsSeek(xFilial(cAlias1) + &(cChave))
		nSavReg := Recno()

		nCnt := 0
		While ! INCLUI .And. !Eof() .And. 	xFilial(cAlias) + &(cChave) ==;
											xFilial(cAlias1) + &(cChave1)
			nCnt++
			dbSkip()
		End

		If ! INCLUI .And. ((Type("lSemItens") = "U" 	.Or.;
									  lSemItens = Nil) 	.Or. ! lSemItens)	// Indica se verifica existencia dos
			If nCnt == 0                               						// itens
				Help(" ",1,"CA10SEMREG")
				Return .T.
			Endif
		Endif

		nCnt := If(nCnt = 0, 1, nCnt)

		PRIVATE aCOLS[nCnt][nUsado + 1]

		dbSelectArea(cAlias1)
		dbSetOrder(1)
		dbGoTo(nSavReg)
		nCnt := 0

		While 	! INCLUI .And. ! Eof() .And.;
				xFilial(cAlias) + &(cChave1) == xFilial(cAlias1) + &(cChave)

			nUsado 	:= 0
			nCnt++

			For nHeader := 1 To Len(aHeader)
				If X3USO(aHeader[nHeader][7]) .And. cNivel >= aHeader[nHeader][11]
					nUsado++
					If aHeader[nHeader][10] = "V"				// Campo virtual
						If ! Empty(aHeader[nHeader][13])		// inicializador BROWSE
							aCOLS[nCnt][nUsado] := &(aHeader[nHeader][13])
						Endif
					ElseIf INCLUI
						aCOLS[nCnt][nUsado] := CriaVar(AllTrim(aHeader[nHeader][2]))
						If "_ITEM" $ aHeader[nHeader][2]
							aCOLS[nCnt][nUsado] := StrZero(nCnt, Len(aCOLS[nCnt][nUsado]))
						Endif
					Else
						aCOLS[nCnt][nUsado] := &(cAlias1 + "->" + aHeader[nHeader][2])
					Endif
				Endif
			Next
			aCOLS[nCnt][nUsado + 1] := .F.

			If ALTERA .Or. EXCLUI
				SoftLock(cAlias1)
			Endif

			DbSkip()
		Enddo

		If nCnt = 0
			nCnt ++
			nUsado := 0
			For nHeader := 1 To Len(aHeader)
				If X3USO(aHeader[nHeader][7]) .And. cNivel >= aHeader[nHeader][11]
					nUsado++
					If aHeader[nHeader][10] = "V"				// Campo virtual
						If ! Empty(aHeader[nHeader][13])		// inicializador BROWSE
							aCOLS[nCnt][nUsado] := &(aHeader[nHeader][13])
						Endif
					ElseIf INCLUI
						aCOLS[nCnt][nUsado] := CriaVar(AllTrim(aHeader[nHeader][2]))
						If "_ITEM" $ aHeader[nHeader][2]
							aCOLS[nCnt][nUsado] := StrZero(nCnt, Len(aCOLS[nCnt][nUsado]))
						Endif
					Else
						aCOLS[nCnt][nUsado] := &(cAlias1 + "->" + aHeader[nHeader][2])
					Endif
				Endif
			Next
			aCOLS[nCnt][nUsado + 1] := .F.
		Endif

		dbSelectArea(cAlias1)

		If FieldPos(cAlias1 + "_ITEM") > 0
			cContador := "+" + cAlias1 + "_ITEM"
		Endif

		dbSetOrder(1)
		dbGoTo(nSavReg)
	Endif

	If Type("cLinhaOk") = "U" .Or. cLinhaOk = Nil
		cLinhaOk := "AllwaysTrue"
	Endif

	If Type("cTudoOk") = "U" .Or. cTudoOk = Nil
		cTudoOk := "AllwaysTrue"
	Endif
	If Type("aButbar") != "A"
		aButBar := {}
	Endif
	If Type("cFunc") <> "U" .And. cFunc <> Nil		// Funcao para inicializar variaveis de
		&cFunc.()									// Memoria
	Endif

	If oDlg = Nil
		DEFINE MSDIALOG oDlg TITLE cCadastro From aSize[7],0 to aSize[6],aSize[5] of oMainWnd PIXEL
	Endif

	If lPanelFin  //Chamado pelo Painel Financeiro
		dbSelectArea(cAlias)
		If nOpc == 3
			RegToMemory(cAlias,.T.,,,FunName())
		ElseIf nOpc == 4
			RegToMemory(cAlias,.F.,.F.,,FunName())
		Else
			RegToMemory(cAlias,.F.,.F.)
		Endif

		If Type("aAcho") = "A"
			oEnc01:= MsMGet():New(cAlias,nReg,nOpc,,"AC",STR0008,aAcho,aPosEnch,,,,,,oDlg,,,.F.)
		Else
			oEnc01:= MsMGet():New(cAlias,nReg,nOpc,,"AC",STR0008,,aPosEnch,,,,,,oDlg,,,.F.)
		Endif
		oEnc01:oBox:Align := CONTROL_ALIGN_ALLCLIENT

	Else
		If lModelo1 .Or. ! lModelo2
			If Type("aAcho") = "A"
				EnChoice( cAlias, nReg, nOpc, , , , aAcho, aPosObj[1], , 3, , , , , , .T. )
			Else
				EnChoice( cAlias, nReg, nOpc, , , , , aPosObj[1], , 3, , , , , , .T. )
			Endif
		ElseIf Type("aPosTela") <> "U"

			SX3->(DbSetOrder(2))
			For nCpoTela 	:= 1 to Len(aPosTela)
				cCampo		:= aPosTela[nCpoTela][1]
				SX3->(DbSeek(cCampo))
				nX			:= aPosTela[nCpoTela][2]
				nY			:= aPosTela[nCpoTela][3]
				cCaption	:= X3Titulo()
				cPict		:= If(Empty(SX3->X3_PICTURE),Nil,SX3->X3_PICTURE)
				cValid		:= If(Empty(SX3->X3_VALID),".t.",SX3->X3_VALID)
				cF3			:= If(Empty(SX3->X3_F3),NIL,SX3->X3_F3)
				cWhen		:= If(Empty(SX3->X3_WHEN),"(.t.)","(" +;
								 AllTrim(SX3->X3_WHEN) + ")")
				If Len(aPosTela[nCpoTela]) > 3
					cWhen += " .And. (" + aPosTela[nCpoTela][4] + ")"
				Endif
				cBlKSay 	:= "{|| OemToAnsi('"+cCaption+"')}"
				oSay 		:= TSay():New( nX + 1, nY, &cBlkSay,oDlg,,, .F., .F., .F., .T.,,,,, .F., .F., .F., .F., .F. )
				nLargSay 	:= GetTextWidth(0,cCaption) / 1.8  // estava 2.2
				cCaption 	:= oSay:cCaption
				cBlkGet 	:= "{ | u | If( PCount() == 0, M->"+cCampo+", M->"+cCampo+":= u ) }"
				cBlKVld 	:= "{|| "+cValid+"}"
				cBlKWhen 	:= "{|| "+cWhen+"}"
				oGet 		:= TGet():New( nX, nY+nLargSay,&cBlKGet,oDlg,,,cPict, &(cBlkVld),,,, .F.,, .T.,, .F., &(cBlkWhen), .F., .F.,, .F., .F. ,cF3,(cCampo))
			Next

			Sx3->(DbSetOrder(nOrdSx3))
		Endif
	Endif

	If ! lModelo1
		oGetd := MSGetDados():New(aPosObj[2,1],aPosObj[2,2],aPosObj[2,3],aPosObj[2,4],nOpc,cLinhaOk,cTudoOk,cContador, .T.)
		lTab  := .T.
	Endif
	lOk := (Exclui .Or. !lModelo1)


	If lPanelFin  //Chamado pelo Painel Financeiro
		// define dimenção da dialog
		oDlg:nWidth := aDim[4]-aDim[2]
		ACTIVATE MSDIALOG oDlg ON INIT ( FaMyBar(oDlg,{||nOpca:=1,if(lOk .Or. (Obrigatorio(aGets,aTela) .And. &cTudoOk.()),oDlg:End(),nOpca := 0) }, { || oDlg:End() },aButBar),oDlg:Move(aDim[1],aDim[2],aDim[4]-aDim[2], aDim[3]-aDim[1]))
	Else
		ACTIVATE MSDIALOG oDlg ON INIT EnchoiceBar(oDlg,{||nOpca:=1,if(lOk .Or. (Obrigatorio(aGets,aTela) .And. &cTudoOk.()),oDlg:End(),nOpca := 0) }, { || oDlg:End() },,aButBar)
	Endif

	If nOpca = 1 .And. nOpc # 2
		BEGIN TRANSACTION

			If nOpc = 5 .And. (Type("cPodeExcluir") = "U" .Or. Empty(cPodeExcluir) .Or.;
																	&(cPodeExcluir))
				AdmDelMod(cAlias, cAlias1, lModelo1)
			ElseIf nOpc # 5
				AdmGrvMod(cAlias, cAlias1, lModelo1)
				While (GetSx8Len() > nSaveSx8Len)
					ConfirmSX8()
				End
			Endif

		END TRANSACTION
	Else
		While GetSx8Len() > nSaveSx8Len
			RollBackSX8()
		End
	Endif

	dbSelectArea(cAlias)

	If lPanelFin  //Chamado pelo Painel Financeiro
		dbSelectArea(FinWindow:cAliasFile)
		FinVisual(FinWindow:cAliasFile,FinWindow,(FinWindow:cAliasFile)->(Recno()),.T.)
	Endif
Else
	If isincallstack("FINA191")
		If FindFunction("GetParAuto")
			aCabAuto:= GetParAuto("FINA191TestCase")
		EndIf

		//Criando as variaveis de memoria para evitar erro log na validação
		If nOpc = 4
			RegToMemory(cAlias,.F.,.F.)
		Else
			RegToMemory(cAlias,.T.,.F.)
		EndIf

		nOpca := if(EnchAuto(cAlias,aCabAuto,cTudoOk,nOpc),1,0)

		If nOpca == 1
			If !nOpc = 5
				If nOpc = 3
					reclock("SEF",.T.)
				Else
					reclock("SEF",.F.)
				EndIf

				SEF->EF_FILIAL	:= Xfilial("SEF")
				SEF->EF_CLIENTE := M->EF_CLIENTE
				SEF->EF_LOJACLI	:= M->EF_LOJACLI
				SEF->EF_CPFCNPJ	:= M->EF_CPFCNPJ
				SEF->EF_BANCO	:= M->EF_BANCO
				SEF->EF_AGENCIA	:= M->EF_AGENCIA
				SEF->EF_CONTA	:= M->EF_CONTA
				SEF->EF_NUM		:= M->EF_NUM
				SEF->EF_VALOR	:= M->EF_VALOR
				SEF->EF_VALORBX	:= M->EF_VALORBX
				SEF->EF_DATA	:= M->EF_DATA
				SEF->EF_VENCTO	:= M->EF_VENCTO
				SEF->EF_PREFIXO	:= M->EF_PREFIXO
				SEF->EF_TITULO	:= M->EF_TITULO
				SEF->EF_PARCELA	:= M->EF_PARCELA
				SEF->EF_TIPO	:= M->EF_TIPO
				SEF->EF_EMITENT	:= M->EF_EMITENT
				SEF->EF_ALINEA1	:= M->EF_ALINEA1
				SEF->EF_DTALIN1	:= M->EF_DTALIN1	
				SEF->EF_ALINEA2	:= M->EF_ALINEA2
				SEF->EF_DTALIN2	:= M->EF_DTALIN2							

				//Função usada para complementar os valores
				If Type("cCompCab") <> "U" .And. cCompCab <> Nil
					&cCompCab.()
				Endif

				MsUnLock()
			Else
				reclock("SEF",.F.)

				dbdelete()

				MsUnLock()
			EndIf
		EndIF
	EndIf
EndIF

Return nOpca

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡„o    ³AdmGrvMod ³ Autor ³ Wagner Mobile Costa   ³ Data ³ 21/08/01 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡„o ³ Funcao para gravacao em formato Modelo 1/2/3           	  ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Sintaxe   ³ AdmGrvMod(cPar1, cPar2)                                    ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³ ExpC1 = Alias do arquivo                                   ³±±
±±³          ³ ExpC1 = Alias detalhe do arquivo                           ³±±
±±³          ³ ExpL1 = Identifica se eh tipo MODELO 1 (Sem aCols)         ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                   ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/

Function AdmGrvMod(cAlias,cAlias1,lModelo1)

Local nCampos, nHeader, nMaxArray, bCampo, aAnterior:={}, nItem := 1
Local lModelo2 := cAlias = cAlias1
Local nChaves

BEGIN TRANSACTION

bCampo := {|nCPO| Field(nCPO) }

//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
//³ verifica se o ultimo elemento do array esta em branco ³
//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ

If lModelo1 .Or. ! lModelo2
	//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	//³ Grava arquivo PRINCIPAL ³
	//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
	dbSelectArea(cAlias)

	RecLock(cAlias, If(INCLUI,.T.,.F.))

	For nCampos := 1 TO FCount()
		If "FILIAL"$Field(nCampos)
			FieldPut(nCampos,xFilial(cAlias))
		Else
			FieldPut(nCampos,M->&(EVAL(bCampo,nCampos)))
		EndIf
	Next

	If Type("cCompCab") <> "U" .And. cCompCab <> Nil	// Funcao para complementar
		&cCompCab.()									// gravacao no cabecalho
	Endif
Endif

If ! lModelo1
	nMaxArray := Len(aCols)

	//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	//³ Carrega ja gravados ³
	//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
	dbSelectArea(cAlias1)
	If ! INCLUI .And. MsSeek(xFilial(cAlias1)+&(cChave))
		While !Eof() .And. xFilial(cAlias) + &(cChave1) == xFilial(cAlias1) + &(cChave)
			Aadd(aAnterior,RecNo())
	 		dbSkip()
		Enddo
	Endif

	dbSelectArea(cAlias1)
	nItem := 1

	For nCampos := 1 to nMaxArray

		If Len(aAnterior) >= nCampos
			If ! INCLUI
				DbGoto(aAnterior[nCampos])
			EndIf
			RecLock(cAlias1,.F.)
		Else
			RecLock(cAlias1,.T.)
		Endif

		//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		//³ Verifica se tem marcacao para apagar.                          ³
		//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
	   If aCols[nCampos][Len(aCols[nCampos])]
			RecLock(cAlias1,.F.,.T.)
			dbDelete()
		Else
			For nHeader := 1 to Len(aHeader)
				If aHeader[nHeader][10] # "V" .And. ! "_ITEM" $ AllTrim(aHeader[nHeader][2])
					Replace &(Trim(aHeader[nHeader][2])) With aCols[nCampos][nHeader]
				ElseIf "_ITEM" $ AllTrim(aHeader[nHeader][2])
					Replace &(AllTrim(aHeader[nHeader][2])) With StrZero(nItem ++,;
						Len(&(AllTrim(aHeader[nHeader][2]))))
				Endif
			Next

			//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
			//³ Atualiza as chaves de itens ³
			//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
			If Type("aChaves") # "U" .Or. aChaves # Nil
				Replace &(cAlias1 + "_FILIAL") With xFilial(cAlias1)
				For nChaves := 1 To Len(aChaves)
					Replace &(aChaves[nChaves][1]) With &(aChaves[nChaves][2])
				Next
			Endif

			If Type("cCompIt") <> "U" .And. cCompIt <> Nil		// Funcao para complementar
				&cCompIt.(aCols[nCampos])						// gravacoes no item
			Endif
			dbSelectArea(cAlias1)
		Endif

	Next nCampos
Endif

END TRANSACTION

Return .T.

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡„o    ³AdmDelMod ³ Autor ³ Wagner Mobile Costa   ³ Data ³ 21/08/01 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡„o ³ Funcao para delecao em formato Modelo 1/2/3                ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Sintaxe   ³ AdmDelMod(cPar1, cPar2)                                    ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³ ExpC1 = Alias do arquivo                                   ³±±
±±³          ³ ExpC1 = Alias detalhe do arquivo                           ³±±
±±³          ³ ExpL1 = Identifica se eh tipo MODELO 1 (Sem aCols)         ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                   ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/

Function AdmDelMod(cAlias,cAlias1,lModelo1)

BEGIN TRANSACTION

If ! lModelo1
	//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	//³ Deleta os itens ³
	//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
	dbSelectArea( cAlias1 )
	MsSeek(xFilial(cAlias1) + &(cChave))
	While !Eof() .And. xFilial(cAlias) + &(cChave1) == xFilial(cAlias1) + &(cChave)
		RecLock(cAlias1,.F.,.T.)
		dbDelete()
		dbSkip()
	End
Endif

If lModelo1 .Or. cAlias # cAlias1    // Se igual eh modelo 2, ou seja nao tem cabecalho
	//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	//³ Deleta o cabecalho ³
	//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
	dbSelectArea(cAlias)
	RecLock(cAlias,.F.,.T.)
	dbDelete()
	dbSelectArea(cAlias)
Endif

END TRANSACTION

Return .T.

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o	 ³AdmMemory   ³ Autor ³ Wagner Mobile       ³ Data ³ 04/09/01 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³Cria variaveis M-> para uso na Enchoice()					  ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso		 ³Enchoice												  	  ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function AdmMemory(cAlias)

SX3->(DbSetOrder(1))
SX3->(MsSeek(cAlias))

While ! SX3->(Eof()) .And. SX3->x3_arquivo == cAlias
	If SX3->X3_CONTEXT = "V" 	// Campo virtual
		If ! Empty(SX3->X3_INIBRW)
			_SetOwnerPrvt(Trim(SX3->X3_CAMPO), &(AllTrim(SX3->X3_INIBRW)))
		Else
			_SetOwnerPrvt(Trim(SX3->X3_CAMPO), CriaVar(Trim(SX3->X3_CAMPO)))
		Endif
	Else
		If INCLUI
			_SetOwnerPrvt(Trim(SX3->X3_CAMPO), CriaVar(Trim(SX3->X3_CAMPO)))
		Else
			_SetOwnerPrvt(Trim(SX3->X3_CAMPO), (cAlias)->&(Trim(SX3->X3_CAMPO)))
		EndIf
	Endif
	SX3->(DbSkip())
EndDo

Return .T.

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o	 ³AdmSqlName  ³ Autor ³ Wagner Mobile       ³ Data ³ 03/05/02 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descricao ³Encapsula RetSqlName para uso em ambiente CodeBase		  ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso		 ³Enchoice												  	  ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function AdmSqlName(cAlias)

#IfDef TOP
	Return RetSqlName(cAlias)
#Else
	Return ""
#Endif


/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºFuncao    ³FormatIn  ºAutor  ³Claudio D. de Souza º Data ³  30/08/01   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Formatar uma string para ser utilizada no clausula IN do    º±±
±±º          ³comando SELECT em ambiente SQL.                             º±±
±±º          ³Exemplo:                                                    º±±
±±º          ³FormatIn("BA /AB-/CA-/XX+", "/")= "('BA','AB-','CA-','XX+')"º±±
±±º          ³Parametros:                                                 º±±
±±º          ³cString  -> String a ser formatada                          º±±
±±º          ³cSep     -> Separador das strings                           º±±
±±º          ³Retorno:                                                    º±±
±±º          ³cRet  -> String formatada, conforme exemplo                 º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ AP6                                                        º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
FUNCTION FormatIn( cString, cSep, nTam, lFinPar )
   LOCAL cRet := "('",; // array de string separadas
         nPoSep         // posicao do separador da string
	DEFAULT nTam := 0
	DEFAULT lFinPar := .F.  // para uso em query do MV_CARTEIR
   WHILE .T.
      // localiza a posicao do separador e separa a string encontrada
      If nTam > 0
      	If !lFinPar
	      	nPoSep := nTam
	  	Else
      		nPoSep := nTam + 1
    	Endif
      Else
	      nPoSep := AT(cSep,cString)
	  Endif
	  If !lFinPar
	      cRet   += IF(nPoSep#0, LEFT(cString,nPoSep-If(nTam>0,0,1))+"','", cString)
	  Else
      	  cRet   += IF(nPoSep#0, LEFT(cString,nPoSep-1)+"','", cString)
      Endif
      // verifica se existem mais separadores
      IF nPoSep#0
      	If Len(cString) > nTam
         	cString := SUBSTR(cString,nPoSep+1)
         Else
         	cRet := Left(cRet,Len(cRet)-3)
     			cRet += "')"
   	      EXIT
   	   Endif
      ELSE
			cRet += "')"
         EXIT
      ENDIF
   ENDDO

RETURN cRet

/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o	 ³AdmRatEst ³ Autor ³ Wagner Mobile Costa   ³ Data ³ 16/05/01 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³Escolhe rateio externo (pre-configurado) e preenche tela 	  ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso		 ³ Rateio de Contas a Pagar/Compras								     ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function AdmRatExt(aHeader, aCols, bCarTmp, oModel)

Local oDlg1
Local cCodRateio	:= CriaVar("CTJ_RATEIO")
Local nOpca 		:= 0
Local cItem			:= If( ValType(oModel) == 'O'  , '' , StrZero(0,Len(aCols[1][1])) )
Local lRatBlq := .F.

DEFINE MSDIALOG oDlg1 FROM  94,1 TO 170,175 TITLE STR0002 PIXEL //"Escolha o Rateio"

@ 02,05 TO 22, 82 OF oDlg1  PIXEL

@ 07,10 MSGET cCodRateio F3 "CTJ" Picture "@!" SIZE 070,10 OF oDLG1 PIXEL ;
		Valid Fa050CodRat(cCodRateio,2)

DEFINE SBUTTON FROM 25,005 TYPE 1 ENABLE OF oDlg1 ACTION (nOpca := 1,;
Iif(Fa050ValRat(2,cCodRateio),oDlg1:End(),nOpca := 0))

DEFINE SBUTTON FROM 25,055 TYPE 2 ENABLE OF oDlg1 ACTION (oDlg1:End(),nOpca := 0)

ACTIVATE MSDIALOG oDlg1 CENTERED

If nOpca == 1

	If ValType(oModel) <> 'O'
		Eval(bCarTmp, aCols, aHeader, @cItem, .T.,@lRatBlq)

		dbSelectArea("CTJ")			// Vale somente para digitacao
		dbSetOrder(1)
		dbSeek(xFilial()+cCodRateio)
		While !Eof() .And. CTJ->CTJ_FILIAL == xFilial() .And. CTJ->CTJ_RATEIO == cCodRateio
			Eval(bCarTmp, aCols, aHeader, @cItem, .F.,@lRatBlq)
			dbSelectArea("CTJ")
			dbSkip()
		EndDo
	Else
		Eval(bCarTmp, oModel, @cItem, .T.,@lRatBlq)

		dbSelectArea("CTJ")			// Vale somente para digitacao
		dbSetOrder(1)
		dbSeek(xFilial()+cCodRateio)
		While !Eof() .And. CTJ->CTJ_FILIAL == xFilial() .And. CTJ->CTJ_RATEIO == cCodRateio
			Eval(bCarTmp, oModel, @cItem, .F.,@lRatBlq)
			dbSelectArea("CTJ")
			dbSkip()
		EndDo
	EndIf
EndIf

Return .T.

/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³ AdmPath      ³ Autor ³ Simone Mie Sato    	³ Data ³ 04/02/03 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³ Usado no SXB (XB_ALIAS= "DIR") para buscar um determinado arq. ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Uso       ³ Siga                                                           ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function AdmPath()

Local cType			:= "*.*"

cAdmArq	:= cGetFile(cType, (STR0003+Subs(cType,1,7) ),,,,GETF_NETWORKDRIVE+GETF_LOCALHARD+GETF_LOCALFLOPPY ) // "Selecione arquivo "

Return !Empty(cAdmArq)

/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³ AdmArq       ³ Autor ³ Simone Mie Sato    	³ Data ³ 04/02/03 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³ Usado no SXB (XB_ALIAS= "DIR") para buscar um determinado arq. ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Uso       ³ Siga                                                           ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function AdmArq()

Return(cAdmArq)

/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³ GravaCv4     ³ Autor ³ Claudio D. de Souza	³ Data ³ 06/08/03 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³ Grava CV4 - Rateio efetuado a partir de um rateio pre-configu- |±±
±±³          ³ rado ou digitado pelo usuario.                                 |±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Uso       ³ SigaCtb/SigaFin                                                ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function	GravaCv4( cSeqCv4, dDataSeq, cDebito, cCredit, nPercent, nValor, cHist,;
						 cCcc, cCcd, cItemD, cItemC, cClVlDb, cClVlCr, cItSeqCV4, cProcPCO, cItemPCO, cProgr, aEntCont )
Local lRet 		:= .F.
Local aArea 	:= GetArea()
Local nQtdEnt 	:= Len(aEntCont)
Local nEnt		:= 0

If RecLock("CV4", .T. )
	lRet := .T.
	CV4->CV4_FILIAL	:= xFilial("CV4")
	CV4->CV4_SEQUEN	:= cSeqCv4
	CV4->CV4_DTSEQ		:= dDataSeq
	CV4->CV4_DEBITO	:= cDebito
	CV4->CV4_CREDIT	:= cCredit
	CV4->CV4_PERCEN	:= nPercent
	CV4->CV4_VALOR		:= nValor
	CV4->CV4_HIST		:= cHist
	CV4->CV4_CCC		:= cCcc
	CV4->CV4_CCD		:= cCcd
	CV4->CV4_ITEMD		:= cItemD
	CV4->CV4_ITEMC		:= cItemC
	CV4->CV4_CLVLDB	:= cClVlDb
	CV4->CV4_CLVLCR	:= cClVlCr
	If CV4->(FieldPos("CV4_ITSEQ")) > 0 .And. cItSeqCV4 != NIL
		CV4->CV4_ITSEQ	:= cItSeqCV4
	EndIf

	For nEnt := 1 To nQtdEnt
		&("CV4->CV4_EC" +  aEntCont[nEnt][1] + "DB") := aEntCont[nEnt][2]
		&("CV4->CV4_EC" +  aEntCont[nEnt][1] + "CR") := aEntCont[nEnt][3]
	Next nEnt

	CV4->(MsUnlock())
    FKCOMMIT()

    IF ExistBlock("F050CV4")
		//ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		//³ Ponto de Entrada para Gravacao dos campos de usuarios         ³
		//ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ
		ExecBlock("F050CV4",.f.,.f.)
	Endif

	//Gera lancamento para o modulo PCO
	PcoDetLan( cProcPCO, cItemPCO, cProgr ) //("000021","01","FINA050")

Endif

RestArea(aArea)

Return lRet

/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³LeDadosCV4	 ³ Autor ³ Claudio D. de Souza	³ Data ³ 07/08/03 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³ Le os dados do CV4 - Rateio efetuado a partir de um rateio     |±±
±±³          ³ pre-configurado ou digitado pelo usuario e inclui no alias TMP|±±
±±³          ³ para visualizacao do rateio. 	                                 |±±
±±³          ³ nValRat deve ser enviada por referencia								|±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Uso       ³ SigaFin         			                                       ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function LeDadosCV4(cChave,lConta,lCusto,lItem,lClVl,nValRat, cPadrao, nValorBase, lExclusao, aRecCV4)
Local aArea 	:= GetArea()
Local aAreaCV4 := CV4->(GetArea())
Local cBusca
Local nRecCTK, nRecCV4
Local lMovEnt05 := CtbMovSaldo("CT0",,'05')
Local lMovEnt06 := CtbMovSaldo("CT0",,'06')
Local lMovEnt07 := CtbMovSaldo("CT0",,'07')
Local lMovEnt08 := CtbMovSaldo("CT0",,'08')
Local lMovEnt09 := CtbMovSaldo("CT0",,'09')

Default lExclusao := .T.
Default aRecCV4 := {}

cChave := Rtrim(cChave)

dbSelectArea("CV4")
dbSetOrder(1)
If MsSeek(cChave)   //cChave jah contem filial

	While CV4->(!Eof()) .And.;
			CV4->CV4_FILIAL+DTOS(CV4->CV4_DTSEQ)+CV4->CV4_SEQUEN == RTrim(cChave)

		dbSelectArea("TMP")
		dbAppend()
		// Carrega campos
		If lConta
			TMP->CTJ_DEBITO	:= CV4->CV4_DEBITO //If( lExclusao, CV4->CV4_CREDIT, CV4->CV4_DEBITO )
			TMP->CTJ_CREDIT	:= CV4->CV4_CREDIT //If( lExclusao, CV4->CV4_DEBITO, CV4->CV4_CREDIT )
		EndIf

		If lCusto
			TMP->CTJ_CCD		:= CV4->CV4_CCD //If( lExclusao, CV4->CV4_CCC, CV4->CV4_CCD )
			TMP->CTJ_CCC		:= CV4->CV4_CCC //If( lExclusao, CV4->CV4_CCD, CV4->CV4_CCC )
		EndIf

		If lItem
			TMP->CTJ_ITEMD	:= CV4->CV4_ITEMD //If( lExclusao, CV4->CV4_ITEMC, CV4->CV4_ITEMD )
			TMP->CTJ_ITEMC	:= CV4->CV4_ITEMC //If( lExclusao, CV4->CV4_ITEMD, CV4->CV4_ITEMC )
		EndIf
		If lCLVL
			TMP->CTJ_CLVLDB	:= CV4->CV4_CLVLDB //If( lExclusao, CV4->CV4_CLVLCR, CV4->CV4_CLVLDB )
			TMP->CTJ_CLVLCR	:= CV4->CV4_CLVLCR //If( lExclusao, CV4->CV4_CLVLDB, CV4->CV4_CLVLCR )
		EndIf

		If lMovEnt05
			TMP->CTJ_EC05DB	:= CV4->CV4_EC05DB //If( lExclusao, CV4->CV4_CLVLCR, CV4->CV4_CLVLDB )
			TMP->CTJ_EC05CR	:= CV4->CV4_EC05CR //If( lExclusao, CV4->CV4_CLVLDB, CV4->CV4_CLVLCR )
		EndIf

		If lMovEnt06
			TMP->CTJ_EC06DB	:= CV4->CV4_EC06DB //If( lExclusao, CV4->CV4_CLVLCR, CV4->CV4_CLVLDB )
			TMP->CTJ_EC06CR	:= CV4->CV4_EC06CR //If( lExclusao, CV4->CV4_CLVLDB, CV4->CV4_CLVLCR )
		EndIf

		If lMovEnt07
			TMP->CTJ_EC07DB	:= CV4->CV4_EC07DB //If( lExclusao, CV4->CV4_CLVLCR, CV4->CV4_CLVLDB )
			TMP->CTJ_EC07CR	:= CV4->CV4_EC07CR //If( lExclusao, CV4->CV4_CLVLDB, CV4->CV4_CLVLCR )
		EndIf

		If lMovEnt08
			TMP->CTJ_EC08DB	:= CV4->CV4_EC08DB //If( lExclusao, CV4->CV4_CLVLCR, CV4->CV4_CLVLDB )
			TMP->CTJ_EC08CR	:= CV4->CV4_EC08CR //If( lExclusao, CV4->CV4_CLVLDB, CV4->CV4_CLVLCR )
		EndIf

		If lMovEnt09
			TMP->CTJ_EC09DB	:= CV4->CV4_EC09DB //If( lExclusao, CV4->CV4_CLVLCR, CV4->CV4_CLVLDB )
			TMP->CTJ_EC09CR	:= CV4->CV4_EC09CR //If( lExclusao, CV4->CV4_CLVLDB, CV4->CV4_CLVLCR )
		EndIf

		TMP->CTJ_HIST			:= CV4->CV4_HIST
		TMP->CTJ_VALOR		:= CV4->CV4_VALOR
		TMP->CTJ_PERCEN		:= CV4->CV4_PERCEN
		TMP->CTJ_FLAG 			:= .F.

		// Soma efetivamente os valores digitados no rateio -> e nao os gerados
		// pelo lancamento contabil
		nValRat += TMP->CTJ_VALOR
		aAdd(aRecCV4, CV4->(Recno()))

		If ExistBlock("F50CV4TMP1")
			nRecCV4 := CV4->( Recno() )
			ExecBlock("F50CV4TMP1", .F., .F., {"CV4"})
			CV4->( dbGoto(nRecCV4) )//volta para o registro que estava antes de entrar no ponto de entrada
		Endif

		dbSelectArea("CV4")
		dbSkip()

	EndDo
Else

	If cPadrao == "512"
		cBusca := "511"
	ElseIf cPadrao == "557"
		cBusca := "516"
	ElseIf cPadrao == "558"
		cBusca := "517"
	Else
		cBusca := cPadrao
	EndIf

	CTK->(MsSeek(xFilial("CTK")+cChave+cBusca))
	cTPSald	:= CTK->CTK_TPSALD								/// DETERMINA O TIPO DE SALDO EM QUE FOI PROCESSADO O RATEIO
	If Empty(cTPSald)										/// SE A PRIMEIRA SEQUENCIA NÃO TEM O TIPO DE SALDO
	   nRecCTK := CTK->(Recno())
		While CTK->(!Eof()) .And. CTK->CTK_FILIAL == xFilial("CTK") .And. CTK->(CTK_SEQUEN+CTK_LP) == cChave+cBusca
			If !Empty(CTK->CTK_TPSALD)
				cTPSald	:= CTK->CTK_TPSALD					/// O TIPO DE SALDO CARREGADO SERÁ O PRIMEIRO VALIDO.
				Exit
			Endif
			CTK->(dbSkip())
		EndDo
		CTK->(MsGoTo(nRecCTK))								/// SE NÃO TIVER NENHUM PREENCHIDO DEVE SER EM BRANCO MESMO
	Endif

	While CTK->(!Eof()) .And. CTK->CTK_FILIAL == xFilial("CTK") .And.;
			CTK->(CTK_SEQUEN+CTK_LP)== cChave+cBusca
		If CTK->CTK_CONTAB == "4" .And. CTK->CTK_TPSALD == cTPSald
			dbSelectArea("TMP")
			dbAppend()

			// Carrega campos -> Invertidos, pois eh exclusao
			If lConta				// considera contas no rateio -> mv_par03 == 1
				TMP->CTJ_DEBITO	:= If( lExclusao, CTK->CTK_CREDIT, CTK->CTK_DEBITO )
				TMP->CTJ_CREDIT	:= If( lExclusao, CTK->CTK_DEBITO, CTK->CTK_CREDIT )
			EndIf

			If lCusto
				TMP->CTJ_CCD		:= If( lExclusao, CTK->CTK_CCC, CTK->CTK_CCD )
				TMP->CTJ_CCC		:= If( lExclusao, CTK->CTK_CCD, CTK->CTK_CCC )
			EndIf
			If lItem
				TMP->CTJ_ITEMD	:= If( lExclusao, CTK->CTK_ITEMC, CTK->CTK_ITEMD )
				TMP->CTJ_ITEMC	:= If( lExclusao, CTK->CTK_ITEMD, CTK->CTK_ITEMC )
			EndIf
			If lCLVL
				TMP->CTJ_CLVLDB	:= If( lExclusao, CTK->CTK_CLVLCR, CTK->CTK_CLVLDB )
				TMP->CTJ_CLVLCR	:= If( lExclusao, CTK->CTK_CLVLDB, CTK->CTK_CLVLCR )
			EndIf

			TMP->CTJ_HIST			:= CTK->CTK_HIST
			TMP->CTJ_VALOR			:= CTK->CTK_VLR01
			TMP->CTJ_PERCEN		:= Round(((TMP->CTJ_VALOR) / nValorBase)*100,2)
			TMP->CTJ_FLAG 			:= .F.

			nValRat += TMP->CTJ_VALOR

			If ExistBlock("F50CV4TMP1")
				nRecCTK := CTK->( Recno() )
				ExecBlock("F50CV4TMP1", .F., .F., { "CTK" })
				CTK->( dbGoto(nRecCTK) )//volta para o registro que estava antes de entrar no ponto de entrada
			Endif

		Endif
		CTK->(dbSkip())
	EndDo

Endif

CV4->(RestArea(aAreaCv4))
RestArea(aArea)

Return .T.

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³ClassCTBDCºAutor  ³Marcos S. Lobo      º Data ³  12/15/03   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Funcao de tela generica para digitação dos dados contabeis. º±±
±±º          ³Parametro1: cAliasGrv = Alias da tabela a ser gravada(posic)º±±
±±º          ³Parametro2: aCpsCTBD  = Array com os campos a debito (max4) º±±
±±º          ³Parametro3: aCpsCTBC  = Array com os campos a credito(max4) º±±
±±º          ³Parametro4: lGrava    = Se deve gravar os campos digitados. º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºRetorno   ³Array de posições fixas com o conteúdo dos gets digitados	  º±±
±±º          ³aRetorna[1] = Conta Debito							      º±±
±±º          ³aRetorna[2] = C.Custo Debito							      º±±
±±º          ³aRetorna[3] = Item Contabil Debito						  º±±
±±º          ³aRetorna[4] = Cl.Valor Debito							      º±±
±±º          ³aRetorna[5] = Conta Credito							      º±±
±±º          ³aRetorna[6] = C.Custo Credito							      º±±
±±º          ³aRetorna[7] = Item Contabil Credito						  º±±
±±º          ³aRetorna[8] = Cl.Valor Credito						      º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ AP6                                                        º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function ClassCTBDC(cAliasGrv,aCpsCTBD,aCpsCTBC,lGrava)

Local aAreaAnt  := GetArea()
Local nOrdSX3	:= SX3->(IndexOrd())
Local nRegSX3	:= SX3->(Recno())
Local cPrefCpo	:= ""
Local lOkCTB	:= .F.				/// SE PASSOU (.T.) OU NÃO (.F.) PELAS VALIDAÇÕES
Local cCTBCT1D  := ""
Local cCTBCT1C	:= ""
Local cCTBCTTD  := ""
Local cCTBCTTC	:= ""
Local cCTBCTDD  := ""
Local cCTBCTDC	:= ""
Local cCTBCTHD  := ""
Local cCTBCTHC	:= ""
Local oDlgCTB

Local aRetorna := {cCTBCT1D,cCTBCTTD,cCTBCTDD,cCTBCTHD,cCTBCT1C,cCTBCTTC,cCTBCTDC,cCTBCTHC}

Local nLinT		:= 000
Local nLinG		:= 000
Local nCol1		:= 010
Local nCol2		:= 110

Local lShowDlg	:= .F.

DEFAULT cAliasGrv	:= Alias()
DEFAULT aCpsCTBD 	:= {}
DEFAULT aCpsCTBC 	:= {}
DEFAULT lGrava		:= .T.

cPrefCpo := PrefixoCpo(cAliasGrv)

lCTBDCVLD := ExistBlock("CTBDCVLD")						/// PONTO DE ENTRADA PARA VALIDACOES DA TOK (CTBDCVLD)

If Len(aCpsCTBD) <= 0
	aAdd(aCpsCTBD,cPrefCpo+"_DEBITO")					/// CAMPO PARA A CONTA A DEBITO
Endif
If Len(aCpsCTBD) <= 1
	aAdd(aCpsCTBD,cPrefCpo+"_CCD"   )					/// CAMPO PARA O C.CUSTO A DEBITO
Endif
If Len(aCpsCTBD) <= 2
	aAdd(aCpsCTBD,cPrefCpo+"_ITEMD" )					/// CAMPO PARA O ITEM A DEBITO
Endif
If Len(aCpsCTBD) <= 3
	aAdd(aCpsCTBD,cPrefCpo+"_CLVLDB")					/// CAMPO PARA A CL.VLR A DEBITO
Endif

If Len(aCpsCTBC) <= 0
	aAdd(aCpsCTBC,cPrefCpo+"_CREDIT")					/// CAMPO PARA A CONTA A CREDITO
Endif
If Len(aCpsCTBC) <= 1
	aAdd(aCpsCTBC,cPrefCpo+"_CCC"   )					/// CAMPO PARA O C.CUSTO A CREDITO
Endif
If Len(aCpsCTBC) <= 2
	aAdd(aCpsCTBC,cPrefCpo+"_ITEMC" )					/// CAMPO PARA O ITEM A CREDITO
Endif
If Len(aCpsCTBC) <= 3
	aAdd(aCpsCTBC,cPrefCpo+"_CLVLCR")					/// CAMPO PARA A CL. VLR A CREDITO
Endif

/////////////////////////////////////////////////////////////////////////////////////////////
//// MONTAGEM DA TELA PARA LANCAMENTO DOS DADOS CONTABEIS
/////////////////////////////////////////////////////////////////////////////////////////////

DEFINE MSDIALOG oDlgCTB FROM 000,000 TO 300,400 TITLE STR0004 PIXEL //"Dados Contabeis"

nLinT += 25
nLinG := 32

dbSelectArea(cAliasGrv)

If !Empty(aCpsCTBD[1])							//// TRATAMENTOS PARA CARREGAR AS CONTAS CONTABEIS
	dbSelectArea("SX3")
	SX3->(dbSetOrder(2))
	If SX3->(MsSeek(aCpsCTBD[1],.F.)) .AND. X3Uso(SX3->X3_USADO)
		@ nLinT,nCol1 SAY RetTitle(aCpsCTBD[1]) PIXEL
		cCTBCT1D := CriaVar(aCpsCTBD[1])
		@ nLinG,nCol1 MSGET cCTBCT1D SIZE Len(cCTBCT1D)*3.4,10 F3 "CT1" VALID Vazio(cCTBCT1D) .or. CTB105CTA(cCTBCT1D) OF oDlgCTB PIXEL
		lShowDlg := .T.
	Endif
Endif
If !Empty(aCpsCTBC[1])
	dbSelectArea("SX3")
	SX3->(dbSetOrder(2))
	If SX3->(MsSeek(aCpsCTBC[1],.F.)) .AND. X3Uso(SX3->X3_USADO)
		@ nLinT,nCol2 SAY RetTitle(aCpsCTBC[1]) PIXEL
		cCTBCT1C := CriaVar(aCpsCTBC[1])
		@ nLinG,nCol2 MSGET cCTBCT1C SIZE Len(cCTBCT1C)*3.4,10 F3 "CT1" VALID Vazio(cCTBCT1C) .or. CTB105CTA(cCTBCT1C) OF oDlgCTB PIXEL
		lShowDlg := .T.
	Endif
Endif

If CtbMovSaldo("CTT")							//// TRATAMENTOS PARA CARREGAR OS CENTROS DE CUSTO
	nLinT := nLinG + 12
	nLinG := nLinT + 8
	If !Empty(aCpsCTBD[2])
		dbSelectArea("SX3")
		SX3->(dbSetOrder(2))
		If SX3->(MsSeek(aCpsCTBD[2],.F.)) .AND. X3Uso(SX3->X3_USADO)
			@ nLinT,nCol1 SAY RetTitle(aCpsCTBD[2]) PIXEL
			cCTBCTTD := CriaVar(aCpsCTBD[2])
			@ nLinG,nCol1 MSGET cCTBCTTD SIZE Len(cCTBCTTD)*3.4,10 F3 "CTT" VALID  Vazio(cCTBCTTD) .or. CTB105CC(cCTBCTTD) OF oDlgCTB PIXEL
			lShowDlg := .T.
		Endif
	Endif
	If !Empty(aCpsCTBC[2])
		dbSelectArea("SX3")
		SX3->(dbSetOrder(2))
		If SX3->(MsSeek(aCpsCTBC[2],.F.)) .AND. X3Uso(SX3->X3_USADO)
			@ nLinT,nCol2 SAY RetTitle(aCpsCTBC[2]) PIXEL
			cCTBCTTC := CriaVar(aCpsCTBC[2])
			@ nLinG,nCol2 MSGET cCTBCTTC SIZE Len(cCTBCTTC)*3.4,10 F3 "CTT" VALID  Vazio(cCTBCTTC) .or. CTB105CC(cCTBCTTC) OF oDlgCTB PIXEL
			lShowDlg := .T.
		Endif
	Endif
Endif

If CtbMovSaldo("CTD")							//// TRATAMENTOS PARA CARREGAR OS ITENS CONTABEIS
	nLinT := nLinG + 12
	nLinG := nLinT + 8
	If !Empty(aCpsCTBD[3])
		dbSelectArea("SX3")
		SX3->(dbSetOrder(2))
		If SX3->(MsSeek(aCpsCTBD[3],.F.)) .AND. X3Uso(SX3->X3_USADO)
			@ nLinT,nCol1 SAY RetTitle(aCpsCTBD[3]) PIXEL
			cCTBCTDD := CriaVar(aCpsCTBD[3])
			@ nLinG,nCol1 MSGET cCTBCTDD SIZE Len(cCTBCTDD)*3.4,10 F3 "CTD" VALID  Vazio(cCTBCTDD) .or. CTB105ITEM(cCTBCTDD) OF oDlgCTB PIXEL
			lShowDlg := .T.
		Endif
	Endif
	If !Empty(aCpsCTBC[3])
		dbSelectArea("SX3")
		SX3->(dbSetOrder(2))
		If SX3->(MsSeek(aCpsCTBC[3],.F.)) .AND. X3Uso(SX3->X3_USADO)
			@ nLinT,nCol2 SAY RetTitle(aCpsCTBC[3]) PIXEL
			cCTBCTDC := CriaVar(aCpsCTBC[3])
			@ nLinG,nCol2 MSGET cCTBCTDC SIZE Len(cCTBCTDC)*3.4,10 F3 "CTD" VALID  Vazio(cCTBCTDC) .or. CTB105ITEM(cCTBCTDC) OF oDlgCTB PIXEL
			lShowDlg := .T.
		Endif
	Endif
Endif

If CtbMovSaldo("CTH")							//// TRATAMENTOS PARA CARREGAR AS CLASSES DE VALOR
	nLinT := nLinG + 12
	nLinG := nLinT + 8
	If !Empty(aCpsCTBD[4])
		dbSelectArea("SX3")
		SX3->(dbSetOrder(2))
		If SX3->(MsSeek(aCpsCTBD[4],.F.)) .AND. X3Uso(SX3->X3_USADO)
			@ nLinT,nCol1 SAY RetTitle(aCpsCTBD[4]) PIXEL

			cCTBCTHD := CriaVar(aCpsCTBD[4])
			@ nLinG,nCol1 MSGET cCTBCTHD SIZE Len(cCTBCTHD)*3.4,10 F3 "CTH" VALID  Vazio(cCTBCTHD) .or. CTB105CLVL(cCTBCTHD) OF oDlgCTB PIXEL
			lShowDlg := .T.
		Endif
	Endif
	If !Empty(aCpsCTBC[4])
		dbSelectArea("SX3")
		SX3->(dbSetOrder(2))
		If SX3->(MsSeek(aCpsCTBC[4],.F.)) .AND. X3Uso(SX3->X3_USADO)
			@ nLinT,nCol2 SAY RetTitle(aCpsCTBC[4]) PIXEL
			cCTBCTHC := CriaVar(aCpsCTBC[4])
			@ nLinG,nCol2 MSGET cCTBCTHC SIZE Len(cCTBCTHC)*3.4,10 F3 "CTH" VALID  Vazio(cCTBCTHC) .or. CTB105CLVL(cCTBCTHC) OF oDlgCTB PIXEL
			lShowDlg := .T.
		Endif
	Endif
Endif

If !lShowDlg			/// SE NENHUM CAMPO FOI HABILITADO NAO MOSTRA A TELA
	SX3->(dbSetOrder(nOrdSX3))
	SX3->(dbGoTo(nRegSX3))
	RestArea(aAreaAnt)		/// ABORTA EXIBIÇAO
	Return(aRetorna)
Endif

nLinT := nLinG + 24
@ 15,nCol1-5 GROUP oGrpDeb TO nLinT,nCol2-10 OF oDlgCTB LABEL STR0005 PIXEL //"Debito"
@ 15,nCol2-5 GROUP oGrpCrd TO nLinT,nCol2+90 OF oDlgCTB LABEL STR0006 PIXEL //"Credito"

oDlgCTB:nHeight:= nLinT + 160

ACTIVATE DIALOG oDlgCTB ON INIT EnchoiceBar(oDlgCTB,{||If(lOkCTB := VldCTBDC(1,cAliasGrv,aCpsCTBD,aCpsCTBC,cCTBCT1D,cCTBCTTD,cCTBCTDD,cCTBCTHD,cCTBCT1C,cCTBCTTC,cCTBCTDC,cCTBCTHC),oDlgCTB:End(),)},{||If(lOkCTB := VldCTBDC(2,cAliasGrv,aCpsCTBD,aCpsCTBC,cCTBCT1D,cCTBCTTD,cCTBCTDD,cCTBCTHD,cCTBCT1C,cCTBCTTC,cCTBCTDC,cCTBCTHC),oDlgCTB:End(),)}) CENTERED

If lOkCTB						//// SE CONFIRMOU A TELA DE CLASSIFICACAO
	If lGrava					//// SE FOI PASSADO PARA EFETUAR GRAVACAO
		aGrvD := {cCTBCT1D,cCTBCTTD,cCTBCTDD,cCTBCTHD}
		aGrvC := {cCTBCT1C,cCTBCTTC,cCTBCTDC,cCTBCTHC}
		GrvCTBDC(cAliasGrv,aCpsCTBD,aCpsCTBC,aGrvD,aGrvC)
	Endif
	aRetorna := {cCTBCT1D,cCTBCTTD,cCTBCTDD,cCTBCTHD,cCTBCT1C,cCTBCTTC,cCTBCTDC,cCTBCTHC}		/// DEFINE O RETORNO
Endif

SX3->(dbSetOrder(nOrdSX3))
SX3->(dbGoTo(nRegSX3))
RestArea(aAreaAnt)

Return(aRetorna)

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³VldCTBDC  ºAutor  ³Marcos S. Lobo      º Data ³  12/15/03   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Efetua as validações para os campos digitados               º±±
±±º          ³na tela generica de classificacao contabil.                 º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ AP6                                                        º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Static Function VldCTBDC(nOPCTBDC,cAliasGrv,aCpsCTBD,aCpsCTBC,cCTBCT1D,cCTBCTTD,cCTBCTDD,cCTBCTHD,cCTBCT1C,cCTBCTTC,cCTBCTDC,cCTBCTHC)

Local lTOk := .T.

DEFAULT nOPCTBDC := 1			/// INDICA 1 = Ok / 2 = Cancel DE ACORDO COM O BOTAO SELECIONADO

If lCTBDCVLD
	lTOk := ExecBlock("CTBDCVLD",.F.,.F.,nOPCTBDC,cAliasGrv,aCpsCTBD,aCpsCTBC,cCTBCT1D,cCTBCTTD,cCTBCTDD,cCTBCTHD,cCTBCT1C,cCTBCTTC,cCTBCTDC,cCTBCTHC)
Endif

Return(lTOk)

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³GrvCTBDC  ºAutor  ³Marcos S. Lobo      º Data ³  12/15/03   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Funcao para gravacao da tela generica para classificacao    º±±
±±º          ³contabil.                                                   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ AP6                                                        º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Static Function GrvCTBDC(cAliasGrv,aCpsCTBD,aCpsCTBC,aGrvD,aGrvC)

Local nXCpo := 1

RecLock(cAliasGrv,.F.)
For nXCpo := 1 to Len(aCpsCTBD)
	If !Empty(aCpsCTBD[nXCpo]) .and. !Empty(aGrvD[nXCpo])
		&("Field->"+ALLTRIM(aCpsCTBD[nXCpo])) := aGrvD[nXCpo]
	Endif
Next

For nXCpo := 1 to Len(aCpsCTBC)
	If !Empty(aCpsCTBC[nXCpo]) .and. !Empty(aGrvC[nXCpo])
		&("Field->"+ALLTRIM(aCpsCTBC[nXCpo])) := aGrvC[nXCpo]
	Endif
Next
(cAliasGrv)->(MsUnlock())

Return

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³CTBNOTA   ºAutor  ³Marcos S. Lobo      º Data ³  04/07/04   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Mostra a tela da nota fiscal de entrada/saida na consulta deº±±
±±º          ³rastreamento gerados pelos LP 610 ou 650 (Itens de NF)      º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ Rotina do cadastro de Relacionamento SIGACTB (LP:610/650)  º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function CTBNota()
Local aAreaNota	:= GetArea()
Local cAliasCAB := "SF1"
Local cKEYCAB	:= ""
Local cArq		:= ""
Local nOrdem 	:= 0
Local cChave 	:= ""

dbSelectArea("CTL")
DO CASE
CASE CTL->CTL_ALIAS == "SD2"
	cAliasCAB := "SF2"
CASE CTL->CTL_ALIAS == "SF1"
	a103NFISCAL("SF1",SF1->(Recno()),2)
	RestArea(aAreaNota)
	Return
CASE CTL->CTL_ALIAS == "SF2"
	MC090VISUAL()
	RestArea(aAreaNota)
	Return
ENDCASE

cArq		:= CTL->CTL_ALIAS
nOrdem 		:= Val( CTL->CTL_ORDER )
cChave 		:= RTrim(CV3->CV3_KEY)
If	ChkFile( cArq )
	dbSelectArea(cArq)
	dbSetOrder(nOrdem)
	If dbSeek(cChave)

		dbSelectArea(cAliasCAB)
		dbSetOrder(1)
		cKEYCAB := IndexKey()

		dbSelectArea(CTL->CTL_ALIAS)
		cKEYCAB := STRTRAN(cKEYCAB,SubStr(cAliasCAB,2,2),SubStr(CTL->CTL_ALIAS,2,2))
		cKEYCAB := &(CTL->CTL_ALIAS+"->("+cKEYCAB+")")

		dbSelectArea(cAliasCAB)
		IF MsSeek(cKEYCAB,.F.)
			If cAliasCAB == "SF1"
				a103NFISCAL("SF1",Recno(),2)
			Else
				MC090VISUAL()
			Endif
		Endif
	Endif
Endif

RestArea(aAreaNota)
Return
/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³MsSomaMes ºAutor  ³Microsiga           º Data ³  16/05/06   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Soma meses em determinada data                              º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ Generico                                                   º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function MsSomaMes( dGivenDate, nAddMonths, lMakeEOM)
// Baseada em FT_MADD da Nanfor
  LOCAL nAdjDay, dTemp, i

  IF(VALTYPE(dGivenDate) != 'D', dGivenDate := DATE(), )
  IF(VALTYPE(nAddMonths) != 'N', nAddMonths := 0, )
  IF(VALTYPE(lMakeEOM)   != 'L', lMakeEom := .F., )

  nAdjDay := DAY( dGivenDate ) - 1

  dTemp := dGivenDate - nAdjDay     // first of month

  /* Trabalha sempre com o primeiro dia dos meses.*/
  For i := 1 To Abs(nAddMonths)
      dTemp += If( nAddMonths > 0, 31, -1 )
      dTemp += 1 - DAY( dTemp )
  Next

  If lMakeEom
     dTemp += 31 - DAY( dTemp + 31 )
  Else
     dTemp := MIN( (dTemp + nAdjday), (dTemp += 31 - DAY( dTemp + 31 )))
  Endif

Return dTemp

/*/
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³ MsgTimer   ³ Autor ³ Marcio Menon		  ³ Data ³ 17/01/07 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³ Mostra uma tela de mensagem informando ao usuário que a 		³±±
±±³          ³ tela será finalizada, conforme configuração dos				³±±
±±³          ³ parâmetros MV_FATOUT e MV_MSGTIM.           					³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³ nTimeMsg -> Informar quantos segundos a tela da mensagem     ³±±
±±³          ³ deverá aparecer antes do fechamento da tela principal.       ³±±
±±³          ³ oDlgAux  -> Obejto da tela principal que será fechada.		³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Retorno   ³ Nenhum.									                    ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ FINA090, FINA091, FINA110, FINA280, FINA281, FINA290 e	    ³±±
±±³       	 ³ FINA460.		                                                ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function MsgTimer(nTimeMsg,oDlgAux)

Local oDlgMsg
Local oTimer2
Local lMsgOk := .F.

DEFINE MSDIALOG oDlgMsg TITLE STR0007 From 10,10 To 18,45 OF oDlgAux STYLE DS_MODALFRAME	//"Atenção"
oDlgMsg:lCentered := .T.
oDlgMsg:lEscClose := .F. // Nao permite sair ao pressionar a tecla ESC.

@ 0.5, 1.8 Say STR0008										FONT oDlgMsg:oFont Of oDlgMsg	//"Esta tela será finalizada automaticamente em "
@ 1.5, 1.8 Say AllTrim(Str((nTimeMsg/1000)/60))+STR0009	FONT oDlgMsg:oFont Of oDlgMsg	//" minuto(s), caso continue sem utilização."

DEFINE SBUTTON FROM 40,55 TYPE 1 ACTION (lMsgOk:=.T.,oDlgMsg:End()) ENABLE OF oDlgMsg

oTimer2:= TTimer():New(nTimeMsg,{|| oDlgMsg:End() },oDlgMsg)
oTimer2:Activate()

oDlgMsg:Activate()

If !lMsgOK
	oDlgAux:End()
EndIf

Return

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³VerifMoedaºAutor  ³ Marcio Menon	     º Data ³  31/05/07   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³ Verifica se a Moeda informada existe no SX6 e no SM2.	 	  º±±
±±º          ³ 															      		  º±±
±±º          ³ Parametros:	nMoeda -> Número da Moeda			      		  º±±
±±º          ³ Retorno:    .T.    -> Moeda cadastrada				     		  º±±
±±º          ³ 			   .F.    -> Moeda nao cadastrada	      		  º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ Validação nos parametros dos relatório em que o campo      º±±
±±º          ³ Moeda foram modificados de tipo Combo para Get.	    		  º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function VerifMoeda(nMoeda)

Local cMoeda := ""

If nMoeda != 0
	cMoeda := AllTrim(Str(nMoeda))
Else
	Help(" ",1,"VERMOEDA",,STR0057,1,0)		//"Moeda não informada."
	Return .F.
EndIf

If SX6->(FieldPos("MV_MOEDA" + cMoeda)) == 0 .And. SM2->(FieldPos("M2_MOEDA" + cMoeda)) == 0
	Help(" ",1,"VERMOEDA",,STR0010,1,0)		//"Moeda não cadastrada."
	Return .F.
EndIf

Return .T.

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³AjustMoedaºAutor  ³ Marcio Menon		 º Data ³  31/05/07   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³ Altera a pergunta das Moedas do tipo Combo para Get,       º±±
±±º          ³ possibilitando o usuário utilizar mais que 5 moedas.       º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºParametros³ cGrupo  - > Nome do Grupo de Perguntas			          º±±
±±º          ³ cOrdem  - > Ordem da pergunta 				              º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ Relatórios/Financeiro                                      º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function AjustMoeda(cGrupo,cOrdem)
/*SNOARQUBE - Acesso à dicionário não permitido - Sistêmico jan/2019*/
Return nil



/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³ADMParSQL ºAutor  ³ARNALDO R. JUNIOR   º Data ³  15/07/2008 º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³ Função para conversão do filtro do usuario em relatorios   º±±
±±º          ³ Tipo SETPRINT para sintaxe SQL                             º±±
±±º          ³ Baseada nas funcoes GEPARSQL e PLSPARSQL                   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºParametros³ cFilADV --> aReturn[7] da SETPRINT                         º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ RELATORIOS TIPO SETPRINT                                   º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function ADMParSQL(cFilADV)
	Local cOk:=""
	Local cMeio:=""
	Local cFalta:=""
	Local nSub:=0
	Local lAtCifrao := .F.

	If '$' $ cFilAdv
		lAtCifrao := .T.
	EndIf

	cFilADV := Upper( cFilADV )
	cFilADV := StrTran( Upper(cFilADV),".AND."," AND ")
	cFilADV := StrTran( cFilADV, ".OR.",       " OR ")
	cFilADV := StrTran( cFilADV, "==",         " = ")
	cFilADV := StrTran( cFilADV, '"',          "'")
	cFilADV := StrTran( cFilADV, '$',          " IN ")
	cFilADV := StrTran( cFilADV, 'ALLTRIM(',   "LTRIM(")
	cFilADV := StrTran( cFilADV, 'DTOS',       "")
	cFilADV := StrTran( cFilADV, '!=',       " <> ")
	cFilADV := StrTran( cFilADV, '!',       " NOT ")
	cFalta  :=cFilADV
	//somente se tiver operador contem advpl $ deve entrar neste laco
	While lAtCifrao .And. At( ' IN ' ,  cFalta) <>0
		cOk:=cOk+SUBSTR(cFalta,1,At( ' IN ' ,  cFalta)-1)
		cFalta:=SUBSTR(cFalta,At( ' IN ' ,  cFalta),len(cFalta))
		nSub:=At( ' AND ' ,  cFalta)
		nSub:=iif(((At( ' OR ' ,  cFalta)>0) .AND.(At( ' OR ' ,  cFalta)<nSub)) .OR.(nSub==0),At( ' OR ' ,  cFalta),nSub)
		If nSub==0
			cOk:= cOk+StrTran(cFalta," IN "," IN (") +")"
			cFalta:=""
		Else
			cMeio:=substr(cFalta,At( ' IN ' ,  cFalta),nSub)
			cMeio:=StrTran(cMeio," IN "," IN (")
			If right(cMeio,1) =="A"
				cFalta:=substr(cFalta,nSub+3,len(cFalta))
				cMeio:= LEFT(cMeio,len(cMeio)-1) +   ") AND "
			Elseif right(cMeio,1) =="O"
				cFalta:=substr(cFalta,nSub+2,len(cFalta))
				cMeio:= LEFT(cMeio,len(cMeio)-1) +   ") OR "
			Endif
			cOk :=cOk+cMeio
		Endif
	EndDo
	If cOk!=''
		cFilADV:=cOk+cFalta
	Endif
	If At( '+' , cFilADV ) <> 0 .And. Upper(TcGetDb()) $ "ORACLE,POSTGRES,DB2,INFORMIX"
		cFilADV := StrTran( cFilADV, '+', "||")
	Endif
Return(cFilADV)

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Programa  ³ AddSqlExpr  ³ Autor ³ Renato F. Campos      ³ Data ³ 04.02.09 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Finalidade³ Adiciona uma clausula a uma condicao Where                    ³±±
±±³          ³ cWhere = Expressao where já montada                           ³±±
±±³          ³ cExpr  = Expressao a ser adicionada                           ³±±
±±³          ³                                                               ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Uso       ³ CtbXSal                                                       ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function AddSqlExpr( cWhere , cExpr , pValor , lOr , lParent )
Local cRet 		:= ''
Local cAux 		:= ''

DEFAULT cWhere 	:= ''
DEFAULT cExpr  	:= ''
DEFAULT pValor 	:= Nil
DEFAULT lOR    	:= .F.
DEFAULT lParent	:= .F.

// verificação dos campos de motangem da expressão
If Empty( cExpr )
	Return cWhere
Endif

// adição do where ou and
IF Empty( cWhere )
	cRet := " WHERE "
Else
	If !lOr
		cRet := cWhere + " AND "
	Else
		cRet := cWhere + " OR "
	Endif
Endif

// tratativa do tipo de campo para a montagem da expressão
IF  ValType( pValor ) == "N"    && numeric
	cAux := Alltrim( Str( pValor ) )

ElseIf ValType( pValor ) == "C" && character
	cAux := "'" + pValor + "'"

ElseIf ValType( pValor ) == "B" && block
	cAux := Eval( pValor )

ElseIf ValType( pValor ) == "D" && date
	cAux := "'" + Dtos( pValor ) + "'"

ElseIf ValType( pValor ) == "L" && logic
	cAux := IIf( pValor, "TRUE" , "FALSE" )

Else && undefined
//	cAux := ''Valor

Endif

// efetuo a tratativa do AdmParSQl
cExpr := ADMParSQL( cExpr + " " + cAux )

If lParent && verifico se é necessario a inclusão de parenteses
	cExpr := "( " + cExpr + " ) "
Endif

cRet += cExpr

RETURN cRet


/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³ADMGETFIL ºAutor  ³Rafael Gama         º Data ³  05/06/09   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³ Adm_Opcoes de pesquisa por filiais existente no cadastro deº±±
±±º          ³ empresa                                                    º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±³Retorno   ³ aSelFil(Contem todas as filiais da empresa selecionada)    ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±ºUso       ³ SIGACTB, SIGAATF, SIGAFIN                                  º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function ADMGETFIL(lTodasFil, lSohFilEmp, cAlias, lSohFilUn, lHlp, lExibTela, lTudoComp)
Local cEmpresa 	:= cEmpAnt
Local cTitulo	:= ""
Local MvPar		:= ""
Local MvParDef	:= ""
Local nI 		:= 0
Local aArea 	:= GetArea() 					 // Salva Alias Anterior
Local nReg	    := 0
Local nSit		:= 0
Local aSit		:= {}
Local aSit_Ant	:= {}
Local aFil 		:= {}
Local nTamFil	:= Len(xFilial("CT2"))
Local lDefTop 	:= IIF( FindFunction("IfDefTopCTB"), IfDefTopCTB(), .F.) // verificar se pode executar query (TOPCONN)
Local nInc		:= 0
Local aSM0		:= AdmAbreSM0()
Local aFilAtu	:= {}
Local lPEGetFil := ExistBlock("CTGETFIL")
Local lPESetFil := ExistBlock("CTSETFIL")
Local aFil_Ant
Local lGestao	:= AdmGetGest()
Local lFWCompany := FindFunction( "FWCompany" )
Local cEmpFil 	:= " "
Local cUnFil	:= " "
Local nTamEmp	:= 0
Local nTamUn	:= 0
Local lOk		:= .T.
Local lUserOk	:= .T.

Default lTodasFil 	:= .F.
Default lSohFilEmp 	:= .F.	//Somente filiais da empresa corrente (Gestao Corporativa)
Default lSohFilUn 	:= .F.	//Somente filiais da unidade de negocio corrente (Gestao Corporativa)
Default lHlp		:= .T.
Default cAlias		:= ""
Default lExibTela	:= .T.
Default lTudoComp   := .F.	/*Nível de compartilhamento da tabela. Define se no filtro de filiais serão consideradas todas as 
							filiais da empresa corrente, ou apenas a filial corrente quando a tabela (cAlias), estiver totalmente 
							compartilhada. A definição de listar ou não todas as filiais da empresa corrente deve sempre 
							ficar a cargo da regra de negócio da rotina que está consumindo a AdmGetFil*/

/*
Defines do SM0
SM0_GRPEMP  // Código do grupo de empresas
SM0_CODFIL  // Código da filial contendo todos os níveis (Emp/UN/Fil)
SM0_EMPRESA // Código da empresa
SM0_UNIDNEG // Código da unidade de negócio
SM0_FILIAL  // Código da filial
SM0_NOME    // Nome da filial
SM0_NOMRED  // Nome reduzido da filial
SM0_SIZEFIL // Tamanho do campo filial
SM0_LEIAUTE // Leiaute do grupo de empresas
SM0_EMPOK   // Empresa autorizada
SM0_GRPEMP  // Código do grupo de empresas
SM0_USEROK  // Usuário tem permissão para usar a empresa/filial
SM0_RECNO   // Recno da filial no SIGAMAT
SM0_LEIAEMP // Leiaute da empresa (EE)
SM0_LEIAUN  // Leiaute da unidade de negócio (UU)
SM0_LEIAFIL // Leiaute da filial (FFFF)
SM0_STATUS  // Status da filial (0=Liberada para manutenção,1=Bloqueada para manutenção)
SM0_NOMECOM // Nome Comercial
SM0_CGC     // CGC
SM0_DESCEMP // Descricao da Empresa
SM0_DESCUN  // Descricao da Unidade
SM0_DESCGRP // Descricao do Grupo
*/

//Caso o Alias não seja passado, traz as filiais que o usuario tem acesso (modo padrao)
lSohFilEmp := IF(Empty(cAlias),.F.,lSohFilEmp)
lSohFilUN  := IF(Empty(cAlias),.F.,lSohFilUn) .And. lSohFilEmp

//Caso use gestão corporativa , busca o codigo da empresa dentro do M0_CODFIL
//Em caso contrario, , traz as filiais que o usuario tem acesso (modo padrao)
cEmpFil := IIF(lGestao .and. lFwCompany, FWCompany(cAlias)," ")
cUnFil  := IIF(lGestao .and. lFwCompany, FWUnitBusiness(cAlias)," ")

//Tamanho do codigo da filial
nTamEmp := Len(cEmpFil)
nTamUn  := Len(cUnFil)

If lDefTop
	If !(lExibTela .AND. IsBlind())
		PswOrder(1)
		If PswSeek( __cUserID, .T. )

			aSit		:= {}
			aFilNome	:= {}
			aFilAtu		:= FWArrFilAtu( cEmpresa, cFilAnt )
			If Len( aFilAtu ) > 0
				cTxtAux := IIF(lGestao,STR0047,STR0048)//"Empresa/Unidade/Filial de "##"Filiais de "
				cTitulo := cTxtAux + AllTrim( aFilAtu[6] )
			EndIf

			// Adiciona as filiais que o usuario tem permissão
			For nInc := 1 To Len( aSM0 )
				//DEFINES da SMO encontra-se no arquivo FWCommand.CH
				//Na função FWLoadSM0(), ela retorna na posicao [SM0_USEROK] se esta filial é válida para o user

				//Incluido validação de acesso as filiais, caso o usuário não tenha acesso a todas as filiais do grupo não seta a variável lTodasFil,
				//isso porque os saldos contábeis estavam saindo errados, pois buscavam todas as filiais, mesmo aquelas que o usuário não tem acesso.
				If (aSM0[nInc][SM0_GRPEMP] == cEmpAnt .And. ((ValType(aSM0[nInc][SM0_EMPOK]) == "L" .And. aSM0[nInc][SM0_EMPOK]) .Or. ValType(aSM0[nInc][SM0_EMPOK]) <> "L") .And. ! aSM0[nInc][SM0_USEROK] )
					lUserOk := .F.
				Endif

				If (aSM0[nInc][SM0_GRPEMP] == cEmpAnt .And. ((ValType(aSM0[nInc][SM0_EMPOK]) == "L" .And. aSM0[nInc][SM0_EMPOK]) .Or. ValType(aSM0[nInc][SM0_EMPOK]) <> "L") .And. aSM0[nInc][SM0_USEROK] )

					//Verificacao se as filiais a serem apresentadas serao
					//Apenas as filiais da empresa conrrente (M0_CODFIL)
					If lGestao .and. lFwCompany .and. lSohFilEmp
						//Se for exclusivo para empresa
						If !Empty(cEmpFil)
							lOk := IIf(cEmpFil == Substr(aSM0[nInc][2],1,nTamEmp),.T.,.F.)
							
							//Verifica se as filiais devem pertencer a mesma unidade de negocio da filial corrente
							If lOk .And. lSohFilUn
								//Se for exclusivo para unidade de negocio
								If !Empty(cUnFil)
									lOk := IIf(cUnFil == Substr(aSM0[nInc][2],nTamEmp + 1,nTamUn),.T.,.F.)
								Endif
							Endif
						Elseif !Empty(cUnFil)
							lOk := IIf(cUnFil == Substr(aSM0[nInc][2],nTamEmp + 1,nTamUn),.T.,.F.)	
						ElseIf !(lOk := lTudoComp)
							lOk := (cFilAnt == aSM0[nInc][SM0_CODFIL]) //Traz apenas a filial corrente quando for tudo compatilhado.
						Endif
					Endif
					
					If lOk
						AAdd(aSit, {aSM0[nInc][SM0_CODFIL],aSM0[nInc][SM0_NOMRED],Transform(aSM0[nInc][SM0_CGC],PesqPict("SA1","A1_CGC"))})
						MvParDef += aSM0[nInc][SM0_CODFIL]
						nI++
					Endif

					//ponto de entrada para usuario poder manipular as filiais selecionada
					//por exemplo para um usuario especifico poderia adicionar uma filial que normalmente nao tem acesso
					If lPESetFil
						aSit_Ant := aClone(aSit)
						aSit := ExecBlock("CTSETFIL",.F.,.F.,{aSit,nI})

						If aSit == NIL .Or. Empty(aSit) .Or. !Valtype( "aSit" ) <> "A"
		                	aSit := aClone(aSit_Ant)
						EndIf
                		nI := Len(aSit)
					EndIf
				Endif
			Next
			
			If Len( aSit ) <= 0
				// Se não tem permissão ou ocorreu erro nos dados do usuario, pego a filial corrente.
				Aadd(aSit, aFilAtu[2]+" - "+aFilAtu[7] )
				MvParDef := aFilAtu[2]
				nI++
			EndIf
		EndIf
		If lExibTela
			aFil := {}
			If ExistBlock("ADMSELFIL")	// PE para substituir a AdmOpcoes
				aFil := ExecBlock("ADMSELFIL",.F.,.F.,{cTitulo,aSit,MvParDef,nTamFil})
			ElseIf AdmOpcoes(@MvPar,cTitulo,aSit,MvParDef,,,.F.,nTamFil,nI,.T.,,,,,,,,.T.)  // Chama funcao Adm_Opcoes
				nSit := 1
				For nReg := 1 To len(mvpar) Step nTamFil  // Acumula as filiais num vetor
					If SubSTR(mvpar, nReg, nTamFil) <> Replicate("*",nTamFil)
				 		AADD(aFil, SubSTR(mvpar, nReg, nTamFil) )
					endif
					nSit++
				next
				If Empty(aFil) .And. lHlp
		 	  		Help(" ",1,"ADMFILIAL",,STR0042,1,0)		//"Por favor selecionar pelo menos uma filial"
				EndIF

				If Len(aFil) == Len(aSit) .And. lUserOk
					lTodasFil := .T.
				EndIf
			Endif
		Else
			aFil := aClone(aSit)
		EndIf
	Else
		aFil := {cFilAnt}
	EndIf

	//ponto de entrada para usuario poder manipular as filiais selecionada
	//por exemplo para um usuario especifico poderia adicionar uma filial que normalmente nao tem acesso
	If lExibTela .and. lPEGetFil
		aFil_Ant := aClone(aFil)
		aFil := ExecBlock("CTGETFIL",.F.,.F.,{aFil})
		If aFil == NIL .Or. Empty(aFil)
			aFil := aClone(aFil_Ant)
		EndIf
	EndIf

Else
	Help("  ",1,"ADMFILTOP",,STR0043,1,0) //"Função disponível apenas para ambientes TopConnect"
EndIf

RestArea(aArea)

Return(aFil)

/*
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Funcao    ³AdmOpcoes ³ Autor ³ Totvs                 ³ Data ³03/10/2008³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³Selecao de Opcoes                                           ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Sintaxe   ³<Vide Parametros Formais>								  	  ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³<Vide Parametros Formais>								  	  ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Uso       ³Generico                                                    ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß/*/
Function AdmOpcoes(	uVarRet			,;	//01-Variavel de Retorno
						cTitulo			,;	//2-Titulo da Coluna com as opcoes
						aOpcoes			,;	//3-Opcoes de Escolha (Array de Opcoes)
						cOpcoes			,;	//4-String de Opcoes para Retorno
						nLin1			,;	//5-Nao Utilizado
						nCol1			,;	//6-Nao Utilizado
						l1Elem			,;	//7-Se a Selecao sera de apenas 1 Elemento por vez
						nTam			,;	//8-Tamanho da Chave
						nElemRet		,;	//9-No maximo de elementos na variavel de retorno
						lMultSelect		,;	//10-Inclui Botoes para Selecao de Multiplos Itens
						lComboBox		,;	//11-Se as opcoes serao montadas a partir de ComboBox de Campo ( X3_CBOX )
						cCampo			,;	//12-Qual o Campo para a Montagem do aOpcoes
						lNotOrdena		,;	//13-Nao Permite a Ordenacao
						lNotPesq		,;	//14-Nao Permite a Pesquisa
						lForceRetArr    ,;	//15-Forca o Retorno Como Array
						cF3				,;	//16-Consulta F3
						lVisual			,;  //17-Apenas visualizacao
						lColunada		 ;  //18-Apresenta dados em colunas (Apenas AdmGetFil)
				  )

Local aListBox			:= {}
Local aSvKeys			:= GetKeys()
Local aAdvSize			:= {}
Local aInfoAdvSize		:= {}
Local aObjCoords		:= {}
Local aObjSize			:= {}
Local aButtons			:= {}
Local aX3Box			:= {}

Local bSvF3				:= SetKey( VK_F3  , NIL )
Local bSetF3			:= { || NIL }
Local bSet15			:= { || NIL }
Local bSet24			:= { || NIL }
Local bSetF4			:= { || NIL }
Local bSetF5			:= { || NIL }
Local bSetF6			:= { || NIL }
Local bCapTrc			:= { || NIL }
Local bDlgInit			:= { || NIL }
Local bOrdena			:= { || NIL }
Local bPesquisa			:= { || NIL }

Local cCodOpc			:= ""
Local cDesOpc			:= ""
Local cCodDes			:= ""
Local cPict				:= "@E 999999"
Local cVarQ				:= ""
Local cReplicate		:= ""
Local cTypeRet			:= ""

Local lExistCod			:= .F.
Local lSepInCod			:= .F.

Local nOpcA				:= 0
Local nFor				:= 0
Local nAuxFor			:= 1
Local nOpcoes			:= 0
Local nListBox			:= 0
Local nElemSel			:= 0
Local nInitDesc			:= 1
Local nTamPlus1			:= 0
Local nSize				:= 0

Local oSize
Local a1stRow			:= {}
Local a2ndRow			:= {}
Local a3rdRow			:= {}

Local oDlg
Local oListbox		:= NIL
Local oElemSel      	:= NIL
Local oElemRet		:= NIL
Local oOpcoes			:= NIL
Local oFontNum		:= NIL
Local oFontTit		:= NIL
Local oBtnMarcTod		:= NIL
Local oBtnDesmTod		:= NIL
Local oBtnInverte		:= NIL
Local oGrpOpc			:= NIL
Local oGrpRet			:= NIL
Local oGrpSel			:= NIL

Local uRet				:= NIL
Local uRetF3			:= NIL

DEFAULT uVarRet			:= &( ReadVar() )
DEFAULT cTitulo			:= OemToAnsi( STR0011 )	//"Escolha Padr”es"
DEFAULT aOpcoes			:= {}
DEFAULT cOpcoes			:= ""
DEFAULT l1Elem			:= .F.
DEFAULT lMultSelect 	:= .T.
DEFAULT lComboBox		:= .F.
DEFAULT cCampo			:= ""
DEFAULT lNotOrdena		:= .F.
DEFAULT lNotPesq		:= .F.
DEFAULT lForceRetArr	:= .F.
DEFAULT lVisual			:= .F.
DEFAULT lColunada		:= .F.

Begin Sequence

	uRet				:= uVarRet
	cTypeVarRet			:= ValType( uVarRet )
	cTypeRet			:= IF( lForceRetArr , "A" , ValType( uRet ) )
	lMultSelect 		:= !( l1Elem )
	nSize				:= If(lColunada,20,0)

	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Coloca o Ponteiro do Cursor em Estado de Espera			   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	//CursorWait()

		IF !( lComboBox )
			DEFAULT nTam	:= 1
			nTamPlus1		:= ( nTam + 1 )
			IF ( ( nOpcoes := Len( aOpcoes ) ) > 0 )
				For nFor := 1 To nOpcoes
					If !lColunada
					    IF !Empty( cOpcoes )
						    cCodOpc		:= SubStr( cOpcoes , nAuxFor , nTam )
					    	lExistCod	:= .F.
					    	nInitDesc	:= 1
					    	IF !( " <-> "		== SubStr( aOpcoes[ nFor ] , nTamPlus1 , 5 ) ) .and. ;
					    	   !( " <=> "		== SubStr( aOpcoes[ nFor ] , nTamPlus1 , 5 ) ) .and. ;
		  	   			       !( " <-> "		== SubStr( aOpcoes[ nFor ] , nTam      , 5 ) ) .and. ;
					    	   !( " <=> "		== SubStr( aOpcoes[ nFor ] , nTam      , 5 ) )
					    		IF !( "<->"		== SubStr( aOpcoes[ nFor ] , nTamPlus1 , 3 ) ) .and. ;
					    		   !( "<=>"		== SubStr( aOpcoes[ nFor ] , nTamPlus1 , 3 ) ) .and. ;
					    		   !( " - "		== SubStr( aOpcoes[ nFor ] , nTamPlus1 , 3 ) ) .and. ;
					    		   !( " = "		== SubStr( aOpcoes[ nFor ] , nTamPlus1 , 3 ) ) .and. ;
					    		   !( "<->"		== SubStr( aOpcoes[ nFor ] , nTam      , 3 ) ) .and. ;
					    		   !( "<=>"		== SubStr( aOpcoes[ nFor ] , nTam	   , 3 ) ) .and. ;
					    		   !( " - "		== SubStr( aOpcoes[ nFor ] , nTam	   , 3 ) ) .and. ;
					    		   !( " = "		== SubStr( aOpcoes[ nFor ] , nTam	   , 3 ) )
					    			IF !( "-"	== SubStr( aOpcoes[ nFor ] , nTamPlus1 , 1 ) ) .and. ;
					    			   !( "="	== SubStr( aOpcoes[ nFor ] , nTamPlus1 , 1 ) ) .and. ;
					    			   !( "-"	== SubStr( aOpcoes[ nFor ] , nTam	   , 1 ) ) .and. ;
					    			   !( "="	== SubStr( aOpcoes[ nFor ] , nTam      , 1 ) )
					    				nInitDesc	:= 1
					    				lExistCod	:= .F.
					    			Else
				    					nInitDesc	:= nTamPlus1 /* 1 */
					    				lExistCod	:= .T.
					    			EndIF
					    		Else
					    			IF (;
					    					lSepInCod := (;
															( "<->" $ cCodOpc ) .or. ;
					    									( "<=>" $ cCodOpc ) .or. ;
					    									( " - " $ cCodOpc ) .or. ;
					    									( " = " $ cCodOpc )		 ;
					    							   	  );
										)
					    				nInitDesc	:= nTamPlus1
					    			Else
					    				nInitDesc	:= ( nTamPlus1 + 2 ) /* 123 */
					    			EndIF
					    			lExistCod	:= .T.
					    		EndIF
					    	Else
				    			IF (;
				    					lSepInCod := (;
				    									( " <-> " $ cCodOpc ) .or. ;
				    									( " <=> " $ cCodOpc )	   ;
				    							   );
									)
				    				nInitDesc	:= nTamPlus1
				    			Else
					    			nInitDesc	:= ( nTamPlus1 + 4 ) /* 12345 */
					    		EndIF
					    		lExistCod	:= .T.
					    	EndIF
						    cDesOpc		:= SubStr( aOpcoes[ nFor ] , nInitDesc )
						    cCodDes		:= IF( lExistCod , aOpcoes[ nFor ] , cCodOpc + " - " + cDesOpc )
						    aAdd( aListBox , { .F. , cCodDes , cCodOpc , cDesOpc } )
							nAuxFor := ( ( nFor * nTam ) + 1 )
						Else
							aAdd( aListBox , { .F. , aOpcoes[ nFor ] , aOpcoes[ nFor ] , aOpcoes[ nFor ] } )
						EndIF
						IF (;
						   		( cTypeVarRet == "C" );
						   		.and.;
						   		( aListBox[ nFor , 03 ] $ uVarRet );
						   	)
							aListBox[ nFor , 01 ] := .T.
						EndIF
	        		Else
					    aAdd( aListBox , { .F. , aOpcoes[ nFor,1 ] , aOpcoes[ nFor,2 ], aOpcoes[ nFor,3 ] } )

					Endif

				Next nFor
			Else
				/*
				ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
				³ Restaura o Ponteiro do Cursor                  			   ³
				ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
				//CursorArrow()
				//"N„o existem dados para consulta"###"Escolha Padr”es"
				MsgInfo( OemToAnsi( STR0039 ) , IF( Empty( cTitulo ) , OemToAnsi( STR0011 ) , cTitulo ) )
				Break
			EndIF
		Else
			DEFAULT nTam	:= ( TamSx3( cCampo )[1] )
			aListBox := MontaCombo( cCampo , @cTitulo )
			IF ( ( nOpcoes := Len( aListBox ) ) > 0 )
				For nFor := 1 To nOpcoes
			    	IF (;
			    			( cTypeVarRet == "C" );
			    			.and.;
			    			( aListBox[ nFor , 03 ] $ uVarRet );
			    		)
		    	    	aListBox[ nFor , 01 ] := .T.
		    		EndIF
				Next nFor
			Else
				/*
				ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
				³ Restaura o Ponteiro do Cursor                  			   ³
				ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
				//CursorArrow()
				//"N„o existem dados para consulta"
				MsgInfo( OemToAnsi( STR0039 ) , IF( Empty( cTitulo ) , OemToAnsi( STR0011 ) , cTitulo ) )
			EndIF
		EndIF

		/*
		ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		³ Define o DEFAULT do Maximo de Elementos que Podem ser Retorna³
		³ dos														   ³
		ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
		DEFAULT nElemRet := ( Len( &( ReadVar() ) ) / nTam )

		/*
		ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		³ Define os numeros de Elementos que serao Mostrados		   ³
		ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
		nOpcoes		:= Len( aListbox )
		nElemRet    := Min( nElemRet , nOpcoes )
		nElemRet	:= IF( !( lMultSelect ) , 01 , nElemRet )
		/*
		ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		³ Verifica os Elementos ja Selecionados          			   ³
		ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
		aEval( aListBox , { |x| IF( x[1] , ++nElemSel , NIL ) } )


	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Restaura o Ponteiro do Cursor                  			   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	//CursorArrow()

	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Define Bloco e Botao para a Ordenacao das Opcoes       	   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	IF !( lNotOrdena )
		bOrdena := { || AdmOpcOrd(;
									oListBox	,;
									STR0024		 ; //"Ordenar <F7>..."
								 ),;
					 	SetKey( VK_F7 , bOrdena );
					}
		aAdd(;
				aButtons	,;
								{;
									"SDUORDER"				,;
		   							bOrdena 				,;
		       	   					OemToAnsi( STR0024 )	,;	//"Ordenar <F7>..."
		       	   					OemtoAnsi( STR0026 )	 ;	//"Ordenação"
		           				};
		     )
	EndIF

	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Define Bloco e  Botao para a Pesquisa                   	   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	IF !( lNotPesq )
		bPesquisa := { || aListBox := AdmOpcPsq(;
									oListBox	,;
									STR0025		,; // "Pesquisar <F8>..."
									lNotOrdena  ,;
									cF3			,;
									aX3Box		 ;
								 ),;
					 	SetKey( VK_F8 , bPesquisa );
					}
		aAdd(;
				aButtons	,;
								{;
									"PESQUISA"				,;
		   							bPesquisa				,;
		       	   					OemToAnsi( STR0025 )	,;	//"Pesquisar <F8>..."
		       	   					OemToAnsi( STR0034 )	 ;	//"Pesquisar"
		           				};
		     )
	EndIF

	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Define o Bloco para a CaPexTroca()						   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	bCapTrc := { |cTipo,lMultSelect| ;
										aListBox := AdmexTroca(;
																oListBox:nAt,;
																@aListBox,;
																l1Elem,;
																nOpcoes,;
																nElemRet,;
																@nElemSel,;
																lMultSelect,;
																cTipo;
															),;
										oListBox:nColPos := 1,;
										oListBox:Refresh(),;
										oElemSel:Refresh();
				}

	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Seta a consulta F3                						   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	IF !Empty( cCampo )
		IF !Empty( cF3 )
			bSetF3	:= { || AdmPesqF3( cF3 , cCampo , oListBox ) , SetKey( VK_F3 , bSetF3 ) }
		Else
			aX3Box	:= Sx3Box2Arr( cCampo )
		EndIF
	EndIF

	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Disponibiliza Dialog para Selecao 						   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	DEFINE FONT oFontNum NAME "Arial" SIZE 000,-014 BOLD
	DEFINE FONT oFontTit NAME "Arial" SIZE 000,-011 BOLD

	DEFINE MSDIALOG oDlg FROM 000,000 TO 390,500 TITLE STR0056 OF oMainWnd PIXEL //STR0056 - SELEÇÃO DE FILIAIS

	//Faz o calculo automatico de dimensoes de objetos
	oSize := FwDefSize():New(.T.,,,oDlg)

	oSize:lLateral := .F.
	oSize:lProp	:= .T. // Proporcional

	oSize:AddObject( "1STROW" ,  100, 070, .T., .T. ) // Totalmente dimensionavel
	oSize:AddObject( "2NDROW" ,  100, 010, .T., .T. ) // Totalmente dimensionavel
	oSize:AddObject( "3RDROW" ,  100, 020, .T., .T. ) // Totalmente dimensionavel

	oSize:aMargins := { 3, 3, 3, 3 } // Espaco ao lado dos objetos 0, entre eles 3

	oSize:Process() // Dispara os calculos


	a1stRow :=	{oSize:GetDimension("1STROW","LININI"),;
				oSize:GetDimension("1STROW","COLINI"),;
				oSize:GetDimension("1STROW","XSIZE"),;
				oSize:GetDimension("1STROW","YSIZE")}

	a2ndRow :=	{oSize:GetDimension("2NDROW","LININI"),;
				oSize:GetDimension("2NDROW","COLINI"),;
				oSize:GetDimension("2NDROW","XSIZE"),;
				oSize:GetDimension("2NDROW","YSIZE")}

	a3rdRow :=	{oSize:GetDimension("3RDROW","LININI"),;
				oSize:GetDimension("3RDROW","COLINI"),;
				oSize:GetDimension("3RDROW","LINEND"),;
				oSize:GetDimension("3RDROW","COLEND")}


		If lColunada		//Utilizada pela AdmGetFil com Gestao Corporativa
			@ a1stRow[1],a1stRow[2]	LISTBOX oListBox VAR cVarQ FIELDS HEADER "" , STR0049, STR0050, STR0051 SIZE a1stRow[3],a1stRow[4] ON	DBLCLICK Eval( bCapTrc ) NOSCROLL OF oDlg PIXEL //"Filial", "Nome Filial", "CNPJ"
        Else
			@ a1stRow[1],a1stRow[2]	LISTBOX oListBox VAR cVarQ FIELDS HEADER "" , OemToAnsi(cTitulo)  SIZE a1stRow[3],a1stRow[4] ON	DBLCLICK Eval( bCapTrc ) NOSCROLL OF oDlg PIXEL
		Endif

		oListBox:SetArray( aListBox )
		oListBox:bLine := { || LineLstBox( oListBox , .T. ) }
		oListBox:bWhen := { || !lVisual }

		IF ( lMultSelect ) .AND. !lVisual
			/*
			ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
			³ Define Bloco e o Botao para Marcar Todos    				   ³
			ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
			bSetF4		:= { || Eval( bCapTrc , "M" , lMultSelect ) , SetKey( VK_F4 , bSetF4 ) }
			@ a2ndRow[1] + 002 ,a2ndRow[2] + 000  BUTTON oBtnMarcTod	PROMPT OemToAnsi( STR0012 )		SIZE 75,13.50 OF oDlg	PIXEL ACTION Eval( bSetF4 ) //"Marca Todos - <F4>"

			/*
			ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
			³ Define Bloco e o Botao para Desmarcar Todos    			   ³
			ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
			bSetF5		:= { || Eval( bCapTrc , "D" , lMultSelect ) , SetKey( VK_F5 , bSetF5 ) }
			@ a2ndRow[1] + 002,a2ndRow[2] + 080 BUTTON oBtnDesmTod	PROMPT OemToAnsi( STR0013 )		SIZE 75,13.50 OF oDlg	PIXEL ACTION Eval( bSetF5 ) //"Desmarca Todos - <F5>"

			/*
			ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
			³ Define Bloco e o Botao para Inversao da Selecao			   ³
			ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
			bSetF6		:= { || Eval( bCapTrc , "I" , lMultSelect ) , SetKey( VK_F6 , bSetF6 ) }
			@ a2ndRow[1] + 002,a2ndRow[2] + 160 BUTTON oBtnInverte	PROMPT OemToAnsi( STR0014 ) 	SIZE 75,13.50 OF oDlg	PIXEL ACTION Eval( bSetF6 ) //"Inverte Sele‡„o - <F6>"
		EndIF

		If !lVisual
			/*
			ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
			³ Numero de Elementos para Selecao							   ³
			ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
			@ a3rdRow[1] + 000,a3rdRow[2] + 000 GROUP oGrpOpc TO a3rdRow[3]-5,074.50	OF oDlg LABEL OemtoAnsi(STR0020) PIXEL	//"Nro. Elementos"
			oGrpOpc:oFont := oFontTit
			@ a3rdRow[1] + 010,a3rdRow[2] + 010 SAY oOpcoes VAR Transform( nOpcoes	, cPict )	OF oDlg PIXEL	FONT oFontNum

			/*
			ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
			³ Maximo de Elementos que poderm Ser Selecionados			   ³
			ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
			@ a3rdRow[1] + 000,a3rdRow[2] + 080 GROUP oGrpRet TO a3rdRow[3]-5,152.50	OF oDlg LABEL OemtoAnsi(STR0021) PIXEL	//"M x. Elem. p/ Sele‡„o"
			oGrpRet:oFont := oFontTit
			@ a3rdRow[1] + 010,a3rdRow[2] + 090 SAY oElemRet	VAR Transform( nElemRet	, cPict )	OF oDlg PIXEL	FONT oFontNum

			/*
			ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
			³ Numero de Elementos Selecionados                		   	   ³
			ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
			@ a3rdRow[1] + 000,a3rdRow[2] + 160 GROUP oGrpSel	TO a3rdRow[3]-5,230	OF oDlg LABEL OemtoAnsi(STR0022) PIXEL	//"Elem. Selecionados"
			oGrpSel:oFont := oFontTit
			@ a3rdRow[1] + 010,a3rdRow[2] + 170 SAY oElemSel	VAR Transform( nElemSel	, cPict )	OF oDlg PIXEL	FONT oFontNum
		EndIf

		/*
		ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		³ Define Bloco para a Tecla <CTRL-O>              		   	   ³
		ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	  	bSet15 := { || nOpcA := 1 , GetKeys() , SetKey( VK_F3 , NIL ) , oDlg:End() }

		/*
		ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		³ Define Bloco para a Tecla <CTRL-X>              		   	   ³
		ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
		bSet24 := { || nOpcA := 0 , GetKeys() , SetKey( VK_F3 , NIL ) , oDlg:End() }

		/*
		ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		³ Define Bloco para o Init do Dialog              		   	   ³
		ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
		bDlgInit := { || EnchoiceBar( oDlg , bSet15 , bSet24 , NIL , aButtons ),;
						 IF( lMultSelect ,;
						 		(;
						 		 	SetKey( VK_F3 , bSetF3 ),;
						 		 	SetKey( VK_F4 , bSetF4 ),;
						 		 	SetKey( VK_F5 , bSetF5 ),;
						 		 	SetKey( VK_F6 , bSetF6 );
						 		 ),;
						 		NIL;
						 	),;
						 SetKey( VK_F7 , bOrdena ),;
						 SetKey( VK_F8 , bPesquisa );
					}

	ACTIVATE MSDIALOG oDlg CENTERED ON INIT Eval( bDlgInit )

	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Retorna as Opcoes Selecionadas                  		   	   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	IF ( nOpcA == 1 )
		/*
		ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		³ Coloca o Ponteiro do Cursor em Estado de Espera			   ³
		ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
		//CursorWait()
	    IF ( cTypeRet == "C" )
		    uRet		:= ""
			cReplicate	:= Replicate( "*" , nTam )
		    nListBox := Len( aListBox )
		    For nFor := 1 To nListBox
				IF ( aListBox[ nFor , 01 ] )
					uRet += aListBox[ nFor , IIf(lColunada, 02, 03) ]
		    	ElseIF ( lMultSelect )
		    		uRet += cReplicate
		    	EndIF
		    Next nFor
		ElseIF ( cTypeRet == "A" )
		    uRet	 	:= {}
		    nListBox	:= 0
		    While ( ( nFor := aScan( aListBox , { |x| x[1] } , ++nListBox ) ) > 0 )
		    	nListBox := nFor
				aAdd( uRet , aListBox[ nFor , 03 ] )
		    End While
		EndIF
		/*
		ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
		³ Restaura o Ponteiro do Cursor                  			   ³
		ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
		//CursorArrow()
	EndIF

	/*
	ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
	³ Carrega Variavel com retorno por Referencia     		   	   ³
	ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
	uVarRet := uRet

End Sequence

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Restaura o Estado das Teclas de Atalho          		   	   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
RestKeys( aSvKeys , .T. )
SetKey( VK_F3 , bSvF3 )

Return( ( nOpca == 1 ) )


/*
ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿
³Fun‡„o    ³CaPexTroca	    ³Autor³Marinaldo de Jesus ³ Data ³11/09/2003³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´
³Descri‡„o ³Efetua a Troca da Selecao no ListBox da AdmOpcoes()   		³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Sintaxe   ³<Vide Parametros Formais>									³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Uso       ³AdmOpcoes()                                                 ³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³ Retorno  ³Array (Listbox) Com a(s) opcao(oes) Selecionadas			³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Parametros³< Vide Parametros Formais 									³
ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
Static Function AdmexTroca(	nAt			,;	//Indice do ListBox de AdmOpcoes()
							aArray		,;	//Array do ListBox de AdmOpcoes()
							l1Elem		,;	//Se Selecao apenas de 1 elemento
							nOpcoes		,;	//Numero de Elementos disponiveis para Selecao
							nElemRet	,;	//Numero de Elementos que podem ser Retornados
							nElemSel	,;	//Numero de Elementos Selecionados
							lMultSelect	,;	//Se Trata Multipla Selecao
							cTipo		 ;	//Tipo da Multipla Selecao "M"arca Todos; "D"esmarca Todos; "I"nverte Selecao
						   )

Local nOpcao		:= 0

DEFAULT nAt			:= 1
DEFAULT aArray		:= {}
DEFAULT l1Elem		:= .F.
DEFAULT nOpcoes		:= 0
DEFAULT nElemRet	:= 0
DEFAULT nElemSel	:= 0
DEFAULT lMultSelect := .F.
DEFAULT cTipo		:= "I"

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Coloca o Ponteiro do Cursor em Estado de Espera			   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
CursorWait()
	IF !Empty( aArray )
		IF !( l1Elem )
			IF !( lMultSelect )
				aArray[nAt,1] := !aArray[nAt,1]
				IF !( aArray[nAt,1] )
					--nElemSel
				Else
					++nElemSel
				EndIF
			ElseIF ( lMultSelect )
				IF ( cTipo == "M" )
					nElemSel := 0
					aEval( aArray , { |x,y| aArray[y,1] := IF( ( y <= nElemRet ) , ( ++nElemSel , .T. ) , .F. ) } )
				ElseIF ( cTipo == "D" )
					aEval( aArray , { |x,y| aArray[y,1] := .F. , --nElemSel } )
				ElseIF ( cTipo == "I" )
					nElemSel := 0
					aEval( aArray , { |x,y| IF( aArray[y,1] , aArray[y,1] := .F. , IF( ( ( ++nElemSel ) <= nElemRet ) , aArray[y,1] := .T. , NIL ) ) } )
					nElemSel := Min( nElemSel , nElemRet )
				EndIF
			EndIF
		Else
			For nOpcao := 1 To nOpcoes
				IF ( nOpcao == nAt )
					aArray[ nOpcao , 1 ]	:= .T.
				Else
					aArray[ nOpcao , 1 ]	:= .F.
				EndIF
			Next nOpcao
			nElemSel := 01
		EndIF
	EndIF
/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Restaura o Ponteiro do Cursor                  			   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
CursorArrow()

IF ( nElemSel > nElemRet )
	aArray[nAt,1] := .F.
	nElemSel := nElemRet
	MsgInfo(;
				OemToAnsi( STR0023 ) ,;	//"Excedeu o n£mero de elementos permitidos para sele‡„o"
				OemToAnsi( STR0015 )  ;	//"Aten‡„o"
		    )
ElseIF ( nElemSel < 0 )
	nElemSel := 0
EndIF

Return( aArray )

/*
ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿
³Fun‡„o    ³fOpcPesqF3		³Autor³Marinaldo de Jesus ³ Data ³11/11/2004³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´
³Descri‡„o ³Efetua Pesquisa Via Tecla F3                         		³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Sintaxe   ³<Vide Parametros Formais>									³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Uso       ³AdmOpcoes()                                                 ³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Retorno   ³NIL															³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Parametros³< Vide Parametros Formais 									³
ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
Static Function AdmPesqF3( cF3 , cCampo , oListBox )

Local cAlias
Local lConpad1
Local nAt
Local uRetF3

IF FindFunction( "AliasCpo" )
	cAlias := AliasCpo( cCampo )
	IF (;
			!Empty( cAlias );
			.and.;
			( Select( cAlias ) > 0 );
		)
		lConpad1 := ConPad1( NIL , NIL , NIL , cF3 , NIL , NIL , .F. )
		IF( lConpad1 )
			uRetF3	:= ( cAlias )->( FieldGet( FieldPos( cCampo ) ) )
			nAt		:= aScan( oListBox:aArray , { |x| x[3] == uRetF3 } )
			IF ( nAt > 0 )
				oListBox:nAt := nAt
				oListBox:Refresh()
			Else
				MsgInfo( OemToAnsi( STR0036 ) ) //"c¢digo n„o encontrado"
			EndIF
		EndIF
	EndIF
EndIF

Return( NIL )

/*
ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿
³Fun‡„o    ³AdmOpcOrd	    ³Autor³Marinaldo de Jesus ³ Data ³11/09/2003³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´
³Descri‡„o ³Ordenar as Opcoes em AdmOpcoes                        		³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Sintaxe   ³<Vide Parametros Formais>									³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Uso       ³AdmOpcoes()                                                 ³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Retorno   ³NIL															³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Parametros³< Vide Parametros Formais 									³
ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
Static Function AdmOpcOrd( oListBox , cTitulo )

Local aSvKeys		:= GetKeys()
Local aAdvSize		:= {}
Local aInfoAdvSize	:= {}
Local aObjCoords	:= {}
Local aObjSize		:= {}

Local bSort			:= { || NIL }

Local lbSet15		:= .F.

Local nOpcRad		:= 1

Local oFont			:= NIL
Local oDlg			:= NIL
Local oGroup		:= NIL
Local oRadio		:= NIL

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Monta as Dimensoes dos Objetos         					   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
aAdvSize		:= MsAdvSize( .T. , .T. )
/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Redimensiona                           					   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
aAdvSize[3] -= 25
aAdvSize[4] -= 40
aAdvSize[5] -= 50
aAdvSize[6] -= 20
aAdvSize[7] += 50
aInfoAdvSize	:= { aAdvSize[1] , aAdvSize[2] , aAdvSize[3] , aAdvSize[4] , 0 , 0 }
aAdd( aObjCoords , { 000 , 000 , .T. , .T. } )
aObjSize		:= MsObjSize( aInfoAdvSize , aObjCoords )

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Define o Bloco para a Teclas <CTRL-O>   ( Button OK da Enchoi³
³ ceBar )													   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
bSet15 := { ||	(;
					lbSet15 := .T. ,;
					GetKeys(),;
					oDlg:End();
				  );
			}

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Define o  Bloco  para a Teclas <CTRL-X> ( Button Cancel da En³
³ choiceBar )												   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
bSet24 := { || GetKeys() , oDlg:End() }

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³Monta Dialogo para a selecao do Periodo 					  ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/

Define Font oFont NAME "Arial" SIZE 0,-11 Bold
Define MsDialog oDlg Title OemToAnsi(cTitulo) From aAdvSize[7],000 To aAdvSize[6],aAdvSize[5] Of GetWndDefault() Pixel

@ aObjSize[1,1],(aObjSize[1,2] + 003) Group oGroup To aObjSize[1,3],aObjSize[1,4] Label OemToAnsi(STR0026) Of oDlg Pixel // "Ordenação"
oGroup:oFont:= oFont

@ (aObjSize[1,1] + 010),(aObjSize[1,2] + 005) Say OemToAnsi(STR0027) Size 300 , 010 Of oDlg Pixel Font oFont // "Efetuar a Ordenação por:"
@ (aObjSize[1,1] + 010),(aObjSize[1,2] + 100) Radio oRadio Var nOpcRad Items OemToAnsi(STR0028),;            // "Código da Filial"
																	 		 OemToAnsi(STR0029),;            // "Nome da Empresa"
																	 		 OemToAnsi(STR0030),;            // "Ítem selecionado + Código da Filial"
																	 		 OemToAnsi(STR0031),;            // "Ítem selecionado + Nome da Empresa"
																	 		 OemToAnsi(STR0032),;            // "Ítem não selecionado + Código da Filial"
																	 		 OemToAnsi(STR0033);             // "Ítem não selecionado + Nome da Empresa"
																			 Size 115 , 010 Of oDlg Pixel
oRadio:oFont := oFont

Activate MsDialog oDlg Centered On Init EnchoiceBar(oDlg,bSet15,bSet24)

If (lbSet15)

	Do Case
		Case (nOpcRad == 1)
			bSort := {|x,y| x[2] < y[2]}
		Case (nOpcRad == 2)
			bSort := {|x,y| x[3] < y[3]}
		Case (nOpcRad == 3)
			bSort := {|x,y| (If( x[1],"A","Z") + x[2]) < (If( y[1],"A","Z") + y[2])}
		Case (nOpcRad == 4)
			bSort := {|x,y| (If( x[1],"A","Z") + x[3]) < (If( y[1],"A","Z") + y[3])}
		Case (nOpcRad == 5)
			bSort := {|x,y| (If(!x[1],"A","Z") + x[2]) < (If(!y[1],"A","Z") + y[2])}
		Case (nOpcRad == 6)
			bSort := {|x,y| (If(!x[1],"A","Z") + x[3]) < (If(!y[1],"A","Z") + y[3])}
	End Case

	aSort( oListBox:aArray , NIL , NIL , bSort )
	oListBox:nAt := 1
	oListBox:Refresh()
EndIf

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Restaura as Teclas de Atalho                     	  		  ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
RestKeys( aSvKeys , .T. )

Return( NIL )

/*
ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿
³Fun‡„o    ³AdmOpcPsq	    ³Autor³Marinaldo de Jesus ³ Data ³11/09/2003³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´
³Descri‡„o ³Pesquisar as Opcoes em AdmOpcoes                      		³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Sintaxe   ³<Vide Parametros Formais>									³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Uso       ³AdmOpcoes()                                                 ³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Retorno   ³NIL															³
ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´
³Parametros³< Vide Parametros Formais 									³
ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
Static Function AdmOpcPsq( oListBox , cTitulo , lNotOrdena , cF3 , aX3Box )

Local aSvKeys		:= GetKeys()
Local aAdvSize		:= {}
Local aInfoAdvSize	:= {}
Local aObjCoords	:= {}
Local aObjSize		:= {}
Local aCloneArr		:= {}

Local bSort			:= { || NIL }
Local bAscan		:= { || NIL }
Local bSvF3			:= SetKey( VK_F3  , NIL )

Local cCodigo		:= Space( 20 )
Local cDescri		:= Space( 60 )
Local cMsg			:= ""

Local lbSet15		:= .F.

Local nOpcRad		:= 1
Local nAt			:= 0

Local oFont			:= NIL
Local oDlg			:= NIL
Local oGroup		:= NIL
Local oRadio		:= NIL
Local oCodigo		:= NIL
Local oDescri		:= NIL

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Monta as Dimensoes dos Objetos         					   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
aAdvSize		:= MsAdvSize( .T. , .T. )
/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Redimensiona                           					   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
aAdvSize[3] -= 25
aAdvSize[4] -= 40
aAdvSize[5] -= 50
aAdvSize[6] -= 50
aAdvSize[7] += 20
aInfoAdvSize	:= { aAdvSize[1] , aAdvSize[2] , aAdvSize[3] , aAdvSize[4] , 0 , 0 }
aAdd( aObjCoords , { 000 , 000 , .T. , .T. } )
aObjSize		:= MsObjSize( aInfoAdvSize , aObjCoords )

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Define o Bloco para a Teclas <CTRL-O>   ( Button OK da Enchoi³
³ ceBar )													   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
bSet15 := { ||	(;
					lbSet15 := .T. ,;
					GetKeys(),;
					oDlg:End();
				  );
			}

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Define o  Bloco  para a Teclas <CTRL-X> ( Button Cancel da En³
³ choiceBar )												   ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
bSet24 := { || GetKeys() , oDlg:End() }

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³Monta Dialogo para a selecao do Periodo 					  ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
Define Font oFont Name "Arial" Size 000,-11 Bold
Define MsDialog oDlg Title OemToAnsi(cTitulo) From aAdvSize[7],000 To aAdvSize[6] + 020,aAdvSize[5] Of GetWndDefault() Pixel

@ aObjSize[1,1],(aObjSize[1,2] + 003) Group oGroup To aObjSize[1,3] + 012,aObjSize[1,4] Label OemToAnsi(STR0034) Of oDlg Pixel // "Pesquisa"
oGroup:oFont := oFont

@ (aObjSize[1,1] + 010),(aObjSize[1,2] + 005) Say OemToAnsi(STR0035) Size 300,010 Of oDlg Pixel Font oFont // "Efetuar Pesquisa por:"
@ (aObjSize[1,1] + 010),(aObjSize[1,2] + 100) Radio oRadio Var nOpcRad Items OemToAnsi(STR0028),;          // "Código da Filial"
																	 		 OemToAnsi(STR0029) ;          // "Nome da Empresa"
																			 Size 115,010 Of oDlg Pixel;
																			 ON CHANGE ( cCodigo := Space( 20 ),;
																			 			 cDescri := Space( 60 ),;
																						 Iif( nOpcRad == 1, oCodigo:SetFocus(), oDescri:SetFocus()) )
oRadio:cToolTip := OemToAnsi(STR0038)	//"Ap¢s selecionar pressione a tecla <TAB> para habilitar a digita‡„o"
oRadio:oFont	:= oFont

@ (aObjSize[1,1] + 050),(aObjSize[1,2] + 005) Say OemToAnsi(STR0028 + ":") Size 100 , 010 Of oDlg Pixel Font oFont // "Código da Filial"

If Empty(aX3Box)
	@ (aObjSize[1,1] + 045),(aObjSize[1,2] + 100) MsGet oCodigo Var cCodigo Size 100 , 010 Of oDlg Pixel Font oFont When (nOpcRad == 1)

	If !Empty(cF3)
		oCodigo:cF3 := cF3
	EndIf

Else
	@ (aObjSize[1,1] + 045),(aObjSize[1,2] + 100) ComboBox oCodigo Var cCodigo Items aX3Box	Size 100 , 010 Of oDlg Pixel Font oFont When (nOpcRad == 1)
EndIf

@ (aObjSize[1,1] + 070),(aObjSize[1,2] + 005) Say OemToAnsi(STR0029 + ":") Size 100 , 010 Of oDlg Pixel Font oFont						//"descri‡„o"
@ (aObjSize[1,1] + 065),(aObjSize[1,2] + 100) MsGet oDescri VAR cDescri	Size 190 , 010 Of oDlg Pixel Font oFont When (nOpcRad == 2)

Activate MsDialog oDlg Centered On Init EnchoiceBar(oDlg,bSet15,bSet24)

If ( lbSet15 )
	Do Case
		Case ( nOpcRad == 1 )
			bSort	:= { |x,y| x[2] < y[2] }
			bAscan	:= { |x| x[2] $ cCodigo }
			cMsg	:= STR0036	//"c¢digo n„o encontrado"
		Case ( nOpcRad == 2 )
			bSort 	:= { |x,y| x[3] < y[3] }
			bAscan	:= { |x,y| Upper( AllTrim( cDescri ) ) $ SubStr( Upper( AllTrim( x[3] ) ) , 1 , Len( AllTrim( cDescri ) ) ) }
			cMsg	:= STR0037	//"descri‡„o n„o encontrada"
	End Case
	aCloneArr := aClone( oListBox:aArray )
	IF !( lNotOrdena )
		aSort( oListBox:aArray , NIL , NIL , bSort )
	EndIF
	IF ( ( ( nAt := aScan( oListBox:aArray , bAscan ) ) ) > 0 )
		oListBox:nAt := nAt
		oListBox:Refresh()
	Else
		MsgInfo( OemToAnsi( cMsg ) , cTitulo )
		oListBox:aArray := aClone( aCloneArr )
		oListBox:Refresh()
	EndIF
EndIF

/*
ÚÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ¿
³ Restaura as Teclas de Atalho                     	  		  ³
ÀÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ*/
RestKeys( aSvKeys , .T. )
SetKey( VK_F3 , bSvF3 )

Return oListBox:aArray

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³AdmAbreSM0³ Autor ³ Orizio                ³ Data ³ 22/01/10 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³Retorna um array com as informacoes das filias das empresas ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                   ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/
Function AdmAbreSM0()
	Local aArea			:= SM0->( GetArea() )
	Local aAux			:= {}
	Local aRetSM0		:= {}
	Local lFWLoadSM0	:= FindFunction( "FWLoadSM0" )
	Local lFWCodFilSM0 	:= FindFunction( "FWCodFil" )

	If lFWLoadSM0
		aRetSM0	:= FWLoadSM0()
	Else
		DbSelectArea( "SM0" )
		SM0->( DbGoTop() )
		While SM0->( !Eof() )
			aAux := { 	SM0->M0_CODIGO,;
						IIf( lFWCodFilSM0, FWGETCODFILIAL, SM0->M0_CODFIL ),;
						"",;
						"",;
						"",;
						SM0->M0_NOME,;
						SM0->M0_FILIAL }

			aAdd( aRetSM0, aClone( aAux ) )
			SM0->( DbSkip() )
		End
	EndIf

	RestArea( aArea )
Return aRetSM0

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³AdmCBMoedas³ Autor ³ Marcelo Akama        ³ Data ³ 12/02/10 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³ Retorna uma string com as moedas para ser usada no combo   ³±±
±±³          ³ box no formato 1=Moeda 1;.....;n=Moeda n                   ³±±
±±³          ³ X3_CBOX := "#AdmCBMoedas()"                                ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                   ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/

Function AdmCBMoedas()
Local cMoeda
Local nI
Local cRet:=''

For nI:=1 To 99
	cMoeda := Alltrim( Str( nI ) )
	If GetNewPar("MV_MOEDA"+cMoeda,"x") <> "x"
		cRet := cRet + IIf(empty(cRet),'',';') + cMoeda + '=' + STR0044 + ' ' + cMoeda // "Moeda"
	Else
		nI:= 99
	EndIf
Next

Return( cRet )

/*/
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³AdmCBGener³ Autor ³ Marcelo Akama         ³ Data ³ 12/02/10 ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³ Retorna uma string com os itens da tabela generica         ³±±
±±³          ³ informada para uso no combo box                            ³±±
±±³          ³ X3_CBOX := "#AdmCBGener(cFil,cAlias,cTabela,cIdioma)"      ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ Generico                                                   ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Sintaxe  ³ AdmCBGener(cFil,cAlias,cTabela,cIdioma)                    ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³ cAlias  - Alias ("SX5", "CW0" ou "SN0")                    ³±±
±±³          ³ cTabela - Codigo da tabela                                 ³±±
±±³          ³ cIdioma - Idioma                                           ³±±
±±³          ³               "01" - Portugues                             ³±±
±±³          ³               "02" - Espanhol                              ³±±
±±³          ³               "03" - Ingles                                ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
/*/

Function AdmCBGener(cFil,cAlias,cTabela,cIdioma)
Local cRet:=''
Local aArea:=GetArea()
Local aArea2
Local cPrefix
Local cFldDsc

DEFAULT cIdioma := ""
DEFAULT cAlias  := "SX5"
DEFAULT cTabela := "01"
DEFAULT cFil    := xFilial(cAlias)

If cAlias $ "SX5.CW0.SN0"

	If cAlias=="SX5"
		cPrefix := "X5"
	ElseIf cAlias=="SN0"
		cPrefix := "N0"
		cFldDsc := "_DESC01"
	Else
		cPrefix := cAlias
		cFldDsc := "_DESC01"
	EndIf
	
	If AliasInDic(cAlias)
		dbSelectArea(cAlias)
		aArea2:=(cAlias)->(GetArea())

		(cAlias)->(dbSetOrder(1))
		(cAlias)->( MsSeek( cFil + cTabela ) )

		Do While cFil + cTabela == (cAlias)->( &(cPrefix + "_FILIAL") + &(cPrefix + "_TABELA") )
			cRet := cRet + IIf(empty(cRet),'',';') + Alltrim(&(cPrefix + "_CHAVE")) + '=' + Alltrim(IIf(cAlias=="SX5",X5Descri(),&(cPrefix + cFldDsc)))
			(cAlias)->(dbSkip())
		EndDo

		RestArea(aArea2)
	EndIf
EndIf

RestArea(aArea)

cRet := Alltrim(cRet)

Return( cRet )

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³ADMGETMOED ºAutor  ³Alvaro Camillo Neto º Data ³  23/09/10   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³ Adm_Opcoes de pesquisa por moedas existente no cadastro de º±±
±±º          ³ Moedas                                                     º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±³Retorno   ³ aSelMoed(Contem todas as moedas selecionada)   			  ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±ºUso       ³ SIGACTB, SIGAATF, SIGAFIN                                  º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function ADMGETMOED(lTodasMoed)

Local cTitulo	:= ""
Local MvPar		:= ""
Local MvParDef	:= ""
Local nI 		:= 0
Local aArea 	:= GetArea()
Local nReg	    := 0
Local nSit		:= 0
Local aSit		:= {}
Local aMoed		:= {}
Local lDefTop 	:= IIF( FindFunction("IfDefTopCTB"), IfDefTopCTB(), .F.) // verificar se pode executar query (TOPCONN)
Local nInc		:= 0
Local cMoeda
Local nTamMoed	:= 2

Default lTodasMoed := .F.

If lDefTop
	If !IsBlind()
		aSit		:= {}
		cTitulo := STR0045 //" Selecione as Moedas "

		For nInc :=1 To 99
			cDesc := GetNewPar("MV_MOEDA"+cValtoChar( nInc ),"")
			If !Empty(cDesc)
				cMoeda := StrZero(nInc,2)
			   	Aadd(aSit, cDesc )
			   	MvParDef += cMoeda
				nI++
			Else
				Exit
			EndIf
		Next

		IF AdmOpcoes(@MvPar,cTitulo,aSit,MvParDef,,,.F.,nTamMoed,nI,.T.)  // Chama funcao Adm_Opcoes
			nSit := 1
			For nReg := 1 To len(mvpar) Step nTamMoed  // Acumula as filiais num vetor
				If SubSTR(mvpar, nReg, nTamMoed) <> Replicate("*",nTamMoed)
			 		AADD(aMoed, SubSTR(mvpar, nReg, nTamMoed) )
				endif
				nSit++
			next
			If Empty(aMoed)
	 	  		Help(" ",1,"ADMSMOED",,STR0046,1,0)		//"Por favor selecionar pelo menos uma filial"
			EndIF

			If Len(aMoed) == Len(aSit)
				lTodasFil := .T.
			EndIf
		Endif
	Else
		aMoed := {"01"}
	EndIf
Else
	Help("  ",1,"ADMMOEDTOP",,STR0043,1,0) //"Função disponível apenas para ambientes TopConnect"
EndIf

RestArea(aArea)

Return(aMoed)


/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³ADMTabExc ºAutor  ³Alvaro Camillo Neto º Data ³  08/15/11   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Verifica se a tabela é exclusiva na empresa,unidade ou filialº±±
±±º          ³ substituindo tratamento do !Empty(xFilial(cAlias))         º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ AP                                                        º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function ADMTabExc(cAlias)
Local lGestao		:= AdmGetGest()
Local lExclusivo 	:= .F.
Local aModoComp		:= {}

If lFWCodFil .And. lGestao
	aAdd(aModoComp, Alltrim(FWModeAccess(cAlias,1)) )
	aAdd(aModoComp, Alltrim(FWModeAccess(cAlias,2)) )
	aAdd(aModoComp, Alltrim(FWModeAccess(cAlias,3)) )
	lExclusivo := Ascan(aModoComp, 'E') > 0
Else
	dbSelectArea(cAlias)
	lExclusivo := !Empty(xFilial(cAlias))
EndIf

Return lExclusivo
/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³ADMMoedas ºAutor  ³ Leandro F. Dourado º Data ³  25/10/11   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³ Retorna o numero de moedas configuradas no sistema         º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ GENERICO                                                   º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function ADMMoedas()
Local aGetArea		:= {}
Local aAreaSX3		:= {}
Local __nQtdMoed	:= Nil


__nQtdMoed	:= 5  		// Valor Default
aGetArea	:= GetArea()	// Salva ambiente
aAreaSX3 := SX3->(GetArea())
aAreaSX6 := SX6->(GetArea())

For __nQtdMoed := 5 to 99

	DbSelectArea("SX3")
	dbsetOrder(2)

	If !dbSeek("M2_MOEDA"+Alltrim(Str(__nQtdMoed)))
		__nQtdMoed-- // Caso não encontre o campo a maior moeda configurada é a anterior
		Exit
	EndIf

	DbSelectArea("SX6")
	dbsetOrder(1)

	If !dbSeek("        MV_MOEDA"+Alltrim(Str(__nQtdMoed))) // Os 8 espacos da string preenchem o campo filial, verifique o indice
		__nQtdMoed-- // Caso não encontre o campo a maior moeda configurada é a anterior
		Exit
	EndIf

Next __nQtdMoed

RestArea( aAreaSX3 )
RestArea( aAreaSX6 )
RestArea( aGetArea )


Return __nQtdMoed

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³AdmDiffArrayºAutor  ³Fernando Radu Muscaluº Data ³  31/10/11   º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Compara dois arrays e retorna .t. se ha diferencas entre eles  º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºParametros³aArray1	- Tipo: A       	                                 º±±
±±º          ³aArray2	- Tipo: A		                                     º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ Generico	                                                     º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function AdmDiffArray(aArray1,aArray2)

Local lDiff 	:= .f.

Local nQtdLin1 	:= len(aArray1)
Local nQtdLin2 	:= Len(aArray2)
Local nI		:= 0

If nQtdLin1 == nQtdLin2
	For nI := 1 to nQtdLin1
		If Valtype(aArray1[nI]) == Valtype(aArray2[nI])
			If Valtype(aArray1[nI]) <> "A"
				If aArray1[nI] <> aArray2[nI]
					lDiff := .t.
					Exit
				Endif
			Else
				lDiff := AdmDiffArray(aArray1[nI],aArray2[nI])
				If lDiff
					Exit
				Endif
			Endif
		Else
			lDiff := .t.
			Exit
		Endif
	Next nI
Else
	lDiff := .t.
Endif

Return(lDiff)

/*
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÚÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÂÄÄÄÄÄÄÂÄÄÄÄÄÄÄÄÄÄ¿±±
±±³Fun‡…o    ³ADMSELECFIL³ Autor ³                      ³ Data ³26/07/2013³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄ´±±
±±³Descri‡…o ³Permite a selecao de filiais atraves da AdmGetFil, mas      ³±±
±±³          ³apresentando aviso quanto ao uso da selecao.                ³±±
±±³          ³Permite tambem a interacao a com um grupo de perguntas,     ³±±
±±³          ³carregando ou nao os parametros desse grupo.                ³±±
±±³          ³                                                            ³±±
±±³          ³Retorna um array com as filiais selecionadas ou com somente ³±±
±±³          ³a filial correte, caso o usuario nao selecione alguma.      ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³Parametros³cPerg: codigo do grupo de perguntas. Se informado, os       ³±±
±±³          ³       parametros desse grupo serao carregados.             ³±±
±±³          ³nMVPAR: numero do parametro (pergunta MV_PAR??).Deve ser do ³±±
±±³          ³        tipo numerico, representado a escolha por selecao   ³±±
±±³          ³        de filiais ( = 1) ou nao (= 2). Se informado, a     ³±±
±±³          ³        selecao sera condicionada a esse para parametro ser ³±±
±±³          ³        igual a um; se nenhuma filial for selecionada, o    ³±±
±±³          ³        o valor desse parametro sera alterado para 2.       ³±±
±±³          ³lPergunta: define se serao apresentados os parametros para  ³±±
±±³          ³           digitacao (.T.) ou nao (.F.).                    ³±±
±±³          ³aFilSel: array que recebera os codigos das filiais          ³±±
±±³          ³         selecionadas.                                      ³±±
±±³          ³cAliasBase: alias base para selecao (para a AdmGetFil).     ³±±
±±³          ³lEmpresa: define se serao apresentadas somente as filias    ³±±
±±³          ³          da empresa corrente (.T.)                         ³±±
±±³          ³                                                            ³±±
±±ÃÄÄÄÄÄÄÄÄÄÄÅÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄ´±±
±±³ Uso      ³ AP                                                         ³±±
±±ÀÄÄÄÄÄÄÄÄÄÄÁÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÄÙ±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function AdmSelecFil(cPerg,nMVPAR,lPergunta,aFilSel,cAliasBase,lEmpresa,lUnidNeg,cFilPadrao)
Local lContinua	:= .T.
Local cVar		:= ""

Default cPerg		:= ""
Default nMVPAR		:= 0
Default lPergunta	:= .T.
Default aFilSel		:= {}
Default cAliasBase	:= "SE2"
Default lEmpresa	:= .T.
Default lUnidNeg	:= .F.
Default cFilPadrao	:= cFilAnt

If !Empty(cPerg)
	If nMVPAR > 0
		cVar := "MV_PAR" + StrZero(nMVPAR,2)
	Endif
	If lPergunta
		lContinua := Pergunte(cPerg,.T.)
	Else
		Pergunte(cPerg,.F.)
		lContinua := .T.
	Endif
Endif
If lContinua
	aFilSel := {}
	If nMVPAR == 0 .Or. (&cVar) == 1
		MsgAlert(STR0052 + CRLF + CRLF + STR0053,"")
			//"Selecao de filiais: se nenhuma for selecionada, será considerada somente a filial corrente." ### "Importante: Algumas entidades (como fornecedores, clientes, bancos, etc) podem possuir um mesmo código de identificação para diferentes filiais, sendo que não necessariamente a referência é para a mesma entidade."
		While lContinua
			aFilSel := AdmGetFil(.F.,lEmpresa,cAliasBase,lUnidNeg,.F.)
			If Empty(aFilSel)
				If !MsgYesNo(STR0052 + CRLF + CRLF + STR0054,"")
					//"Seleção de filiais: se nenhuma for selecionada, será considerada somente a filial corrente." ###	"Deseja voltar à seleção de filiais?"
					lContinua := .F.
					If nMVPAR > 0
						(&cVar) := 2
					Endif
					Aadd(aFilSel,cFilPadrao)
				Endif
			Else
				lContinua := .F.
			Endif
		Enddo
	Endif
Endif
Return()

//---------------------------------------------------------------------------------------
/*/{Protheus.doc} GetMoedaCT
Exibe tela de seleção de moedas contábeis (CTO)

@author Pedro Alencar
@since 19/12/13
@version P11.90
@return aRet, Moedas selecionadas
/*/
//---------------------------------------------------------------------------------------
Function GetMoedaCT ()
	Local cRet := ""
	Local aMoedas := {}
	Local nTamMoed := 2
	Local nMaxRet := 5
	Local aRet := {}
	Local nCount := 0
	Local aAreaAnt := CTO->(GetArea())
	Local cRetornos := ""

	DbSelectArea("CTO")
	DbSetOrder(1)
	MsSeek(xFilial("CTO"))

	//Adiciona em um vetor todas as moedas contábeis (Descrição) cadastradas na tabela "CTO"
	While CTO->(!EOF())	.And. CTO->CTO_FILIAL == xFilial("CTO")
		aAdd(aMoedas, CTO->CTO_DESC)
		//Concatena todos os códigos de moedas contábeis em uma string, que será as opções de retorno da função AdmOpcoes
		cRetornos += CTO->CTO_MOEDA

		DbSkip()
	EndDo

	If Len(aMoedas) > 0
		//Abre a tela de seleção com base nos parâmetros informados
		If AdmOpcoes(@cRet,OemToAnsi(STR0055),aMoedas,cRetornos,,,.F.,nTamMoed,nMaxRet,.T.) //'Selecione as moedas contábeis:'
			For nCount := 1 To len(cRet) Step nTamMoed
				//Adiciona no vetor de retorno, somente as moedas selecionadas
				If SubSTR(cRet, nCount, nTamMoed) <> Replicate("*",nTamMoed)
			 		AADD(aRet, SubSTR(cRet, nCount, nTamMoed) )
				Endif
			Next nCount
		Else //Se não for selecionada nenhuma moeda, retorna a moeda local
			aRet := {"01"}
		Endif
	Else //Se não houver moeda contábil cadastrada, retorna a moeda local
		aRet := {"01"}
	Endif

	CTO->(RestArea(aAreaAnt))
Return aRet

/*
ÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜÜ
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
±±ÉÍÍÍÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍËÍÍÍÍÍÍÑÍÍÍÍÍÍÍÍÍÍÍÍÍ»±±
±±ºPrograma  ³AdmGetGest ºAutor  ³Daniel Mendes º Data ³  28/07/14        º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÊÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºDesc.     ³Indica se usa Gestao Corporativa                            º±±
±±ÌÍÍÍÍÍÍÍÍÍÍØÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¹±±
±±ºUso       ³ AdmXFun                                                    º±±
±±ÈÍÍÍÍÍÍÍÍÍÍÏÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍ¼±±
±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±±
ßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßßß
*/
Function AdmGetGest()
Return Iif( lFWCodFil , ( "E" $ FWSM0Layout() .Or. "U" $ FWSM0Layout() ) , .F. )

//---------------------------------------------------------------------------------------
/*/{Protheus.doc} FinLOTab
Retorna o tamanho do LayOut da tabela para ser utilizado na query

@author Rodrigo Oliveira
@since 01/06/22
@version P12.1.33
@return cTamLOut, Tamanho do lay out da tabela
			(Vazio, caso a tabela esteja compartilhada)
/*/
//---------------------------------------------------------------------------------------
Function FinLOTab(cAls As Character) As Character
	Local cTamLOut 	As Character
	Local nLayoutGC	As Numeric
	Local aModAc	As Array

	cTamLOut := ""
	
	If !Empty(cAls)
		If ADMTabExc(cAls)
			If AdmGetGest()
				aModAc	:= {}
				nLayoutGC	:= Len(RTrim(FWSM0LayOut()))

				AAdd( aModAc, FWModeAccess( cAls, 1 ) )
				AAdd( aModAc, FWModeAccess( cAls, 2 ) )
				AAdd( aModAc, FWModeAccess( cAls, 3 ) )

				If aModAc[3] == "E"
					cTamLOut := STR(nLayoutGC)
				ElseIf aModAc[2] == "E"
					cTamLOut := STR(nLayoutGC - Len(FwFilial()))
				Else
					cTamLOut := STR(Len(FwCodEmp()))
				EndIf
				FwFreeArray(aModAc)
				cTamLOut := AllTrim(cTamLOut)
			Else
				cTamLOut := AllTrim(Str(Len(xfilial(cAls))))
			EndIf
		EndIf
	EndIf

Return cTamLOut
