"""
extract_rpo.py: Extrai a lista de funções/objetos de um RPO Protheus
consultando a MEMÓRIA de um appsrvlinux real durante uma compilação —
o gravador do RPO só roda no fluxo de ESCRITA (confirmado em
docs/rpo-format.md, Fase 4.3), então esta técnica sempre passa por um
`-compile` real (mesmo de um fonte trivial) para forçar o appserver a
carregar e regravar o RPO existente, expondo tanto as funções antigas
quanto a nova.

BUGS CORRIGIDOS (2026-09-19):
1. A versão original nunca chamava `gdb.execute("run ...")` de fato — só
   fazia `continue` num loop, que falha silenciosamente se o processo
   nunca foi iniciado.
2. Primeira reescrita hookava `tAppMap::save(tPublicEnv*)`, que dispara
   UMA VEZ só, ANTES de todas as funções do fonte serem registradas
   (confirmado: fonte com 3 funções só mostrava a 1ª). Tentar
   `gdb.execute("finish")` para esperar o retorno falhou com
   "Cannot execute this command while the selected thread is running"
   (appserver é multi-thread; finish em all-stop não é seguro aqui).
3. Tentativa intermediária: trocar o ponto de interceptação para
   `tAppMap::WriteApo(char const*, ...)`, que dispara uma vez por
   objeto — mas testado e REFUTADO: a granularidade de WriteApo é por
   **recurso/arquivo-fonte** (`"RPOMULTI.PRW"`) mais uma série de mapas
   internos (`sigainit.map`, `siga.map`, `sigacls.map`, etc — estruturas
   de runtime do SIGA, não funções do usuário). Nenhuma das 3 funções
   (`U_RPOMUL1/2/3`) aparece nesse nível; são registradas em um mapa
   separado, o mesmo consultado por `GetApoCount()`/`GetFuncName(i)`.
4. Fix definitivo: manter `GetApoCount()`/`GetFuncName(i)` (a API certa,
   confirmada por dar `U_RPOMUL1` de nome real), mas trocar o ponto de
   observação de `tAppMap::save()` (chamado só 1x, cedo demais — só a
   1ª função já estava registrada) para `tAppMap::EndBuild(tPublicEnv*)`,
   que faz mais sentido semântico como o ponto pós-todas-as-funções do
   ciclo de build antes da gravação final.

Uso (note o `--args` do PRÓPRIO gdb — é isso que faz `run`, dentro do
script, reutilizar os argumentos do appsrvlinux sem o script precisar
recebê-los por fora; `sys.argv` do Python embutido no gdb não reflete de
forma confiável o que vem depois do nome do script):

  docker cp extract_rpo.py <container>:/tmp/extract_rpo.py
  docker exec <container> bash -c '
    cd <dir-do-appserver> &&
    export LD_LIBRARY_PATH=. &&
    gdb -q -batch -x /tmp/extract_rpo.py \
      --args ./appsrvlinux -compile -env=<ENV> -files=<FONTE.prw> -includes=<DIR>
  '
  docker cp <container>:/tmp/rpo_extract.log.json .

Qualquer fonte trivial serve, mesmo já existente no projeto — o objetivo
é só disparar o ciclo de save() do RPO já presente em -files/-includes,
expondo tanto as funções que já estavam no RPO quanto a nova.
"""

import gdb
import json

LOG_PATH = "/tmp/rpo_extract.log"

# Símbolos mangled — descobertos via:
#   readelf --dyn-syms -W libaplinux.so | c++filt | grep "tAppMap::"
# Mudam de endereço (não de nome) entre builds; se o nome mangled mudar,
# reconfirme com o comando acima.
SYM_END_BUILD = "_ZN7tAppMap8EndBuildEP10tPublicEnv"
SYM_GET_APO_COUNT = "_ZN7tAppMap11GetApoCountEv"
SYM_GET_FUNC_NAME = "_ZN7tAppMap11GetFuncNameEi"

BEST = {"count": 0, "functions": [], "all_apos": []}


def log(msg):
    print(msg)
    with open(LOG_PATH, "a") as f:
        f.write(msg + "\n")
        f.flush()


def call_sym(sym, this_addr, ret_type, arg_ctype=None, arg_val=None):
    if arg_ctype:
        expr = (
            f"(({ret_type}(*)(void*,{arg_ctype}))&{sym})"
            f"((void*)0x{this_addr:x}, {arg_val})"
        )
    else:
        expr = f"(({ret_type}(*)(void*))&{sym})((void*)0x{this_addr:x})"
    try:
        return gdb.parse_and_eval(expr)
    except gdb.error as e:
        log(f"  call_sym ERROR {sym}: {e}")
        return None


class EndBuildBreakpoint(gdb.Breakpoint):
    """tAppMap::EndBuild() roda depois que todas as funções do fonte já
    foram registradas no mapa (ao contrário de save(), interceptado na
    entrada e disparado cedo demais — só via com a 1a função já
    presente). Lê o snapshot completo de GetApoCount()/GetFuncName(i)
    aqui."""

    def __init__(self):
        super().__init__(SYM_END_BUILD, internal=False)
        self.hits = 0

    def stop(self):
        self.hits += 1
        this = int(gdb.parse_and_eval("$rdi"))

        count_val = call_sym(SYM_GET_APO_COUNT, this, "int")
        if count_val is None:
            return False
        count = int(count_val)

        names = []
        all_apos = []
        for i in range(count):
            name_val = call_sym(SYM_GET_FUNC_NAME, this, "char*", "int", i)
            name = ""
            if name_val and int(name_val) != 0:
                name = name_val.string(errors="replace")
            all_apos.append({"index": i, "name": name})
            if name:
                names.append(name)

        log(f"[hit#{self.hits}] this=0x{this:x} count={count} com_nome={len(names)}")
        for n in names:
            log(f"  {n}")

        if count >= BEST["count"]:
            BEST["count"] = count
            BEST["functions"] = names
            BEST["all_apos"] = all_apos

        return False  # nunca pausa de fato — só observa e deixa continuar


def main():
    with open(LOG_PATH, "w") as f:
        f.write("RPO Extract - Protheus RPO function extractor\n")
        f.write(f"Symbols: EndBuild={SYM_END_BUILD} GetApoCount={SYM_GET_APO_COUNT} GetFuncName={SYM_GET_FUNC_NAME}\n\n")

    gdb.execute("set breakpoint pending on")
    try:
        EndBuildBreakpoint()
    except gdb.error as e:
        log(f"ERRO ao criar breakpoint: {e}")
        log("Redescubra os símbolos: readelf --dyn-syms -W libaplinux.so | c++filt | grep tAppMap::")
        return

    log("Iniciando appsrvlinux (bloqueante até o processo terminar; "
        "argumentos vêm do --args do gdb, ver docstring do arquivo)...")
    gdb.execute("run", to_string=False)

    log(f"\nMelhor snapshot: {BEST['count']} apoios, {len(BEST['functions'])} com nome")
    for fname in BEST["functions"]:
        log(f"  {fname}")

    with open(LOG_PATH + ".funcs", "w") as fh:
        for fname in BEST["functions"]:
            fh.write(fname + "\n")

    with open(LOG_PATH + ".all", "w") as fh:
        fh.write(f"Count: {BEST['count']}\n")
        for apo in BEST["all_apos"]:
            fh.write(f"  [{apo['index']}] {apo['name']}\n")

    with open(LOG_PATH + ".json", "w") as fh:
        json.dump(BEST, fh, indent=2, ensure_ascii=False)

    log("\nResultados salvos:")
    for suffix in (".funcs", ".all", ".json"):
        log(f"  {LOG_PATH}{suffix}")


if __name__ == "__main__":
    main()
