# Investigação do formato RPO — Protheus 12.1.2510

**Data:** 2026-09-19
**Relação com o documento principal:** este documento é o mapeamento
equivalente ao feito em [`docs/rpo-format.md`](./rpo-format.md) (Protheus
12.1.2310), mas para a versão 12.1.2510, instalada em
`~/.shared/protheus/protheus-12.1.2510/` (imagem matriz `protheus:12.1.2510`)
e testada via `~/.shared/protheus/protheus-compile-12.1.2510/` (imagem
`protheus-compile:12.1.2510`, com includes AdvPL completos).

**Não repete o que já foi cross-validado entre as duas versões** — só
documenta o que é específico da 12.1.2510 e reafirma, com evidência própria
desta versão, o que já estava confirmado para a 12.1.2310. Ver o documento
principal para a metodologia completa (Fases 1–8) e o histórico de como
cada técnica foi descoberta.

Escala de confiança igual à do documento principal:
🟢 CONFIRMADO · 🟡 INFERIDO · 🔴 LACUNA

---

## Ambiente

- **Imagem matriz:** `protheus:12.1.2510`
  (`~/.shared/protheus/protheus-12.1.2510/Dockerfile`) — instalada a
  partir do instalador oficial (`installer_lnx.jar`, IzPack) via console
  não-interativo (`install.exp`).
- **Imagem de compilação:** `protheus-compile:12.1.2510`
  (`~/.shared/protheus/protheus-compile-12.1.2510/Dockerfile`) — `FROM`
  a matriz, com os 16.604 arquivos de includes AdvPL/TLPP de
  `~/Documentos/ApostilasP12/include/` copiados para `/protheus12/apo`
  (via symlink de compatibilidade `/protheus12 -> /totvs/protheus1212510/protheus`).
- **Binário do appserver:** `/totvs/protheus1212510/protheus/bin/appserver/appsrvlinux`
  ```
  Build Version:  24.3.1.1
  Architecture:   x86_64
  Build Profile:  RelWithDebInfo
  Build Date:     Oct 3 2025 - 18:04:37
  SVN Revision:   46019
  ```
  (Contraste com a 12.1.2310: `Build Version 20.3.2.14`, `Build
  7.00.210324P`, `Sep 2 2024`, `SVN Revision 42368` — build bem mais
  recente, mesma família de código.)
- **`libaplinux.so`:** 85.732 símbolos dinâmicos totais (12.1.2310 tinha
  84.456) — cresceu ligeiramente entre as versões, estrutura de classes
  praticamente idêntica.
- **Nome do ambiente no `appserver.ini` gerado pelo instalador:**
  `environment` (seção `[environment]`) — usar `-env=environment`, não
  `-env=P12` (esse é o nome usado no container 12.1.2310 mais antigo,
  configurado manualmente/por outro instalador).

---

## Parte 1 — Estrutura de container (Fases 1 e 4 do documento principal)

### 1.1 Header/footer de um `custom.rpo` recém-compilado

```
Fonte de teste: User Function RPOTEST() Return .T. (mesmo teste usado em 12.1.2310)
Tamanho final: 21174 bytes
SelfOffset (offset 0x00-0x03): 438  (0x000001b6)
Nome (offset 0x04-0x13): "custom"
Sentinela (offset 0x18-0x1b): FFFFFFFF
Footer magic: "APNSRM0419" (34 bytes antes do fim, idêntico à 12.1.2310)
```

🟢 **Estrutura de container idêntica à 12.1.2310** — mesmo campo de
auto-referência, mesmo bloco de nome+sentinela, mesmo footer. O parser
Go já existente (`pkg/rpo/rpo.go`) funciona **sem nenhuma alteração**
contra RPOs desta versão (só testado manualmente até aqui — ver "O que
falta" ao final).

### 1.2 Cross-validação com `tttm120.rpo` (RPO base do sistema, 606MB)

```
Nome: "tttm120"
Sentinela: 0xFFFFFF00  (bytes 00 ff ff ff)
Footer magic: "APNSRM0421"
```

🟢 **Bate exatamente** com o par sentinela/magic já catalogado em
`KnownSentinels`/`KnownFooterMagics` (`pkg/rpo/rpo.go`) a partir do
`tttm120.rpo` de 379MB da 12.1.2310 — **mesmo par, em duas versões
diferentes do produto, em dois arquivos de tamanhos bem diferentes
(379MB vs 606MB)**. Confirma que a tabela de sentinelas/magics não é
coincidência de uma instalação específica, é convenção real e estável do
formato.

---

## Parte 2 — Mecanismo de criptografia (Fase 8 do documento principal)

Já documentado com evidência cruzada das duas versões na Fase 8 do
documento principal. Resumo específico desta versão: os símbolos
`tApoFile::Encrypt/Decrypt/ReadIndex`, `tCryptoEVP::Encrypt/Decrypt/
SetKey` e `tAESModeCBC::keyGen` existem nesta build também, em endereços
diferentes (esperado, build diferente):

```
tApoFile::ReadIndex                                    @ 0x25e5490 (5122 bytes)
tApoFile::Decrypt(tApoReg&)                            @ 0x25dd270 (618 bytes)
tApoFile::Encrypt(tApoReg&)                            @ 0x25dda20 (370 bytes)
tCryptoEVP::Encrypt(int,int,int,char*,int,tAutoChar&,int&) @ 0x26604f0 (4333 bytes)
tCryptoEVP::Decrypt(int,int,int,char*,int,tAutoChar&,int&) @ 0x26615e0 (4893 bytes)
tCryptoEVP::SetKey(char const*,int,char const*,int,char const*) @ 0x265cbe0 (805 bytes)
tAESModeCBC::keyGen()                                  @ 0x265bbc0 (229 bytes)
tAppMap::save(tPublicEnv*)                             @ 0x1ab4cf0 (4715 bytes)
tAppMap::GetApoCount()                                 @ 0x1aa2770 (66 bytes)
tAppMap::GetFuncName(int)                               @ 0x1aa55f0 (40 bytes)
tAppMap::WriteApo(...)                                  @ 0x1aa2690 (138 bytes)
```

Rastreamento ao vivo (mesma técnica da Fase 8) confirmou, **nesta
versão**, o mesmo padrão: chave/IV de 16 bytes capturados durante
`SetKey`, plaintext de entrada em `Encrypt` começando com magic zlib
`78 9c`, e nome do fonte em texto puro antes de cifrar
(`"RPOTESTCR.PRW"`). Ver Fase 8 do documento principal para os bytes
exatos capturados desta versão (a seção já cobre ambas).

---

## Parte 3 — Extração ao vivo de funções (Fase 6 do documento principal)

Mesma API (`tAppMap::GetApoCount()`/`GetFuncName(int)` chamadas por
endereço via `gdb`), mas com o ponto de interceptação corrigido — ver
"Bug da extração multi-função" abaixo. Reproduzido nesta versão com
fixture real de 3 funções:

```advpl
User Function RPOMUL1()
Return .T.

User Function RPOMUL2()
Return .T.

User Function RPOMUL3()
Local nX := 1
Return nX
```

Resultado (`tools/rpo-live-inspect/extract_rpo.py`, versão corrigida):
```
[hit#1] this=0x3c3d0d30 count=14 com_nome=3
  U_RPOMUL1
  U_RPOMUL2
  U_RPOMUL3
```

🟢 **Técnica confirmada funcionando nesta versão, extração completa** —
todas as 3 funções do fonte foram capturadas corretamente (`count=14`
inclui outros apoios internos do mapa, além das 3 funções do usuário).

### Bug da extração multi-função — investigado e corrigido (2026-09-19)

A primeira tentativa (`tAppMap::save(tPublicEnv*)` como ponto de
interceptação) só capturava a 1ª função de um fonte multi-função
(`count=1`, só `U_RPOMUL1`). Duas hipóteses testadas:

1. **"Acumular o maior count entre múltiplos hits de save()"** —
   REFUTADA: `save()` só dispara **uma única vez** no processo inteiro
   (confirmado, só `[hit#1]` aparece), então não há hits posteriores para
   acumular.
2. **"Esperar `save()` terminar com `gdb.execute("finish")` antes de
   consultar o mapa"** — REFUTADA: falha com `Cannot execute this
   command while the selected thread is running` (o appserver é
   multi-thread; `finish` em modo all-stop não é seguro nesse ponto).
3. **"Trocar para `tAppMap::WriteApo(char const*, ...)`, que recebe o
   nome como argumento direto"** — REFUTADA: a granularidade de
   `WriteApo` é por **recurso/arquivo gravado no RPO**, não por função —
   um hit para o nome do fonte (`"RPOMUL...PRW"`) e vários hits para
   mapas internos do SIGA (`sigainit.map`, `siga.map`, `sigacls.map`,
   etc.), nenhuma das 3 funções do usuário aparece nesse nível.

**Fix real**: trocar o ponto de interceptação para
`tAppMap::EndBuild(tPublicEnv*)` — símbolo separado
(`_ZN7tAppMap8EndBuildEP10tPublicEnv`) que roda **depois** que todas as
funções do fonte já foram registradas no mapa, ao contrário de `save()`
(interceptado na entrada, disparado cedo demais). Com esse ponto,
`GetApoCount()`/`GetFuncName(i)` no mesmo `this` retornam o snapshot
completo, sem precisar de `finish` nem de múltiplos hits.

---

## O que falta (em relação à paridade completa com o documento principal)

- **Rastreamento completo de `write()`/`lseek64()`** (Fase 4.4) não foi
  refeito nesta versão — a Parte 1.1 acima usou só a inspeção estática do
  arquivo final (header/footer/offset), que já foi suficiente para
  confirmar que a estrutura é idêntica. Não há razão pra esperar
  diferença (mesmo `tAppMap`/`tRPOAdvpl`, mesmos endereços relativos de
  campo), mas não foi **provado byte a byte** para esta versão como foi
  para a 12.1.2310.
- ~~Extração completa de todas as funções de um fonte multi-função~~ —
  **resolvido em 2026-09-19**, ver Parte 3.
- **`pkg/rpo` (Go)** foi testado manualmente contra o formato desta
  versão via inspeção Python, mas **não há teste automatizado**
  (`pkg/rpo/rpo_test.go`) usando um RPO real da 12.1.2510 — os testes
  atuais só cobrem fixtures da 12.1.2310. Reprodutível: copiar um
  `custom.rpo` desta versão para as fixtures de teste, seguindo o mesmo
  padrão de `TestParseRealRPOs`.

## Conclusão

A arquitetura do formato RPO é **a mesma entre 12.1.2310 e 12.1.2510** —
container idêntico (header/sentinela/footer), mesmo mecanismo de
criptografia (zlib + AES-128-CBC via `tCryptoEVP`, chave efêmera por
sessão), mesma API de extração ao vivo (`tAppMap::GetApoCount`/
`GetFuncName`). O que muda entre versões são só os **endereços**
absolutos dos símbolos (esperado, build diferente) — qualquer script que
dependa de endereço bruto precisa redescobrir os símbolos via
`readelf --dyn-syms | c++filt` para cada nova versão, exatamente como já
estava documentado.
