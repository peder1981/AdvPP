#!/usr/bin/env python3
"""Relay server: minicpm5 (tool-user) -> lfm25 (reader/reproducer), exposed as
a single Ollama-compatible /api/chat endpoint so NeuralInverse (or any Ollama
client) can talk to it like a normal model, without knowing two models and a
bash tool loop are running underneath.

Usage: relay.py --domain advpp|okf --port N
"""
import argparse
import fcntl
import json
import os
import re
import sqlite3
import subprocess
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

OLLAMA_BASE = "http://127.0.0.1:11434"
LOCK_PATH = "/tmp/ernesto-locks/ollama-inference.lock"
MAX_STEPS = 4
NUM_PREDICT = 300
OKF_DB = "/home/peder/Projetos/OKF/index.db"
OKF_BYNAME = "/home/peder/Projetos/OKF/by-name"

DOMAINS = {
    "advpp": {
        "tool_model": "advpp-dev-minicpm5:latest",
        "reader_model": "advpp-dev-lfm25:latest",
        "cwd": "/home/peder/Projetos/AdvPP",
        "virtual_name": "advpp-relay:latest",
        "allow": [r"^\./advplc (check|run|build|compile) ", r"^grep ", r"^cat ", r"^wc ", r"^ls "],
        "extra_tools": "okf_lookup",  # co-pilot also gets grounding lookups into the OKF library
    },
    "okf": {
        "tool_model": "okf-search-minicpm5:latest",
        "reader_model": "okf-search-lfm25:latest",
        "cwd": "/home/peder/Projetos/OKF",
        "virtual_name": "okf-relay:latest",
        "allow": [],  # okf uses native micro-tools instead of raw bash, see OKF_TOOLS
        "extra_tools": None,
    },
}

OKF_LOOKUP_TOOL_JSON = [{
    "type": "function",
    "function": {
        "name": "okf_lookup",
        "description": ("Busca conhecimento tecnico de Protheus/AdvPL/programacao na biblioteca OKF "
                         "(livros, manuais, modulos de codigo-fonte) quando a pergunta exige um fato "
                         "que voce nao tem de cor. Um unico termo de busca, sem sintaxe SQL."),
        "parameters": {
            "type": "object",
            "properties": {"termo": {"type": "string", "description": "palavra-chave, nome de modulo ou nome de arquivo"}},
            "required": ["termo"],
        },
    },
}]


def okf_lookup(termo):
    """Cascading grounding lookup: exact filename -> prefix -> full-text -> code module.
    The caller only supplies a search term; layer selection and SQL syntax
    are entirely this function's job, not the small model's."""
    if not termo:
        return "nao encontrado na base OKF (termo vazio)"
    con = sqlite3.connect(OKF_DB)
    cur = con.cursor()
    try:
        cur.execute("SELECT category, filename FROM docs WHERE filename = ?", (termo,))
        rows = cur.fetchall()
        if rows:
            return "encontrado (nome exato): " + "; ".join(f"{c}/{f}" for c, f in rows)

        cur.execute("SELECT category, filename FROM docs WHERE filename GLOB ? LIMIT 5", (termo + "*",))
        rows = cur.fetchall()
        if rows:
            return "encontrado (prefixo): " + "; ".join(f"{c}/{f}" for c, f in rows)

        try:
            cur.execute("""SELECT d.category, d.filename FROM docs d JOIN docs_fts
                           ON docs_fts.rowid = d.rowid WHERE docs_fts MATCH ? LIMIT 5""", (termo,))
            rows = cur.fetchall()
        except sqlite3.OperationalError:
            rows = []
        if rows:
            return "encontrado (busca por palavra-chave): " + "; ".join(f"{c}/{f}" for c, f in rows)

        cur.execute("SELECT tree, module, approx_files FROM modules WHERE module = ?", (termo,))
        rows = cur.fetchall()
        if rows:
            return "encontrado (modulo de codigo): " + "; ".join(f"tree={t} module={m} approx_files={a}" for t, m, a in rows)

        cur.execute("SELECT tree, module, approx_files FROM modules WHERE module LIKE ? LIMIT 5", (f"%{termo}%",))
        rows = cur.fetchall()
        if rows:
            return "encontrado (modulo, aproximado): " + "; ".join(f"tree={t} module={m} approx_files={a}" for t, m, a in rows)

        return "nao encontrado na base OKF"
    finally:
        con.close()

TOOLS_JSON = [{
    "type": "function",
    "function": {
        "name": "bash",
        "description": "Executa um comando bash e retorna stdout+stderr.",
        "parameters": {
            "type": "object",
            "properties": {"command": {"type": "string", "description": "comando bash completo"}},
            "required": ["command"],
        },
    },
}]

# ponytail: a 1B model reliably picks ONE of a few named tools and fills a
# single string argument, but reliably fails to hand-construct correct SQL
# syntax from an example in the prompt (confirmed empirically: it invented
# `tree -L 20 includeP12` instead of the dictated sqlite3 query). Native
# parameterized functions remove the syntax-construction burden entirely.
OKF_TOOLS_JSON = [
    {"type": "function", "function": {"name": "okf_exact", "description": "Busca um arquivo do OKF pelo nome exato, retorna categoria.",
     "parameters": {"type": "object", "properties": {"filename": {"type": "string"}}, "required": ["filename"]}}},
    {"type": "function", "function": {"name": "okf_prefix", "description": "Busca arquivos do OKF cujo nome comeca com um prefixo.",
     "parameters": {"type": "object", "properties": {"prefix": {"type": "string"}}, "required": ["prefix"]}}},
    {"type": "function", "function": {"name": "okf_module", "description": "Retorna a arvore (tree) e a contagem aproximada de arquivos de um modulo de codigo do OKF pelo nome do modulo.",
     "parameters": {"type": "object", "properties": {"module": {"type": "string"}}, "required": ["module"]}}},
    {"type": "function", "function": {"name": "okf_keyword", "description": "Busca full-text por palavra-chave nos documentos do OKF.",
     "parameters": {"type": "object", "properties": {"keyword": {"type": "string"}}, "required": ["keyword"]}}},
    {"type": "function", "function": {"name": "okf_resolve", "description": "Resolve o caminho real (readlink) de um arquivo do OKF a partir do nome exato.",
     "parameters": {"type": "object", "properties": {"filename": {"type": "string"}}, "required": ["filename"]}}},
]


def okf_call(name, args):
    con = sqlite3.connect(OKF_DB)
    cur = con.cursor()
    try:
        if name == "okf_exact":
            cur.execute("SELECT category, filename FROM docs WHERE filename = ?", (args.get("filename", ""),))
        elif name == "okf_prefix":
            cur.execute("SELECT category, filename FROM docs WHERE filename GLOB ? LIMIT 10", (args.get("prefix", "") + "*",))
        elif name == "okf_module":
            cur.execute("SELECT tree, module, approx_files FROM modules WHERE module = ?", (args.get("module", ""),))
        elif name == "okf_keyword":
            cur.execute("""SELECT d.category, d.filename FROM docs d JOIN docs_fts
                           ON docs_fts.rowid = d.rowid WHERE docs_fts MATCH ? LIMIT 10""", (args.get("keyword", ""),))
        elif name == "okf_resolve":
            path = os.path.join(OKF_BYNAME, args.get("filename", ""))
            return os.path.realpath(path) if os.path.islink(path) or os.path.exists(path) else "(não encontrado em by-name/)"
        else:
            return "(tool desconhecida)"
        rows = cur.fetchall()
        return "\n".join(str(r) for r in rows) if rows else "(nenhum resultado)"
    finally:
        con.close()

READER_SYSTEM = """Você recebe uma pergunta original do usuário e dados brutos já coletados por outra ferramenta.
Sua única função: responder a pergunta em português brasileiro, curto e direto, usando SOMENTE os dados fornecidos.
Não invente dado que não esteja nos dados brutos. Não sugira rodar comandos — a busca já foi feita."""


def ollama_chat(model, messages, tools=None, num_predict=NUM_PREDICT):
    body = {"model": model, "messages": messages, "stream": False, "think": False,
            "options": {"num_predict": num_predict}}
    if tools:
        body["tools"] = tools
    req = urllib.request.Request(f"{OLLAMA_BASE}/api/chat", data=json.dumps(body).encode(),
                                  headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=150) as resp:
        return json.loads(resp.read())


def is_allowed(cmd, patterns):
    return any(re.match(p, cmd) for p in patterns)


def run_tool_stage(domain_name, domain_cfg, user_prompt):
    """Stage A: tool-user model gathers raw findings via a tool loop.
    OKF uses native parameterized micro-tools (no SQL construction by the
    model); AdvPP uses the bash+advplc allowlist (single simple command)."""
    use_native = domain_name == "okf"
    tools = OKF_TOOLS_JSON if use_native else TOOLS_JSON
    if domain_cfg.get("extra_tools") == "okf_lookup":
        tools = tools + OKF_LOOKUP_TOOL_JSON
    messages = [{"role": "user", "content": user_prompt}]
    if use_native:
        messages.insert(0, {"role": "system", "content": (
            "Responda chamando UMA das tools disponiveis (okf_exact, okf_prefix, okf_module, "
            "okf_keyword, okf_resolve) com o argumento certo extraido da pergunta. Nunca escreva "
            "comandos bash/sqlite manualmente — as tools ja fazem a query certa por voce.")})
    trace = []
    used_tool = False
    direct_answer = ""
    for _ in range(MAX_STEPS):
        resp = ollama_chat(domain_cfg["tool_model"], messages, tools=tools)
        msg = resp.get("message", {})
        tool_calls = msg.get("tool_calls") or []
        if tool_calls:
            used_tool = True
            messages.append(msg)
            for tc in tool_calls:
                fn = tc.get("function", {})
                name = fn.get("name", "")
                args = fn.get("arguments", {})
                if not isinstance(args, dict):
                    args = {}
                if use_native:
                    result = okf_call(name, args)
                    trace.append(f"{name}({args})\n{result}")
                elif name == "okf_lookup":
                    result = okf_lookup(args.get("termo", ""))
                    trace.append(f"okf_lookup({args})\n{result}")
                else:
                    cmd = args.get("command", "")
                    if is_allowed(cmd, domain_cfg["allow"]):
                        try:
                            out = subprocess.run(cmd, shell=True, cwd=domain_cfg["cwd"],
                                                  capture_output=True, text=True, timeout=30)
                            result = (out.stdout + out.stderr)[:2000]
                        except Exception as e:
                            result = f"[erro ao executar: {e}]"
                    else:
                        result = "[bloqueado pelo relay: comando fora do allowlist]"
                    trace.append(f"$ {cmd}\n{result}")
                messages.append({"role": "tool", "content": result})
            continue
        else:
            direct_answer = msg.get("content", "").strip()
            break
    raw = "\n\n".join(trace) if trace else "(nenhum dado coletado)"
    return raw, used_tool, direct_answer


def run_reader_stage(domain_cfg, user_prompt, raw_findings):
    """Stage B: reader model composes the final PT-BR answer from raw findings."""
    messages = [
        {"role": "system", "content": READER_SYSTEM},
        {"role": "user", "content": f"Pergunta original: {user_prompt}\n\nDados brutos coletados:\n{raw_findings}"},
    ]
    resp = ollama_chat(domain_cfg["reader_model"], messages)
    return resp.get("message", {}).get("content", "").strip()


def handle_chat(domain_name, domain_cfg, payload):
    messages = payload.get("messages", [])
    user_prompt = next((m["content"] for m in reversed(messages) if m.get("role") == "user"), "")
    lock_fd = open(LOCK_PATH, "w")
    fcntl.flock(lock_fd, fcntl.LOCK_EX)
    try:
        raw, used_tool, direct_answer = run_tool_stage(domain_name, domain_cfg, user_prompt)
        if used_tool:
            # a tool actually ran: hand the raw findings to the reader model
            # to compose a fluent PT-BR answer.
            answer = run_reader_stage(domain_cfg, user_prompt, raw)
        else:
            # pure knowledge question, no tool needed: trust the domain
            # model's own answer directly — routing it through a second
            # model to "rewrite" only risks corrupting an already-correct
            # answer (confirmed empirically: reader flipped correct answers
            # like "from" into wrong ones like "extends" when rewriting).
            answer = direct_answer or raw
    finally:
        fcntl.flock(lock_fd, fcntl.LOCK_UN)
        lock_fd.close()
    return {
        "model": domain_cfg["virtual_name"],
        "message": {"role": "assistant", "content": answer},
        "done": True,
    }


def make_handler(domain_name, domain_cfg):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, fmt, *args):
            pass

        def _json(self, obj, status=200):
            body = json.dumps(obj).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            if self.path.startswith("/api/tags"):
                self._json({"models": [{"name": domain_cfg["virtual_name"], "model": domain_cfg["virtual_name"]}]})
            else:
                self._json({"status": "ok"})

        def do_POST(self):
            length = int(self.headers.get("Content-Length", 0))
            payload = json.loads(self.rfile.read(length) or b"{}")
            if self.path.startswith("/api/chat"):
                try:
                    self._json(handle_chat(domain_name, domain_cfg, payload))
                except Exception as e:
                    self._json({"error": str(e)}, status=500)
            else:
                self._json({"error": "not found"}, status=404)

    return Handler


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--domain", choices=DOMAINS.keys(), required=True)
    ap.add_argument("--port", type=int, required=True)
    args = ap.parse_args()
    cfg = DOMAINS[args.domain]
    server = ThreadingHTTPServer(("127.0.0.1", args.port), make_handler(args.domain, cfg))
    print(f"[relay] domain={args.domain} port={args.port} tool_model={cfg['tool_model']} reader_model={cfg['reader_model']}")
    server.serve_forever()
