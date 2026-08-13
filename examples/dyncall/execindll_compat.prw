// Wrapper de compatibilidade: expõe a API legada TDN de carga de DLL do
// SmartClient (ExecInDLLOpen/ExecInDLLRun/ExeDLLRun2/ExeDLLRun3/
// ExecInDLLClose) sobre o tRunDll (DynCall) nativo do AdvPP, para rodar
// scripts AdvPL legados sem SmartClient (AppServer, job, standalone).
//
// A DLL/SO alvo continua precisando exportar `ExecInClientDLL` com uma das
// duas assinaturas documentadas na TDN:
//   void ExecInClientDLL(int idCommand, char* buffParam, char* buffOutput, int buffLen)   -- ExecInDLLRun/ExeDLLRun2
//   int  ExecInClientDLL(int nParamHdl, char* cParameters, int nSizeParam, char* cBuffer, int nSizeBuffer) -- ExeDLLRun3
//
// Diferença de sinal em relação ao original: o handle retornado por
// ExecInDLLOpen aqui é o objeto tRunDll (referência), não um inteiro — mas
// -1 continua sinalizando falha, então `If hHdl == -1` no código legado
// continua funcionando sem alteração.

#include "tlpp-core.th"

Function ExecInDLLOpen(cDllName As Character) As Object
    Local oDll := TRunDll():New(cDllName) As Object

    If oDll:GetLastError() != 0
        Return -1
    EndIf
Return oDll

// Assinatura DynCall "VIAPI": void ret, int idCommand, char* buffParam
// (string de entrada, letra 'A'), char* buffOutput (ponteiro alocado por
// nós via NewObj — só assim dá pra ler de volta com StrCpy), int buffLen.
Function ExecInDLLRun(hHdl As Object, nOpc As Numeric, cParam As Character) As Character
    Local oBuf := hHdl:NewObj(256) As Object
    Local lOk As Logical

    lOk := hHdl:CallFunction("ExecInClientDLL", "VIAPI", Nil, nOpc, cParam, oBuf, 256)
    If !lOk
        Return ""
    EndIf
Return hHdl:StrCpy(oBuf, 256)

Function ExeDLLRun2(hHdl As Object, nOpc As Numeric, cParam As Character, nBuffLen As Numeric) As Character
    Local oBuf := hHdl:NewObj(nBuffLen) As Object
    Local lOk As Logical

    lOk := hHdl:CallFunction("ExecInClientDLL", "VIAPI", Nil, nOpc, cParam, oBuf, nBuffLen)
    If !lOk
        Return ""
    EndIf
Return hHdl:StrCpy(oBuf, nBuffLen)

// Assinatura DynCall "IIAIPI": a DLL retorna um int real (status/tamanho),
// mas esse retorno escalar continua inacessível aqui — mesma limitação de
// @var que afeta todo CallFunction/CallMethod deste VM (só ponteiro é
// observável). Só o conteúdo do buffer de saída é recuperável.
Function ExeDLLRun3(hHdl As Object, cParam As Character, nBuffLen As Numeric) As Character
    Local oBuf := hHdl:NewObj(nBuffLen) As Object
    Local lOk As Logical

    lOk := hHdl:CallFunction("ExecInClientDLL", "IIAIPI", Nil, 0, cParam, Len(cParam), oBuf, nBuffLen)
    If !lOk
        Return ""
    EndIf
Return hHdl:StrCpy(oBuf, nBuffLen)

Function ExecInDLLClose(hHdl As Object) As Logical
Return hHdl:Free()
