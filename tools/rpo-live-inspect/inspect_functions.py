"""
rpo-live-inspect: extrai a lista de funções compiladas de um RPO real,
consultando a MEMÓRIA de um appsrvlinux (binário real, licenciado, da
TOTVS) enquanto ele carrega esse RPO — em vez de tentar decifrar o
formato binário do arquivo em disco (não decifrado, ver ../../docs/rpo-format.md).

## O que isto É

Um script gdb (Python) que:
1. Sobe o appsrvlinux real apontando para o custom.rpo de interesse.
2. Intercepta `tAppMap::save(tPublicEnv*)` — chamado durante o próprio
   fluxo de gravação do RPO, com o objeto tAppMap totalmente carregado.
3. Chama diretamente, por endereço, os métodos internos (não documentados,
   não exportados como API pública) `tAppMap::GetApoCount()` e
   `tAppMap::GetFuncName(int)` no objeto capturado — o appserver JÁ
   decodificou o conteúdo do RPO para conseguir executá-lo; este script só
   pergunta a ele o que já sabe, sem tocar no formato do arquivo em disco.

Resultado real, verificado contra um RPO de produção (Conciliador BLU,
289KB): 875 recursos totais, 33 com nome de função populado (o resto são
classes/outros tipos, para os quais GetFuncName devolve vazio por design).

## O que isto NÃO é

- **Não é uma feature do compilador AdvPP.** Não lê o `.rpo` sozinho, não
  funciona sem um `appsrvlinux` real, licenciado, da versão exata usada
  aqui (build 7.00.210324P) rodando sob gdb.
- **Não decifra o RPO.** Não avança em nada a leitura offline do arquivo —
  é uma pergunta feita a um processo vivo, não uma leitura de disco.
- **É frágil por construção.** Os nomes mangled (`_ZN7tAppMap...`) e os
  endereços das funções mudam a cada build do appserver (mesmo um patch
  menor pode invalidar isto). Cada versão de appserver exigiria
  redescobrir os símbolos via `readelf --dyn-syms | c++filt` antes de usar
  este script — não é portável entre ambientes/versões sem revalidação.
- **Não substitui o TDS (tds-vscode)**, que faz a mesma coisa de forma
  suportada e oficial, falando com o appserver via protocolo próprio
  (através de um language-server fechado, `advpls`) em vez de gdb.

## Quando usar

Só quando você já tem um ambiente Protheus real (o mesmo tipo usado pela
skill `compile-protheus`) e precisa, pontualmente, extrair a lista de
funções de um `.rpo` sem subir o VS Code + TDS. Trate como ferramenta de
investigação/depuração, não como parte confiável de um pipeline
automatizado.

## Uso

```
docker cp <seu.rpo> protheus-compile:/protheus12/apo/custom.rpo
docker cp inspect_functions.py protheus-compile:/tmp/inspect_functions.py
docker exec protheus-compile bash -c '
  cd /protheus12/bin/appserver &&
  export LD_LIBRARY_PATH=.:$LD_LIBRARY_PATH &&
  gdb -q -batch -x /tmp/inspect_functions.py ./appsrvlinux
'
# saída em /tmp/rpo_functions.log dentro do container
docker cp protheus-compile:/tmp/rpo_functions.log .
```

Requer um arquivo fonte trivial em /protheus12/apo/ para disparar o fluxo
de compilação (o appserver só entra em modo -compile pedindo um fonte;
qualquer .prw de uma linha serve, o RPO existente é carregado normalmente
antes de compilar o novo fonte). Se os símbolos mangled abaixo não
existirem no seu build, rode primeiro:

```
readelf --dyn-syms -W /protheus12/bin/appserver/libaplinux.so | c++filt | \
  grep -E 'tAppMap::(GetApoCount|GetFuncName)\('
```

e ajuste GETAPOCOUNT_SYM/GETFUNCNAME_SYM abaixo para o que aparecer.
"""

import gdb

GETAPOCOUNT_SYM = "_ZN7tAppMap11GetApoCountEv"
GETFUNCNAME_SYM = "_ZN7tAppMap11GetFuncNameEi"

LOG_PATH = "/tmp/rpo_functions.log"


class SaveBP(gdb.Breakpoint):
    def stop(self):
        this = int(gdb.parse_and_eval("$rdi"))
        with open(LOG_PATH, "a") as log:
            log.write(f"tAppMap::save this=0x{this:x}\n")
            try:
                count = int(
                    gdb.parse_and_eval(
                        f"((int(*)(void*))&{GETAPOCOUNT_SYM})((void*)0x{this:x})"
                    )
                )
            except gdb.error as e:
                log.write(f"GetApoCount ERROR: {e}\n")
                return False
            log.write(f"GetApoCount() = {count}\n")

            for i in range(count):
                try:
                    name = gdb.parse_and_eval(
                        f"((char*(*)(void*,int))&{GETFUNCNAME_SYM})"
                        f"((void*)0x{this:x}, {i})"
                    )
                    s = name.string() if name and int(name) != 0 else ""
                except gdb.error as e:
                    s = f"<erro: {e}>"
                if s:
                    log.write(f"  [{i}] {s}\n")
        return False  # nunca pausa de fato — só observa e segue


gdb.execute("set breakpoint pending on")
SaveBP("tAppMap::save(tPublicEnv*)")
