/*/{Protheus.doc} GetSx3Cache
    Busca propriedade de campo no dicionário SX3 com cache interno
    @type Function
    @author Peder Munksgaard
    @since 21/09/2026
    @param cCampo Character - Nome do campo no dicionário
    @param cProperty Character - Propriedade a buscar (TM_NAME, TM_TYPE, etc)
    @return Character - Valor da propriedade ou string vazia
/*/
User Function GetSx3Cache(cCampo, cProperty)
    Local cAlias   := "SX3"
    Local cValor   := ""
    Local lEncontr := .F.
    
    // Verificar se o alias está aberto
    If FwAliasInDic(cAlias)
        Select(cAlias)
        
        // Construir chave de busca: filial + nome do campo
        Local cChave := FWxFilial(cAlias) + AllTrim(cCampo)
        
        // Posicionar no registro
        If DBSeek(cChave)
            // Verificar se encontrou
            If !Eof() And !Bofer()
                // Buscar posição do campo na estrutura
                Local nPos := 0
                
                // Mapear propriedades comuns
                If Upper(cProperty) == "TM_NAME"
                    nPos := FieldPos("A3_NOME", cAlias)
                Else If Upper(cProperty) == "TM_TYPE"
                    nPos := FieldPos("A3_TIPO", cAlias)
                Else If Upper(cProperty) == "TM_SIZE"
                    nPos := FieldPos("A3_SIZE", cAlias)
                Else If Upper(cProperty) == "TM_DEC"
                    nPos := FieldPos("A3_DEC", cAlias)
                Else If Upper(cProperty) == "TM_DEFAULT"
                    nPos := FieldPos("A3_DEFAULT", cAlias)
                Else If Upper(cProperty) == "TM_VISUAL"
                    nPos := FieldPos("A3_VISUAL", cAlias)
                Else If Upper(cProperty) == "TM_GRID"
                    nPos := FieldPos("A3_GRID", cAlias)
                Else If Upper(cProperty) == "TM_VALID"
                    nPos := FieldPos("A3_VALID", cAlias)
                Else If Upper(cProperty) == "TM_ID"
                    nPos := FieldPos("A3_ID", cAlias)
                Else
                    // Tentar buscar diretamente pelo nome da propriedade
                    nPos := FieldPos(cProperty, cAlias)
                EndIf
                
                // Se encontrou a posição, retornar valor
                If nPos > 0
                    cValor := AllTrim(FieldGet(nPos, cAlias))
                    lEncontr := .T.
                EndIf
            EndIf
        EndIf
    Else
        // Alias não está aberto, tentar abrir
        DbUseArea(, "TOPCONN", RetSqlName("SX3"), cAlias, .T.)
        If Select(cAlias) > 0
            // Recursivamente chamar novamente
            cValor := GetSx3Cache(cCampo, cProperty)
        EndIf
    EndIf
    
Return cValor
