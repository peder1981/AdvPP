# Pendências reais (functions TDN sem native no AdvPP) — 2026-08-11

Método: cruzamento das folhas Functions/ do mirror (H1 real) contra as natives
registradas via NewVM (dump runtime). Excluídos stubs confirmados
(ConErr, TCConType) e sem-spec (ProcessMessages).

## Status: 0 pendências do lote original (tasks 32/33a/33b/33c/34) — TODAS implementadas

As 41 funções abaixo foram implementadas (commits 83d1bbe, 072d863, 542fcff,
a7d9ebf, a8bf70b; fixes de review em 45375e3). Runtime atual: 729 natives.

- ~~CmpBuildStr, GetBuild, GetEndPoint~~ ✅ (Task 32)
- ~~DBChangeAlias, DBClearAllFilter, DBClearIndex, DBCloseAll, DBCommitAll,
  DBCreate, DBFieldInfo, DBFilter, DBFilterCB, DBGetActFld, DBGoTo, DBInInsert,
  DBInfo, FLock, Field, FieldBlock, FieldWBlock, Found, GetDBExtension, Header,
  IndexKey, IndexOrd, LastRec, NetErr, OrdBagName, OrdCreate, OrdDescend, OrdKey,
  OrdListAdd, OrdName, OrdNumber, OrdSetFocus, RDDName, RDDSetDefault, RLock,
  RealRDD, RecSize~~ ✅ (Task 33)
- ~~SMIMESign~~ ✅ (Task 34)

TOTAL: 41 ✅ implementadas

## Backlog natural (fora do escopo das tasks do lote): 168 funções TDN ainda sem native

Cruzamento completo (2026-08-11): 710 folhas Functions/ vs 729 natives
(535 cobertas; 7 páginas não-função excluídas; 168 reais sem native).
Implementar sob demanda / conforme brief das próximas tasks:

- **Ambiente/Funcoes-genericas (15)**: GetEnvHost, GetRemoteType, GetSrvVersion,
  GetWebJob, IsPlugin, IsPrinter2, IsSecure, IsSrv64, IsSrvBigE, SrvDisplay,
  ThreadCount, ThreadID, __Quit, __SetPicture
- **Interface-HTTP (20)**: HTTPCTDisp, HTTPCTLen, HTTPCTType, HTTPExitProc,
  HTTPFreeSession, HTTPGetPart, HTTPIsAPW, HTTPIsConnected, HTTPLeaveSession,
  HTTPLogonUser, HTTPOtherContent, HTTPPostXml, HTTPPragma, HTTPRCTDisp,
  HTTPRCTLen, HTTPRCTType, HTTPSend, HTTPSetPart, HttpCache, HttpCountSession
- **Componentes-de-interface-visual (45)**: AppBringToFront, CalcFieldSize,
  ChkBmpRlt, CreateSession, CursorArrow, CursorWait, ExecInClient, MSCalculator,
  MessageBox, MsgNoYes, MsgRetryCal, MsgRun, PtGetTheme, PtSetAcento, PtSetTheme,
  SendToFore, SetDefCaption, SetDefFont, SetFlatControls, SetFocus, SetRmtDate,
  SetTransparentColor, SetWndDefault, ShowHelpCpo, ShowHelpDlg, AddCSSRule,
  AddFontAlias, Beep, CSSDictAdd, GetFocus, GetFontList, GetFontPixWidths,
  GetHeightFont, GetScreenRes, GetSenhAp, GetStringPixSize, GetWndDefault,
  IntIncProc, PtGetSessions, PtKillSession, PtRunInSession, SetCSS, SetKey,
  SetKeyBlock
- **Controle-de-impressao (18)**: DevOut, DevOutPict, DevPos, FechaRel,
  GetConnStatus, GetImpInf, InitPrint, PreparePrint, PrintOut, PrnFlush, QOut,
  QQOut, RmvToken, SetPrc, SndToPrnWin, _PCol, _PRow, __Eject
- **Tratamento-de-XML (8)**: XmlChildEx, XmlCloneNode, XmlDelNode, XmlGetChild,
  XmlGetParent, XmlNewNode, XmlNode2Arr, XmlSVldSch
- **SAML (8)**: getSAMLID, getSAMLSvc, reloadSAML, saveIDPXML, setIDPConf,
  setSAMLID, setSAMLSvc, setSPCert
- **Manipulacao-de-arquivos-discos-IO (6)**: FT_FGoTop, FT_FLastRec, FT_FReadLn,
  FT_FRecno, FT_FSkip, MemoLine, SplitPath
- **Web-Services (4)**: WSClassNew, WSDL2Parser, WSDLParser, WSDescData
- **Verificacao-dos-tipos (5)**: ClearVarSetGet, ContType, VarRef, VarSetGet,
  VarUnref
- **Manipulacao-do-arquivo-INI (4)**: GetProfInt, GetProfString, WritePProString,
  WriteProfString
- **Manipulacao-de-memoria (4)**: __ClearRmt, __ListRmt, __LoadRmt, __SaveRmt
- **Seguranca/Criptografia (6)**: EVPPrivSign, EVPPrivVery, MsCRC32, MsCRC32Str,
  GetSslObj, SetSslObj
- **Outros (24)**: TCConType(removido p/ stub — contagem já ajustada), Dbf,
  OrdBagExt, CTUpdateIntName, ctreeDelIdxs, ctreeDelInt, SocketConn,
  GetCredential, GetUserFromSID, JobInfo, KillApp, KillUser, SysRefresh,
  setFinishAppHandler, __HEXTODEC, Resource2File, AttlsMemberOf, DelClassIntf,
  GetParentTree, HMDel, ClearGlbValue, MemGlbSize, TimeGlbValue, GlbLock,
  GlbUnlock, MailVersion
