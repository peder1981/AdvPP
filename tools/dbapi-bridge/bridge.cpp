// advpp-dbapi-bridge — ponte entre a VM AdvPP e o dbapi.so da TOTVS.
//
// Carrega dbapi.so via dlopen, conecta num dbaccess64 real e expõe um
// protocolo binário simples sobre TCP (default 127.0.0.1:17890) para o
// lado Go (pkg/db/dbapi_bridge.go).
//
// Motivo do processo separado: exceções C++ do dbapi (ex.: tAssertException
// disparada por SQL inválido no server) não podem derrubar a VM Go — cada
// requisição é envolvida em try/catch(...) e, em último caso, o processo
// morre sem afetar o advplc.
//
// Build: g++ -O2 -o advpp-dbapi-bridge bridge.cpp -ldl -lpthread
// Uso:   ./advpp-dbapi-bridge [caminho/dbapi.so] [porta_escuta]
//        env ADVPP_DBAPI_SO   — caminho do dbapi.so
//        env ADVPP_DBAPI_PORT — porta de escuta (default 17890)
//
// Protocolo (ver pkg/db/dbapi_bridge.go para a outra ponta):
//   frame  = u32 BE payloadLen + payload
//   req    = u8 opcode + u32 nfields + fields(u32 len + bytes)*
//   rsp    = u8 status  + u32 nfields + fields(u32 len + bytes)*
//            status: 0=OK 1=ERR; len==0xFFFFFFFF marca campo NULL
//   push   = u8 tag(0x82 ROW / 0x83 COLS / 0x84 DONE) + payload próprio
//
// opcodes req: 1 CONNECT, 2 QUERY, 3 FETCH, 4 EXEC, 5 CLOSEWA,
//              6 DISCONNECT, 7 PING

#include <arpa/inet.h>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <dlfcn.h>
#include <map>
#include <mutex>
#include <netinet/in.h>
#include <pthread.h>
#include <string>
#include <sys/socket.h>
#include <unistd.h>
#include <vector>

// ---------- tipos de função do dbapi.so ----------
typedef void* (*GetTopConnFn)(int type, short prefPort);
typedef int   (*ConnectFn)(void* self, const char* server, char* env, char* user, short port, int locktype);
typedef int   (*IsConnFn)(void* self);
typedef int   (*QueryFn)(void* self, char* sql, char* alias);
typedef int   (*ExecFn)(void* self, char* sql, int a, unsigned char b);
typedef int   (*GoTopFn)(void* self, short wa);
typedef int   (*BoolWaFn)(void* self, short wa);          // Eof/Bof
typedef long long (*RecNoFn)(void* self, short wa);
typedef long long (*RecCountFn)(void* self, short wa);
typedef int   (*ColCountFn)(void* self, short wa);
typedef char* (*FieldNFn)(void* self, short wa, short fld);
typedef int   (*FieldDescFn)(void* self, short wa, short fld, char* name, char* type, short* a, short* b, short* c);
typedef int   (*SkipFn)(void* self, short wa, int n);
typedef int   (*CloseWaFn)(void* self, short wa);
typedef int   (*GetErrFn)(void* self, char* buf, short len);
typedef void  (*ReleaseFn)(void* self);

struct DbApi {
    void* h = nullptr;
    GetTopConnFn GetTopConn = nullptr;
    ConnectFn    Connect    = nullptr;
    IsConnFn     IsConnected= nullptr;
    QueryFn      Query      = nullptr;
    ExecFn       SQLExec    = nullptr;
    GoTopFn      GoTop      = nullptr;
    BoolWaFn     Eof        = nullptr;
    BoolWaFn     Bof        = nullptr;
    RecNoFn      RecNo      = nullptr;
    RecCountFn   RecCount   = nullptr;
    ColCountFn   GetColCount= nullptr;
    FieldNFn     GetFieldN  = nullptr;
    FieldDescFn  GetFieldDesc = nullptr;
    SkipFn       Skip       = nullptr;
    CloseWaFn    CloseWa    = nullptr;
    GetErrFn     GetError   = nullptr;
    ReleaseFn    Release    = nullptr;
};

static DbApi g_api;
static std::mutex g_apiMu;           // serializa chamadas dbapi (v1)
static int g_nextConn = 1;
static std::map<int, void*> g_conns; // connId -> tTOPClient*
static std::mutex g_connsMu;

static void* xsym(void* h, const char* n) {
    void* p = dlsym(h, n);
    if (!p) fprintf(stderr, "[bridge] FATAL: dlsym(%s): %s\n", n, dlerror());
    return p;
}

static bool loadDbApi(const char* path) {
    void* h = dlopen(path, RTLD_NOW | RTLD_GLOBAL);
    if (!h) { fprintf(stderr, "[bridge] dlopen(%s): %s\n", path, dlerror()); return false; }
    g_api.h = h;
    g_api.GetTopConn   = (GetTopConnFn)xsym(h, "v40TC_GetTopConn");
    g_api.Connect      = (ConnectFn)   xsym(h, "_ZN10tTOPClient7ConnectEPKcPcS2_si");
    g_api.IsConnected  = (IsConnFn)    xsym(h, "_ZN12tBasicClient11IsConnectedEv");
    g_api.Query        = (QueryFn)     xsym(h, "_ZN10tTOPClient5QueryEPcS0_");
    g_api.SQLExec      = (ExecFn)      xsym(h, "_ZN10tTOPClient7SQLExecEPcih");
    g_api.GoTop        = (GoTopFn)     xsym(h, "_ZN10tTOPClient5GoTopEs");
    g_api.Eof          = (BoolWaFn)    xsym(h, "_ZN10tTOPClient3EofEs");
    g_api.Bof          = (BoolWaFn)    xsym(h, "_ZN10tTOPClient3BofEs");
    g_api.RecNo        = (RecNoFn)     xsym(h, "_ZN10tTOPClient5RecNoEs");
    g_api.RecCount     = (RecCountFn)  xsym(h, "_ZN10tTOPClient8RecCountEs");
    g_api.GetColCount  = (ColCountFn)  xsym(h, "_ZN10tTOPClient11GetColCountEs");
    g_api.GetFieldN    = (FieldNFn)    xsym(h, "_ZN10tTOPClient9GetFieldNEss");
    g_api.GetFieldDesc = (FieldDescFn) xsym(h, "_ZN10tTOPClient12GetFieldDescEssPcS0_PsS1_S1_");
    g_api.Skip         = (SkipFn)      xsym(h, "_ZN10tTOPClient4SkipEsi");
    g_api.CloseWa      = (CloseWaFn)   xsym(h, "_ZN10tTOPClient5CloseEs");
    g_api.GetError     = (GetErrFn)    xsym(h, "_ZN10tTOPClient8GetErrorEPcs");
    g_api.Release      = (ReleaseFn)   xsym(h, "v40TC_Release");
    bool ok = g_api.GetTopConn && g_api.Connect && g_api.Query &&
              g_api.GetFieldN && g_api.Skip && g_api.Eof && g_api.Release;
    if (!ok) fprintf(stderr, "[bridge] símbolos obrigatórios ausentes\n");
    return ok;
}

// ---------- codec de campos ----------
struct Field { std::string data; bool isNull = false; };
static const uint32_t kNullLen = 0xFFFFFFFFu;

static void putField(std::vector<uint8_t>& out, const std::string& s) {
    uint32_t n = (uint32_t)s.size();
    out.push_back((n >> 24) & 0xff); out.push_back((n >> 16) & 0xff);
    out.push_back((n >> 8) & 0xff);  out.push_back(n & 0xff);
    out.insert(out.end(), s.begin(), s.end());
}
static void putFieldNull(std::vector<uint8_t>& out) {
    out.push_back(0xff); out.push_back(0xff); out.push_back(0xff); out.push_back(0xff);
}
static void putU32(std::vector<uint8_t>& out, uint32_t v) {
    out.push_back((v >> 24) & 0xff); out.push_back((v >> 16) & 0xff);
    out.push_back((v >> 8) & 0xff);  out.push_back(v & 0xff);
}
static void putU64(std::vector<uint8_t>& out, uint64_t v) {
    for (int i = 7; i >= 0; i--) out.push_back((uint8_t)(v >> (8 * i)));
}
static void putU8(std::vector<uint8_t>& out, uint8_t b) { out.push_back(b); }

struct Reader {
    const uint8_t* p; size_t n; size_t off = 0;
    bool u8(uint8_t& v)  { if (off + 1 > n) return false; v = p[off++]; return true; }
    bool u32(uint32_t& v){ if (off + 4 > n) return false; v = (p[off]<<24)|(p[off+1]<<16)|(p[off+2]<<8)|p[off+3]; off += 4; return true; }
    bool u64(uint64_t& v){ if (off + 8 > n) return false; v = 0; for (int i = 0; i < 8; i++) v = (v<<8)|p[off++]; return true; }
    bool bytes(std::string& s, uint32_t len) {
        if (len == kNullLen) { s.clear(); return true; } // NULL marker
        if (off + len > n) return false;
        s.assign((const char*)p + off, len); off += len; return true;
    }
    bool field(Field& f) {
        uint32_t len; if (!u32(len)) return false;
        if (len == kNullLen) { f.isNull = true; f.data.clear(); return true; }
        if (off + len > n) return false;
        f.data.assign((const char*)p + off, len); off += len; f.isNull = false; return true;
    }
    bool fields(std::vector<Field>& fs, uint32_t count) {
        fs.clear(); fs.reserve(count);
        for (uint32_t i = 0; i < count; i++) { Field f; if (!field(f)) return false; fs.push_back(std::move(f)); }
        return true;
    }
};

// ---------- socket helpers ----------
static bool readFull(int fd, void* buf, size_t len) {
    uint8_t* p = (uint8_t*)buf; size_t got = 0;
    while (got < len) {
        ssize_t r = recv(fd, p + got, len - got, 0);
        if (r <= 0) return false;
        got += (size_t)r;
    }
    return true;
}
static bool writeFull(int fd, const void* buf, size_t len) {
    const uint8_t* p = (const uint8_t*)buf; size_t sent = 0;
    while (sent < len) {
        ssize_t r = send(fd, p + sent, len - sent, MSG_NOSIGNAL);
        if (r <= 0) return false;
        sent += (size_t)r;
    }
    return true;
}
static bool sendFrame(int fd, const std::vector<uint8_t>& payload) {
    uint32_t n = htonl((uint32_t)payload.size());
    if (!writeFull(fd, &n, 4)) return false;
    return writeFull(fd, payload.data(), payload.size());
}
static bool recvFrame(int fd, std::vector<uint8_t>& payload) {
    uint32_t n;
    if (!readFull(fd, &n, 4)) return false;
    n = ntohl(n);
    if (n > 64u * 1024u * 1024u) return false; // sanity: 64MB
    payload.resize(n);
    return n == 0 || readFull(fd, payload.data(), n);
}

// ---------- respostas ----------
static std::string fieldStr(const std::vector<Field>& f, size_t i) {
    return (i < f.size() && !f[i].isNull) ? f[i].data : std::string();
}
static void* connById(const std::vector<Field>& f) {
    int id = atoi(fieldStr(f, 0).c_str());
    std::lock_guard<std::mutex> lk(g_connsMu);
    auto it = g_conns.find(id);
    return it == g_conns.end() ? nullptr : it->second;
}
static std::string lastError(void* cli) {
    if (!g_api.GetError) return "erro desconhecido";
    char buf[1024]; memset(buf, 0, sizeof buf);
    g_api.GetError(cli, buf, (short)sizeof(buf) - 1);
    return buf[0] ? buf : "erro desconhecido";
}

// envia resposta terminal (status + fields)
static bool respond(int fd, uint8_t status, std::vector<Field> fields) {
    std::vector<uint8_t> out;
    putU8(out, status);
    putU32(out, (uint32_t)fields.size());
    for (auto& f : fields) {
        if (f.isNull) putFieldNull(out); else putField(out, f.data);
    }
    return sendFrame(fd, out);
}
static bool respondErr(int fd, const std::string& msg) {
    std::vector<Field> f(1); f[0].data = msg;
    return respond(fd, 1, f);
}
static std::vector<Field> okFields(std::initializer_list<std::string> xs) {
    std::vector<Field> f; f.reserve(xs.size());
    for (auto& s : xs) { Field d; d.data = s; f.push_back(d); }
    return f;
}

// ---------- handlers (todos sob g_apiMu + try/catch) ----------
struct Ctx { int fd; };

static void handleConnect(Ctx& c, const std::vector<Field>& in) {
    std::string host = fieldStr(in, 0), env = fieldStr(in, 1), user = fieldStr(in, 2);
    short port = (short)atoi(fieldStr(in, 3).c_str());
    int locktype = atoi(fieldStr(in, 4).c_str());
    if (host.empty() || env.empty()) { respondErr(c.fd, "connect: host/env obrigatorios"); return; }

    void* cli = g_api.GetTopConn(0, port);
    if (!cli) { respondErr(c.fd, "connect: GetTopConn falhou"); return; }
    int rc = g_api.Connect(cli, host.c_str(), (char*)env.c_str(), (char*)user.c_str(), port, locktype);
    if (rc != 0 || (g_api.IsConnected && !g_api.IsConnected(cli))) {
        std::string e = lastError(cli);
        if (g_api.Release) g_api.Release(cli);
        respondErr(c.fd, "connect: " + e);
        return;
    }
    int id;
    { std::lock_guard<std::mutex> lk(g_connsMu); id = g_nextConn++; g_conns[id] = cli; }
    respond(c.fd, 0, okFields({std::to_string(id)}));
}

// QUERY: abre workarea, GoTop, devolve COLS + dispara FETCH implicito?
// Nao: QUERY devolve header (wa,ncols,coldescs); FETCH pagina as rows.
static void handleQuery(Ctx& c, const std::vector<Field>& in) {
    void* cli = connById(in);
    if (!cli) { respondErr(c.fd, "query: conexao invalida"); return; }
    std::string sql = fieldStr(in, 1);
    if (sql.empty()) { respondErr(c.fd, "query: sql vazio"); return; }

    char* sqlbuf = strdup(sql.c_str());
    int wa = g_api.Query(cli, sqlbuf, nullptr);
    free(sqlbuf);
    if (wa < 0) { respondErr(c.fd, "query: " + lastError(cli)); return; }
    if (g_api.GoTop) g_api.GoTop(cli, (short)wa);

    int ncols = g_api.GetColCount ? g_api.GetColCount(cli, (short)wa) : 0;
    if (ncols < 0) ncols = 0;
    if (ncols > 512) ncols = 512;

    // frame COLS (0x83): wa, ncols, e por coluna name/type/size/dec/flags
    std::vector<uint8_t> cols;
    putU8(cols, 0x83);
    putU32(cols, 2 + (uint32_t)ncols * 5);
    auto putF = [&](const std::string& s) { putField(cols, s); };
    putF(std::to_string(wa));
    putF(std::to_string(ncols));
    for (int f = 0; f < ncols; f++) {
        char name[256]; memset(name, 0, sizeof name);
        char type[64];  memset(type, 0, sizeof type);
        short a = 0, b = 0, d = 0;
        if (g_api.GetFieldDesc)
            g_api.GetFieldDesc(cli, (short)wa, (short)f, name, type, &a, &b, &d);
        putF(name); putF(type);
        putF(std::to_string(a)); putF(std::to_string(b)); putF(std::to_string(d));
    }
    if (!sendFrame(c.fd, cols)) return;
    // resposta terminal OK com wa
    respond(c.fd, 0, okFields({std::to_string(wa)}));
}

// FETCH: le ate maxrows a partir da posicao atual; emite ROW(0x82)* e DONE(0x84)
static void handleFetch(Ctx& c, const std::vector<Field>& in) {
    void* cli = connById(in);
    if (!cli) { respondErr(c.fd, "fetch: conexao invalida"); return; }
    int wa = atoi(fieldStr(in, 1).c_str());
    int maxrows = atoi(fieldStr(in, 2).c_str());
    if (maxrows <= 0) maxrows = 100;
    if (maxrows > 100000) maxrows = 100000;
    int ncols = g_api.GetColCount ? g_api.GetColCount(cli, (short)wa) : 0;
    if (ncols < 0) ncols = 0;

    std::vector<uint8_t> rows;
    putU8(rows, 0x82);
    uint32_t nrowsPos = rows.size();
    putU32(rows, 0); // patch depois
    uint32_t nrows = 0;
    bool eof = false;

    for (int i = 0; i < maxrows; i++) {
        if (g_api.Eof && g_api.Eof(cli, (short)wa)) { eof = true; break; }
        long long recno = g_api.RecNo ? g_api.RecNo(cli, (short)wa) : 0;
        putU64(rows, (uint64_t)recno);
        putU32(rows, (uint32_t)ncols);
        for (int f = 1; f <= ncols; f++) {
            char* v = g_api.GetFieldN ? g_api.GetFieldN(cli, (short)wa, (short)f) : nullptr;
            if (!v) putFieldNull(rows); else putField(rows, v);
        }
        nrows++;
        if (g_api.Skip) g_api.Skip(cli, (short)wa, 1);
    }
    if (!eof && g_api.Eof) eof = g_api.Eof(cli, (short)wa) != 0;
    // patch nrows
    rows[nrowsPos]   = (nrows >> 24) & 0xff;
    rows[nrowsPos+1] = (nrows >> 16) & 0xff;
    rows[nrowsPos+2] = (nrows >> 8) & 0xff;
    rows[nrowsPos+3] = nrows & 0xff;
    if (!sendFrame(c.fd, rows)) return;

    // DONE (0x84): eof flag
    std::vector<uint8_t> done;
    putU8(done, 0x84);
    putU32(done, 1);
    putField(done, eof ? "1" : "0");
    sendFrame(c.fd, done);
    respond(c.fd, 0, okFields({})); // terminal OK vazio
}

static void handleExec(Ctx& c, const std::vector<Field>& in) {
    void* cli = connById(in);
    if (!cli) { respondErr(c.fd, "exec: conexao invalida"); return; }
    std::string sql = fieldStr(in, 1);
    if (sql.empty()) { respondErr(c.fd, "exec: sql vazio"); return; }
    char* sqlbuf = strdup(sql.c_str());
    int rc = g_api.SQLExec ? g_api.SQLExec(cli, sqlbuf, 0, 0) : -1;
    free(sqlbuf);
    if (rc != 0) { respondErr(c.fd, "exec: " + lastError(cli)); return; }
    respond(c.fd, 0, okFields({}));
}

static void handleCloseWa(Ctx& c, const std::vector<Field>& in) {
    void* cli = connById(in);
    if (!cli) { respondErr(c.fd, "closewa: conexao invalida"); return; }
    int wa = atoi(fieldStr(in, 1).c_str());
    int rc = (g_api.CloseWa && wa > 0) ? g_api.CloseWa(cli, (short)wa) : 0;
    if (rc != 0) { respondErr(c.fd, "closewa: " + lastError(cli)); return; }
    respond(c.fd, 0, okFields({}));
}

static void handleDisconnect(Ctx& c, const std::vector<Field>& in) {
    int id = atoi(fieldStr(in, 0).c_str());
    void* cli = nullptr;
    { std::lock_guard<std::mutex> lk(g_connsMu);
      auto it = g_conns.find(id); if (it != g_conns.end()) { cli = it->second; g_conns.erase(it); } }
    if (cli && g_api.Release) g_api.Release(cli);
    respond(c.fd, 0, okFields({}));
}

// ---------- loop por cliente ----------
static void* clientThread(void* arg) {
    int fd = (int)(intptr_t)arg;
    std::vector<uint8_t> frame;
    Ctx ctx{fd};
    while (recvFrame(fd, frame)) {
        if (frame.empty()) break;
        Reader r{frame.data(), frame.size()};
        uint8_t op; uint32_t nf;
        if (!r.u8(op) || !r.u32(nf)) break;
        std::vector<Field> in;
        if (!r.fields(in, nf)) break;

        std::lock_guard<std::mutex> apiLock(g_apiMu);
        try {
            switch (op) {
                case 1: handleConnect(ctx, in); break;
                case 2: handleQuery(ctx, in); break;
                case 3: handleFetch(ctx, in); break;
                case 4: handleExec(ctx, in); break;
                case 5: handleCloseWa(ctx, in); break;
                case 6: handleDisconnect(ctx, in); break;
                case 7: respond(ctx.fd, 0, okFields({"pong"})); break;
                default: respondErr(ctx.fd, "opcode desconhecido"); break;
            }
        } catch (...) {
            // exceção C++ do dbapi (tAssertException etc.) — isola o processo
            // respondendo erro em vez de deixar propagar e matar o bridge.
            respondErr(ctx.fd, "excecao interna dbapi");
        }
    }
    close(fd);
    return nullptr;
}

int main(int argc, char** argv) {
    const char* so = argc > 1 ? argv[1] : getenv("ADVPP_DBAPI_SO");
    if (!so || !*so) {
        const char* cands[] = {"./dbapi.so", "/usr/lib/dbapi.so", "/usr/local/lib/dbapi.so"};
        for (auto c : cands) { void* t = dlopen(c, RTLD_NOW | RTLD_GLOBAL); if (t) { dlclose(t); so = c; break; } }
    }
    if (!so) { fprintf(stderr, "[bridge] dbapi.so nao encontrado\nuso: %s [dbapi.so] [porta]\n", argv[0]); return 2; }
    if (!loadDbApi(so)) return 2;
    fprintf(stderr, "[bridge] dbapi.so carregada de %s\n", so);

    int port = argc > 2 ? atoi(argv[2]) : 17890;
    if (const char* e = getenv("ADVPP_DBAPI_PORT")) port = atoi(e);

    int srv = socket(AF_INET, SOCK_STREAM, 0);
    int one = 1; setsockopt(srv, SOL_SOCKET, SO_REUSEADDR, &one, sizeof one);
    sockaddr_in addr{}; addr.sin_family = AF_INET; addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK); addr.sin_port = htons((uint16_t)port);
    if (bind(srv, (sockaddr*)&addr, sizeof addr) != 0 || listen(srv, 16) != 0) {
        perror("[bridge] bind/listen"); return 2;
    }
    fprintf(stderr, "[bridge] escutando em 127.0.0.1:%d\n", port);

    for (;;) {
        int cfd = accept(srv, nullptr, nullptr);
        if (cfd < 0) continue;
        pthread_t th;
        if (pthread_create(&th, nullptr, clientThread, (void*)(intptr_t)cfd) == 0)
            pthread_detach(th);
        else
            close(cfd);
    }
    return 0;
}
