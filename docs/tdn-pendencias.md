# Pendências reais (functions TDN sem native no AdvPP) — 2026-08-11

Método: cruzamento das 532 folhas Functions/ do mirror (H1 real) contra as 688
natives registradas via NewVM (dump runtime). Excluídos stubs confirmados
(ConErr, TCConType) e sem-spec (ProcessMessages).

## Ambiente/Funcoes-genericas (3) — Task 32
- CmpBuildStr, GetBuild, GetEndPoint

## Banco-de-Dados/Funcoes-genericas (37) — Task 33
- DBChangeAlias, DBClearAllFilter, DBClearIndex, DBCloseAll, DBCommitAll,
  DBCreate, DBFieldInfo, DBFilter, DBFilterCB, DBGetActFld, DBGoTo, DBInInsert,
  DBInfo, FLock, Field, FieldBlock, FieldWBlock, Found, GetDBExtension, Header,
  IndexKey, IndexOrd, LastRec, NetErr, OrdBagName, OrdCreate, OrdDescend, OrdKey,
  OrdListAdd, OrdName, OrdNumber, OrdSetFocus, RDDName, RDDSetDefault, RLock,
  RealRDD, RecSize

## Seguranca/Criptografia (1) — Task 34
- SMIMESign

TOTAL: 41
