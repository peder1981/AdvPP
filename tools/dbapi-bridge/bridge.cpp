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
// Build: ./build.sh [saida]  (Linux: g++; Windows: MinGW g++ — ver build.sh)
// Uso:   ./advpp-dbapi-bridge [caminho/dbapi.so|dbapi.dll] [porta_escuta]
//        ./advpp-dbapi-bridge --symbols <dbapi> — lista a resolução dos
//        símbolos e sai (0 = todos achados)
//        env ADVPP_DBAPI_SO   — caminho do dbapi.so / dbapi.dll
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
//              6 DISCONNECT, 7 PING, 8 WACMD (workarea remota)
//
// WACMD fields: [connID, cmd, p1, p2]
//   open      p1=file            -> OK [wa]
//   close     p1=wa              -> OK
//   gotop     p1=wa              -> OK [recno]
//   gobottom  p1=wa              -> OK [recno]
//   goto      p1=wa p2=recno     -> OK [recno]
//   skip      p1=wa p2=n         -> OK [recno]
//   bof/eof   p1=wa              -> OK ["0"|"1"]
//   recno     p1=wa              -> OK [recno]
//   reccount  p1=wa              -> OK [count]
//   colcount  p1=wa              -> OK [n]
//   field     p1=wa p2=nome      -> OK [valor] (NULL se vazio)
//   fieldn    p1=wa p2=idx(1based)-> OK [valor] (NULL se vazio)
//   fielddesc p1=wa p2=idx(0b)   -> OK [name,type,size,dec,flags]
//   setfieldn p1=wa p2=idx(1b) p3=val -> OK [rc]   (buffer do record)
//   setfield  p1=wa p2=nome p3=val     -> OK [rc]
//   update    p1=wa [p2=recno]  -> OK [rc]  (grava buffer no recno)
//   write     p1=wa              -> OK [rc]  (insere novo registro)
//   append    p1=wa              -> OK [recno] (posiciona alem do fim)
//   delete    p1=wa [p2=recno]   -> OK [rc]
//   recall    p1=wa [p2=recno]   -> OK [rc]
//   commit    -                  -> OK [rc]
//   rollback  -                  -> OK [rc]
//   lock      p1=wa p2=recno     -> OK [rc]
//   unlock    p1=wa p2=recno     -> OK [rc]
//
// ATENCAO semantica dbapi: GetFieldN/GetField devolvem char* para dentro
// do record buffer — a string corre ate o fim do registro (todos os
// campos seguintes concatenados). O tamanho real do campo vem do
// fielddesc (campo size). Campos tipo 'F' NAO sao string: o ponteiro
// aponta para um double IEEE-754 little-endian de 8 bytes dentro do
// record buffer (verificado empiricamente: R_E_C_N_O_/COUNT(*)). O
// bridge decodifica 'F' para string numerica; tipos ainda nao
// verificados ('D','L','N','M','B'...) sao devolvidos como hex dos
// bytes crus — ponytail: rever quando houver base com datas/logicals.

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <map>
#include <mutex>
#include <string>
#include <thread>
#include <vector>
#ifdef _WIN32
// Windows: dbapi.dll (mesma API do dbapi.so, build MSVC da TOTVS).
// ponytail: SOCKET guardado em int — handles Winsock cabem em 32 bits na
// prática; trocar por SOCKET em tudo se algum dia não couber.
#  include <winsock2.h>
#  include <windows.h>
#  include <dbghelp.h>
#  include <strings.h>
#  include <cxxabi.h>
#  define MSG_NOSIGNAL 0
#  define closesock closesocket
#else
#  include <arpa/inet.h>
#  include <dlfcn.h>
#  include <netinet/in.h>
#  include <strings.h>
#  include <sys/socket.h>
#  include <unistd.h>
#  define closesock close
#endif

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
typedef int   (*OpenFileFn)(void* self, char* file, unsigned char a, unsigned char b);
typedef int   (*GoBottomFn)(void* self, short wa);
typedef int   (*GoToFn)(void* self, short wa, long long recno, unsigned char flag);
typedef char* (*FieldByNameFn)(void* self, short wa, const char* name);
typedef int   (*FieldNSetFn)(void* self, short wa, short fld, const char* val);
typedef int   (*FieldSetFn)(void* self, short wa, const char* name, const char* val);
typedef int   (*UpdateFn)(void* self, short wa, long long recno);
typedef int   (*WriteFn)(void* self, short wa);
typedef int   (*DelRecFn)(void* self, short wa, long long recno);
typedef int   (*NoArgFn)(void* self);
typedef int   (*LockFn)(void* self, short wa, long long recno);
typedef int   (*SetKeyFn)(void* self, short wa, short keynum);
typedef int   (*SetKeyCFn)(void* self, short wa, const char* name);
typedef int   (*Seek2Fn)(void* self, short wa, char* key, short len, unsigned char flag);
typedef int   (*PackFn)(void* self, short wa);
typedef int   (*KeyNamesFn)(void* self, short wa, char* buf);
typedef int   (*WaFn)(void* self, short wa);   // FLock/FUnLock

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
    OpenFileFn   OpenFile   = nullptr;
    GoBottomFn   GoBottom   = nullptr;
    GoToFn       GoTo       = nullptr;
    FieldByNameFn GetFieldByName = nullptr;
    FieldNSetFn  SetFieldN  = nullptr;
    FieldSetFn   SetField   = nullptr;
    UpdateFn     Update     = nullptr;
    WriteFn      Write      = nullptr;
    DelRecFn     DeleteR    = nullptr;
    DelRecFn     RecallR    = nullptr;
    NoArgFn      Commit     = nullptr;
    NoArgFn      Rollback   = nullptr;
    NoArgFn      StrCmtCtl  = nullptr;
    NoArgFn      EndCmtCtl  = nullptr;
    LockFn       Lock       = nullptr;
    LockFn       UnLock     = nullptr;
    SetKeyFn     SetKey     = nullptr;
    SetKeyCFn    SetKeyC    = nullptr;
    Seek2Fn      Seek2      = nullptr;
    PackFn       Pack       = nullptr;
    KeyNamesFn   GetKeyNames= nullptr;
    WaFn         FLock      = nullptr;
    WaFn         FUnLock    = nullptr;
};

static DbApi g_api;
// serialização por conexão: cada thread de cliente já lê frames em
// série, e ops no mesmo connID compartilham o mesmo tTOPClient —
// mutex por connID basta. CONNECT fica no mutex global (init
// compartilhado do dbapi). ponytail: mutexes nunca são removidos —
// conexões são poucas; se virar problema, erase no DISCONNECT.
static std::mutex g_apiMu;           // só p/ CONNECT
static std::map<int, std::mutex> g_locks;
static std::mutex g_locksMu;
static std::mutex& lockFor(int connID) {
    std::lock_guard<std::mutex> lk(g_locksMu);
    return g_locks[connID];
}
static int g_nextConn = 1;
static std::map<int, void*> g_conns;   // connId -> tTOPClient*
static std::map<int, std::vector<int>> g_fdConns; // fd -> connIds (cleanup no disconnect do socket)
static std::mutex g_connsMu;

static bool g_listSyms = false; // --symbols
static int  g_missingSyms = 0;

#ifdef _WIN32
// O dbapi.dll exporta os métodos com mangling MSVC (?Connect@tTOPClient@@...),
// não Itanium. Em vez de manter uma 2a tabela de nomes, desmangla o nome
// Itanium usado no Linux e casa com o export MSVC desmanglado (mesma
// assinatura C++). ponytail: não verificado contra um dbapi.dll real
// (2026-11-26, sem a DLL); CI só valida contra uma DLL MSVC de teste.
static bool identCh(char c) { return isalnum((unsigned char)c) || c == '_'; }
// Normaliza as duas grafias para comparar: Itanium "char const*, long long"
// e MSVC "char const *,__int64". Espaço só fica entre dois identificadores
// ("unsigned char", "int __cdecl tTOPClient::") — senão o nome qualificado
// grudaria no que vem antes.
static std::string normSig(const std::string& in) {
    std::string s;
    for (size_t i = 0; i < in.size(); i++) {
        if (in[i] == ' ' && !(i > 0 && i + 1 < in.size() && identCh(in[i - 1]) && identCh(in[i + 1]) && !s.empty() && s.back() != ' '))
            continue;
        s += in[i];
    }
    for (size_t p; (p = s.find("long long")) != std::string::npos;) s.replace(p, 9, "__int64");
    for (size_t p; (p = s.find(" __ptr64")) != std::string::npos;) s.erase(p, 8);
    for (size_t p; (p = s.find("(void)")) != std::string::npos;) s.replace(p, 6, "()");
    return s;
}
static std::vector<std::pair<std::string, void*>> g_exports; // assinatura normalizada -> endereço

static void loadExports(HMODULE h) {
    auto base = (const uint8_t*)h;
    auto nt = (const IMAGE_NT_HEADERS*)(base + ((const IMAGE_DOS_HEADER*)base)->e_lfanew);
    auto dir = nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_EXPORT];
    if (!dir.VirtualAddress) return;
    auto ed = (const IMAGE_EXPORT_DIRECTORY*)(base + dir.VirtualAddress);
    auto names = (const DWORD*)(base + ed->AddressOfNames);
    for (DWORD i = 0; i < ed->NumberOfNames; i++) {
        const char* n = (const char*)(base + names[i]);
        char buf[2048];
        if (n[0] != '?' || !UnDecorateSymbolName(n, buf, sizeof buf,
                UNDNAME_NO_MS_KEYWORDS | UNDNAME_NO_ACCESS_SPECIFIERS |
                UNDNAME_NO_FUNCTION_RETURNS | UNDNAME_NO_ALLOCATION_MODEL |
                UNDNAME_NO_ALLOCATION_LANGUAGE | UNDNAME_NO_MEMBER_TYPE |
                UNDNAME_NO_THISTYPE))
            continue;
        g_exports.push_back({normSig(buf), (void*)GetProcAddress(h, n)});
    }
}

static void* findSym(void* h, const char* n, std::string& how) {
    if (strncmp(n, "_Z", 2) != 0) { how = n; return (void*)GetProcAddress((HMODULE)h, n); }
    int st = 0;
    char* d = abi::__cxa_demangle(n, nullptr, nullptr, &st);
    if (!d) { how = "demangle falhou"; return nullptr; }
    std::string want = normSig(d);
    free(d);
    std::string qual = want.substr(0, want.find('(')) + "(";
    void* only = nullptr; int cands = 0;
    for (auto& e : g_exports) {
        size_t p = e.first.find(qual);
        if (p == std::string::npos) continue;
        char prev = p ? e.first[p - 1] : 0;
        if (identCh(prev) || prev == ':') continue;
        std::string got = e.first.substr(p);
        got = got.substr(0, got.rfind(')') + 1);
        if (got == want) { how = got; return e.second; }
        only = e.second; cands++; how = got;
    }
    // assinatura diferente mas nome único: usa e avisa (tipo pode ter sido
    // declarado diferente no build Windows, ex. long vs __int64).
    if (cands == 1) { how = "AVISO: assinatura difere, usando " + how + " para " + want; return only; }
    how = want + (cands ? " (ambíguo)" : " (não exportado)");
    return nullptr;
}
static void* openLib(const char* path) { return (void*)LoadLibraryA(path); }
static std::string libErr() { return "LoadLibrary erro " + std::to_string(GetLastError()); }
#else
static void* findSym(void* h, const char* n, std::string& how) { how = n; return dlsym(h, n); }
static void* openLib(const char* path) { return dlopen(path, RTLD_NOW | RTLD_GLOBAL); }
static std::string libErr() { const char* e = dlerror(); return e ? e : "?"; }
#endif

static void* xsym(void* h, const char* n) {
    std::string how;
    void* p = findSym(h, n, how);
    if (!p) { g_missingSyms++; fprintf(stderr, "[bridge] FATAL: símbolo %s: %s\n", n, how.c_str()); }
    else if (g_listSyms) fprintf(stderr, "[bridge] ok %s -> %s\n", n, how.c_str());
    return p;
}

static bool loadDbApi(const char* path) {
    void* h = openLib(path);
    if (!h) { fprintf(stderr, "[bridge] abrir %s: %s\n", path, libErr().c_str()); return false; }
#ifdef _WIN32
    loadExports((HMODULE)h);
#endif
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
    g_api.OpenFile     = (OpenFileFn)  xsym(h, "_ZN10tTOPClient8OpenFileEPchh");
    g_api.GoBottom     = (GoBottomFn)  xsym(h, "_ZN10tTOPClient8GoBottomEs");
    g_api.GoTo         = (GoToFn)      xsym(h, "_ZN10tTOPClient4GoToEsxh");
    g_api.GetFieldByName = (FieldByNameFn) xsym(h, "_ZN10tTOPClient8GetFieldEsPKc");
    g_api.SetFieldN    = (FieldNSetFn) xsym(h, "_ZN10tTOPClient9SetFieldNEssPKc");
    g_api.SetField     = (FieldSetFn)  xsym(h, "_ZN10tTOPClient8SetFieldEsPKcS1_");
    g_api.Update       = (UpdateFn)    xsym(h, "_ZN10tTOPClient6UpdateEsx");
    g_api.Write        = (WriteFn)     xsym(h, "_ZN10tTOPClient5WriteEs");
    g_api.DeleteR      = (DelRecFn)    xsym(h, "_ZN10tTOPClient7DeleteREsx");
    g_api.RecallR      = (DelRecFn)    xsym(h, "_ZN10tTOPClient7RecallREsx");
    g_api.Commit       = (NoArgFn)     xsym(h, "_ZN10tTOPClient6CommitEv");
    g_api.Rollback     = (NoArgFn)     xsym(h, "_ZN10tTOPClient8RollbackEv");
    g_api.StrCmtCtl    = (NoArgFn)     xsym(h, "_ZN10tTOPClient9StrCmtCtlEv");
    g_api.EndCmtCtl    = (NoArgFn)     xsym(h, "_ZN10tTOPClient9EndCmtCtlEv");
    g_api.Lock         = (LockFn)      xsym(h, "_ZN10tTOPClient4LockEsx");
    g_api.UnLock       = (LockFn)      xsym(h, "_ZN10tTOPClient6UnLockEsx");
    g_api.SetKey       = (SetKeyFn)    xsym(h, "_ZN10tTOPClient6SetKeyEss");
    g_api.SetKeyC      = (SetKeyCFn)   xsym(h, "_ZN10tTOPClient7SetKeyCEsPKc");
    g_api.Seek2        = (Seek2Fn)     xsym(h, "_ZN10tTOPClient5Seek2EsPcsh");
    g_api.Pack         = (PackFn)      xsym(h, "_ZN10tTOPClient4PackEs");
    g_api.GetKeyNames  = (KeyNamesFn)   xsym(h, "_ZN10tTOPClient11GetKeyNamesEsPc");
    g_api.FLock        = (WaFn)         xsym(h, "_ZN10tTOPClient5FLockEs");
    g_api.FUnLock      = (WaFn)         xsym(h, "_ZN10tTOPClient7FUnLockEs");
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
        fs.clear(); fs.reserve(std::min<size_t>(count, (n - off) / 4)); // count vem da rede: cada field tem >= 4 bytes
        for (uint32_t i = 0; i < count; i++) { Field f; if (!field(f)) return false; fs.push_back(std::move(f)); }
        return true;
    }
};

// ---------- socket helpers ----------
static bool readFull(int fd, void* buf, size_t len) {
    uint8_t* p = (uint8_t*)buf; size_t got = 0;
    while (got < len) {
        auto r = recv(fd, (char*)p + got, (int)(len - got), 0);
        if (r <= 0) return false;
        got += (size_t)r;
    }
    return true;
}
static bool writeFull(int fd, const void* buf, size_t len) {
    const uint8_t* p = (const uint8_t*)buf; size_t sent = 0;
    while (sent < len) {
        auto r = send(fd, (const char*)p + sent, (int)(len - sent), MSG_NOSIGNAL);
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

// Decodifica o valor cru de GetFieldN conforme o tipo do fielddesc.
// Retorna string pronta para o fio; isNull reflete ponteiro NULL.
// ponytail: 'F' = double LE 8 bytes (verificado); 'C'/'M'/desconhecido
// textual = char* cru (o consumidor fatia por size); demais tipos =
// hex dos bytes crus ate size (max 256) ate RE confirmar o encoding.
struct ColDesc { std::string type; int size = 0; };
static ColDesc fieldDescOf(void* cli, short wa, short idx0) {
    ColDesc d;
    if (!g_api.GetFieldDesc) return d;
    char name[256]; memset(name, 0, sizeof name);
    char type[64];  memset(type, 0, sizeof type);
    short a = 0, b = 0, c = 0;
    if (g_api.GetFieldDesc(cli, wa, idx0, name, type, &a, &b, &c) == 0) {
        d.type = type; d.size = a;
    }
    return d;
}
static std::string hexBytes(const char* v, int n) {
    static const char* h = "0123456789abcdef";
    std::string s; s.reserve(n * 2 + 2);
    s += "0x";
    for (int i = 0; i < n; i++) {
        unsigned char ch = (unsigned char)v[i];
        s += h[ch >> 4]; s += h[ch & 15];
    }
    return s;
}
static std::string decodeFieldVal(char* v, const ColDesc& d, bool& isNull) {
    isNull = (v == nullptr);
    if (!v) return "";
    if (d.type == "F") {
        // double IEEE-754 little-endian (x86) — unico tipo binario
        // verificado empiricamente; 'N'/'D'/'L' caem no hex abaixo
        double x;
        memcpy(&x, v, 8);
        char buf[64];
        if (x == std::floor(x) && std::fabs(x) < 1e15)
            snprintf(buf, sizeof buf, "%.0f", x);
        else
            snprintf(buf, sizeof buf, "%.10g", x);
        return buf;
    }
    if (d.type == "C" || d.type == "M" || d.type.empty())
        return v; // char* cru — consumidor fatia por size
    if (d.type == "D") {
        // data Protheus: 8 bytes ASCII "YYYYMMDD" no record buffer
        // (verificado em SE1990/E1_EMISSAO — antes caia no hex)
        int n = d.size > 0 && d.size <= 64 ? d.size : 8;
        return std::string(v, n);
    }
    // tipos binarios nao verificados: expoe hex para debug/RE
    int n = d.size > 0 && d.size <= 256 ? d.size : 16;
    return hexBytes(v, n);
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
    { std::lock_guard<std::mutex> lk(g_connsMu);
      id = g_nextConn++; g_conns[id] = cli; g_fdConns[c.fd].push_back(id); }
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
    if (ncols > 512) ncols = 512;
    // descs uma vez por FETCH (GetFieldN devolve char* que over-lee;
    // 'F' e double cru — decodeFieldVal resolve por tipo)
    std::vector<ColDesc> descs(ncols);
    for (int f = 0; f < ncols; f++)
        descs[f] = fieldDescOf(cli, (short)wa, (short)f);

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
            bool isNull = false;
            std::string val = decodeFieldVal(v, descs[f - 1], isNull);
            if (isNull) putFieldNull(rows); else putField(rows, val);
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

// WACMD: workarea remota real (OpenFile/Seek-family do tTOPClient).
// ponytail: comandos implementados um a um via if/else — sao ~12 casos
// fixos; se crescer, migrar para tabela de dispatch. Serialização por
// connID (lockFor no clientThread) — tTOPClient é por conexão.
static void handleWaCmd(Ctx& c, const std::vector<Field>& in) {
    void* cli = connById(in);
    if (!cli) { respondErr(c.fd, "wacmd: conexao invalida"); return; }
    std::string cmd = fieldStr(in, 1);
    std::string p1  = fieldStr(in, 2);
    std::string p2  = fieldStr(in, 3);
    std::string p3  = fieldStr(in, 4);
    auto num = [](const std::string& s) { return atoll(s.c_str()); };
    auto waOf = [&]() { return (short)num(p1); };

    if (cmd == "open") {
        if (p1.empty() || !g_api.OpenFile) { respondErr(c.fd, "wacmd open: arquivo vazio"); return; }
        char* f = strdup(p1.c_str());
        int wa = g_api.OpenFile(cli, f, 0, 0);
        free(f);
        if (wa < 0) { respondErr(c.fd, "wacmd open: " + lastError(cli)); return; }
        respond(c.fd, 0, okFields({std::to_string(wa)}));
        return;
    }
    if (cmd == "close") {
        if (g_api.CloseWa && waOf() > 0) g_api.CloseWa(cli, waOf());
        respond(c.fd, 0, okFields({}));
        return;
    }
    if (cmd == "field") {
        if (!g_api.GetFieldByName) { respondErr(c.fd, "wacmd field: indisponivel"); return; }
        // resolve idx0 pelo nome para decodificar por tipo
        int idx0 = -1, ncol = g_api.GetColCount ? g_api.GetColCount(cli, waOf()) : 0;
        for (int i = 0; i < ncol && i < 512; i++) {
            char nm[256]; memset(nm, 0, sizeof nm);
            char tp[64]; memset(tp, 0, sizeof tp);
            short a=0,b=0,d=0;
            if (g_api.GetFieldDesc && g_api.GetFieldDesc(cli, waOf(), (short)i, nm, tp, &a, &b, &d) == 0) {
                if (strcasecmp(nm, p2.c_str()) == 0) { idx0 = i; break; }
            }
        }
        char* f = strdup(p2.c_str());
        char* v = g_api.GetFieldByName(cli, waOf(), f);
        free(f);
        ColDesc cd = idx0 >= 0 ? fieldDescOf(cli, waOf(), (short)idx0) : ColDesc{};
        bool isNull = false;
        std::string val = decodeFieldVal(v, cd, isNull);
        std::vector<Field> out(1);
        if (!isNull) out[0].data = val;
        respond(c.fd, 0, out);
        return;
    }
    if (cmd == "fieldn") {
        if (!g_api.GetFieldN) { respondErr(c.fd, "wacmd fieldn: indisponivel"); return; }
        int idx1 = (int)num(p2);
        char* v = g_api.GetFieldN(cli, waOf(), (short)idx1);
        ColDesc cd = fieldDescOf(cli, waOf(), (short)(idx1 - 1));
        bool isNull = false;
        std::string val = decodeFieldVal(v, cd, isNull);
        std::vector<Field> out(1);
        if (!isNull) out[0].data = val;
        respond(c.fd, 0, out);
        return;
    }
    if (cmd == "fielddesc") {
        if (!g_api.GetFieldDesc) { respondErr(c.fd, "wacmd fielddesc: indisponivel"); return; }
        int idx = (int)num(p2);
        char name[256]; memset(name, 0, sizeof name);
        char type[64];  memset(type, 0, sizeof type);
        short a = 0, b = 0, d = 0;
        int rc = g_api.GetFieldDesc(cli, waOf(), (short)idx, name, type, &a, &b, &d);
        if (rc != 0) { respondErr(c.fd, "wacmd fielddesc: rc=" + std::to_string(rc)); return; }
        respond(c.fd, 0, okFields({name, type, std::to_string(a), std::to_string(b), std::to_string(d)}));
        return;
    }

    short wa = waOf();
    if (cmd == "gotop" || cmd == "gobottom" || cmd == "goto" ||
        cmd == "skip" || cmd == "bof" || cmd == "eof" || cmd == "recno" ||
        cmd == "reccount" || cmd == "colcount") {
        std::string val;
        if (cmd == "gotop" && g_api.GoTop) { g_api.GoTop(cli, wa); val = std::to_string(g_api.RecNo ? g_api.RecNo(cli, wa) : 0); }
        else if (cmd == "gobottom" && g_api.GoBottom) { g_api.GoBottom(cli, wa); val = std::to_string(g_api.RecNo ? g_api.RecNo(cli, wa) : 0); }
        else if (cmd == "goto" && g_api.GoTo) { g_api.GoTo(cli, wa, num(p2), 0); val = std::to_string(g_api.RecNo ? g_api.RecNo(cli, wa) : 0); }
        else if (cmd == "skip" && g_api.Skip) { g_api.Skip(cli, wa, (int)num(p2)); val = std::to_string(g_api.RecNo ? g_api.RecNo(cli, wa) : 0); }
        else if (cmd == "bof" && g_api.Bof) val = g_api.Bof(cli, wa) ? "1" : "0";
        else if (cmd == "eof" && g_api.Eof) val = g_api.Eof(cli, wa) ? "1" : "0";
        else if (cmd == "recno" && g_api.RecNo) val = std::to_string(g_api.RecNo(cli, wa));
        else if (cmd == "reccount" && g_api.RecCount) val = std::to_string(g_api.RecCount(cli, wa));
        else if (cmd == "colcount" && g_api.GetColCount) val = std::to_string(g_api.GetColCount(cli, wa));
        else { respondErr(c.fd, "wacmd: " + cmd + " indisponivel"); return; }
        respond(c.fd, 0, okFields({val}));
        return;
    }

    // ---- escrita / locks (Fase 3) ----
    // ponytail: SetFieldN recebe char* — o dbapi NAO padding/trunca:
    // valor mais curto que o campo deixa o resto do valor antigo no
    // record buffer. O bridge normaliza campos 'C'/'M' para exatamente
    // size bytes (pad espacos a direita, trunca o excesso — semantica
    // ISAM). SetFieldN retorna tamanho escrito (nao 0=sucesso): rc>=0
    // e OK; so responde erro em excecao.
    if (cmd == "setfieldn" || cmd == "setfield") {
        int idx0 = -1;
        if (cmd == "setfieldn") idx0 = (int)num(p2) - 1;
        else {
            int ncol = g_api.GetColCount ? g_api.GetColCount(cli, waOf()) : 0;
            for (int i = 0; i < ncol && i < 512; i++) {
                char nm[256]; memset(nm, 0, sizeof nm);
                char tp[64]; memset(tp, 0, sizeof tp);
                short a=0,b=0,d=0;
                if (g_api.GetFieldDesc && g_api.GetFieldDesc(cli, waOf(), (short)i, nm, tp, &a, &b, &d) == 0) {
                    if (strcasecmp(nm, p2.c_str()) == 0) { idx0 = i; break; }
                }
            }
            if (idx0 < 0) { respondErr(c.fd, "wacmd setfield: campo nao encontrado " + p2); return; }
        }
        ColDesc cd = fieldDescOf(cli, waOf(), (short)idx0);
        std::string val = p3;
        if ((cd.type == "C" || cd.type == "M" || cd.type.empty()) && cd.size > 0) {
            if ((int)val.size() > cd.size) val.resize(cd.size);
            if ((int)val.size() < cd.size) val.append(cd.size - val.size(), ' ');
        }
        int rc = -1;
        if (cmd == "setfieldn" && g_api.SetFieldN)
            rc = g_api.SetFieldN(cli, waOf(), (short)(idx0 + 1), val.c_str());
        else if (cmd == "setfield" && g_api.SetField) {
            char* nm = strdup(p2.c_str());
            rc = g_api.SetField(cli, waOf(), nm, val.c_str());
            free(nm);
        } else { respondErr(c.fd, "wacmd " + cmd + ": indisponivel"); return; }
        respond(c.fd, 0, okFields({std::to_string(rc)}));
        return;
    }
    if (cmd == "update" || cmd == "write" || cmd == "delete" || cmd == "recall" ||
        cmd == "lock" || cmd == "unlock") {
        long long rec = p2.empty() ? (g_api.RecNo ? g_api.RecNo(cli, waOf()) : 0) : num(p2);
        int rc = -1;
        if (cmd == "update" && g_api.Update) rc = g_api.Update(cli, waOf(), rec);
        else if (cmd == "write" && g_api.Write) rc = g_api.Write(cli, waOf());
        else if (cmd == "delete" && g_api.DeleteR) rc = g_api.DeleteR(cli, waOf(), rec);
        else if (cmd == "recall" && g_api.RecallR) rc = g_api.RecallR(cli, waOf(), rec);
        else if (cmd == "lock" && g_api.Lock) rc = g_api.Lock(cli, waOf(), rec);
        else if (cmd == "unlock" && g_api.UnLock) rc = g_api.UnLock(cli, waOf(), rec);
        else { respondErr(c.fd, "wacmd " + cmd + ": indisponivel"); return; }
        if (rc != 0) { respondErr(c.fd, "wacmd " + cmd + ": rc=" + std::to_string(rc) + " " + lastError(cli)); return; }
        respond(c.fd, 0, okFields({std::to_string(rc)}));
        return;
    }
    // flock/funlock: lock de arquivo inteiro na workarea (FLock/FUnLock
    // do tTOPClient — trava a tabela para outros clientes dbAccess).
    if (cmd == "flock" || cmd == "funlock") {
        WaFn fn = (cmd == "flock") ? g_api.FLock : g_api.FUnLock;
        if (!fn) { respondErr(c.fd, "wacmd " + cmd + ": indisponivel"); return; }
        int rc = fn(cli, waOf());
        if (rc != 0) { respondErr(c.fd, "wacmd " + cmd + ": rc=" + std::to_string(rc) + " " + lastError(cli)); return; }
        respond(c.fd, 0, okFields({std::to_string(rc)}));
        return;
    }
    if (cmd == "commit" || cmd == "rollback" || cmd == "strcmtctl" || cmd == "endcmtctl") {
        int rc;
        if (cmd == "commit") rc = g_api.Commit ? g_api.Commit(cli) : -1;
        else if (cmd == "rollback") rc = g_api.Rollback ? g_api.Rollback(cli) : -1;
        else if (cmd == "strcmtctl") rc = g_api.StrCmtCtl ? g_api.StrCmtCtl(cli) : -1;
        else rc = g_api.EndCmtCtl ? g_api.EndCmtCtl(cli) : -1;
        if (rc != 0) { respondErr(c.fd, "wacmd " + cmd + ": rc=" + std::to_string(rc) + " " + lastError(cli)); return; }
        respond(c.fd, 0, okFields({std::to_string(rc)}));
        return;
    }
    if (cmd == "append") {
        // posiciona alem do ultimo registro: GoBottom + Skip(1) deixa a
        // workarea em posicao de novo registro para FieldPut+Write.
        if (!g_api.GoBottom || !g_api.Skip) { respondErr(c.fd, "wacmd append: indisponivel"); return; }
        g_api.GoBottom(cli, waOf());
        g_api.Skip(cli, waOf(), 1);
        long long rec = g_api.RecNo ? g_api.RecNo(cli, waOf()) : 0;
        respond(c.fd, 0, okFields({std::to_string(rec)}));
        return;
    }
    // ---- indices / seek ----
    // ponytail: Seek2 e SOFT seek — rc=0 so significa "posicionou"
    // (primeiro registro >= chave); quem consome deve verificar o
    // registro corrente. Sem SetKey previo, Seek2 falha (rc<0).
    if (cmd == "setkey" || cmd == "setkeyc") {
        int rc;
        if (cmd == "setkey" && g_api.SetKey) rc = g_api.SetKey(cli, waOf(), (short)num(p2));
        else if (cmd == "setkeyc" && g_api.SetKeyC) rc = g_api.SetKeyC(cli, waOf(), p2.c_str());
        else { respondErr(c.fd, "wacmd " + cmd + ": indisponivel"); return; }
        if (rc != 0) { respondErr(c.fd, "wacmd " + cmd + ": rc=" + std::to_string(rc) + " " + lastError(cli)); return; }
        respond(c.fd, 0, okFields({std::to_string(rc)}));
        return;
    }
    if (cmd == "keys") {
        // GetKeyNames retorna a CONTAGEM de indices (rc>=0) e preenche
        // buf — nao e codigo de erro. Verificado: SE1990 -> 29 (SIX tem
        // 29 ordens), SX2990 -> 3.
        if (!g_api.GetKeyNames) { respondErr(c.fd, "wacmd keys: indisponivel"); return; }
        char buf[8192]; memset(buf, 0, sizeof buf);
        int rc = g_api.GetKeyNames(cli, waOf(), buf);
        if (rc < 0) { respondErr(c.fd, "wacmd keys: rc=" + std::to_string(rc)); return; }
        respond(c.fd, 0, okFields({std::to_string(rc), buf}));
        return;
    }
    if (cmd == "seek2") {
        if (!g_api.Seek2) { respondErr(c.fd, "wacmd seek2: indisponivel"); return; }
        char* k = strdup(p2.c_str());
        int rc = g_api.Seek2(cli, waOf(), k, (short)p2.size(), 0);
        free(k);
        long long rec = g_api.RecNo ? g_api.RecNo(cli, waOf()) : -1;
        // OK [rc,recno] — rc!=0 nao e erro fatal: e "nao encontrado"
        respond(c.fd, 0, okFields({std::to_string(rc), std::to_string(rec)}));
        return;
    }
    if (cmd == "pack") {
        if (!g_api.Pack) { respondErr(c.fd, "wacmd pack: indisponivel"); return; }
        int rc = g_api.Pack(cli, waOf());
        if (rc != 0) { respondErr(c.fd, "wacmd pack: rc=" + std::to_string(rc) + " " + lastError(cli)); return; }
        respond(c.fd, 0, okFields({std::to_string(rc)}));
        return;
    }
    // find: scan sequencial server-side — GoTop, compara key com todos
    // os campos (mesmo criterio do Seek em memoria: igualdade exata no
    // valor fatiado/decodificado), Skip(1) ate EOF. Devolve [recno] ou
    // ["-1"]; a wa fica posicionada no registro encontrado (ou EOF).
    // ponytail: substitui o loop Go de fieldn-por-campo-por-record —
    // sem isso um miss em tabela grande sao ~ncols*n roundtrips.
    if (cmd == "find") {
        if (!g_api.GoTop || !g_api.Skip || !g_api.Eof || !g_api.RecNo ||
            !g_api.GetFieldN || !g_api.GetColCount) {
            respondErr(c.fd, "wacmd find: indisponivel"); return;
        }
        short wa = waOf();
        int ncols = g_api.GetColCount(cli, wa);
        if (ncols < 0 || ncols > 512) { respondErr(c.fd, "wacmd find: colcount invalido"); return; }
        std::vector<ColDesc> descs(ncols);
        for (int f = 0; f < ncols; f++)
            descs[f] = fieldDescOf(cli, wa, (short)f);
        g_api.GoTop(cli, wa);
        long long found = -1;
        for (;;) {
            if (g_api.Eof(cli, wa)) break;
            bool match = false;
            for (int f = 1; f <= ncols && !match; f++) {
                char* v = g_api.GetFieldN(cli, wa, (short)f);
                if (!v) continue;
                ColDesc& d = descs[f - 1];
                std::string val;
                if (d.type == "F") {
                    double x; memcpy(&x, v, 8);
                    char buf[64];
                    if (x == std::floor(x) && std::fabs(x) < 1e15)
                        snprintf(buf, sizeof buf, "%.0f", x);
                    else
                        snprintf(buf, sizeof buf, "%.10g", x);
                    val = buf;
                } else {
                    val = v;
                    if ((d.type == "C" || d.type == "M" || d.type.empty()) &&
                        d.size > 0 && (int)val.size() > d.size)
                        val.resize(d.size);
                }
                if (val == p2) match = true;
            }
            if (match) { found = g_api.RecNo(cli, wa); break; }
            g_api.Skip(cli, wa, 1);
        }
        respond(c.fd, 0, okFields({std::to_string(found)}));
        return;
    }
    respondErr(c.fd, "wacmd: comando desconhecido " + cmd);
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

        // lock por connID para ops que carregam connID no field 0
        // (2..6 e 8); CONNECT(1)/PING(7) não têm connID — CONNECT usa
        // o mutex global, PING não toca o dbapi.
        int lockId = -1;
        if ((op >= 2 && op <= 6) || op == 8) {
            lockId = atoi(fieldStr(in, 0).c_str());
        }
        static std::mutex g_noopMu;
        std::unique_lock<std::mutex> apiLock(
            op == 1 ? g_apiMu : (lockId > 0 ? lockFor(lockId) : g_noopMu));
        try {
            switch (op) {
                case 1: handleConnect(ctx, in); break;
                case 2: handleQuery(ctx, in); break;
                case 3: handleFetch(ctx, in); break;
                case 4: handleExec(ctx, in); break;
                case 5: handleCloseWa(ctx, in); break;
                case 6: handleDisconnect(ctx, in); break;
                case 7: respond(ctx.fd, 0, okFields({"pong"})); break;
                case 8: handleWaCmd(ctx, in); break;
                default: respondErr(ctx.fd, "opcode desconhecido"); break;
            }
        } catch (...) {
            // exceção C++ do dbapi (tAssertException etc.) — isola o processo
            // respondendo erro em vez de deixar propagar e matar o bridge.
            respondErr(ctx.fd, "excecao interna dbapi");
        }
    }
    // socket morreu sem DISCONNECT: libera as conexões deste fd para não
    // vazar licença/sessão no dbaccess64.
    {
        std::vector<int> ids;
        { std::lock_guard<std::mutex> lk(g_connsMu);
          auto it = g_fdConns.find(fd);
          if (it != g_fdConns.end()) { ids = it->second; g_fdConns.erase(it); } }
        for (int id : ids) {
            void* cli = nullptr;
            { std::lock_guard<std::mutex> lk(g_connsMu);
              auto it = g_conns.find(id);
              if (it != g_conns.end()) { cli = it->second; g_conns.erase(it); } }
            if (cli) {
                std::unique_lock<std::mutex> lk(lockFor(id));
                if (g_api.Release) g_api.Release(cli);
            }
        }
    }
    closesock(fd);
    return nullptr;
}

int main(int argc, char** argv) {
#ifdef _WIN32
    WSADATA wsa;
    if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0) { fprintf(stderr, "[bridge] WSAStartup falhou\n"); return 2; }
    const char* cands[] = {"dbapi.dll"}; // LoadLibrary já procura no dir do exe e no PATH
#else
    const char* cands[] = {"./dbapi.so", "/usr/lib/dbapi.so", "/usr/local/lib/dbapi.so"};
#endif
    if (argc > 2 && strcmp(argv[1], "--symbols") == 0) {
        g_listSyms = true;
        bool ok = loadDbApi(argv[2]);
        fprintf(stderr, "[bridge] %d símbolo(s) ausente(s)\n", g_missingSyms);
        return ok && g_missingSyms == 0 ? 0 : 1;
    }
    const char* so = argc > 1 ? argv[1] : getenv("ADVPP_DBAPI_SO");
    if (!so || !*so) {
        so = nullptr;
        for (auto c : cands) { if (openLib(c)) { so = c; break; } }
    }
    if (!so) { fprintf(stderr, "[bridge] dbapi nao encontrado\nuso: %s [dbapi.so|dbapi.dll] [porta]\n", argv[0]); return 2; }
    if (!loadDbApi(so)) return 2;
    fprintf(stderr, "[bridge] dbapi carregada de %s\n", so);

    int port = argc > 2 ? atoi(argv[2]) : 17890;
    if (const char* e = getenv("ADVPP_DBAPI_PORT")) port = atoi(e);

    int srv = (int)socket(AF_INET, SOCK_STREAM, 0);
#ifndef _WIN32
    // no Windows SO_REUSEADDR deixaria outro processo roubar a porta
    int one = 1; setsockopt(srv, SOL_SOCKET, SO_REUSEADDR, &one, sizeof one);
#endif
    sockaddr_in addr{}; addr.sin_family = AF_INET; addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK); addr.sin_port = htons((uint16_t)port);
    if (bind(srv, (sockaddr*)&addr, sizeof addr) != 0 || listen(srv, 16) != 0) {
        perror("[bridge] bind/listen"); return 2;
    }
    fprintf(stderr, "[bridge] escutando em 127.0.0.1:%d\n", port);

    for (;;) {
        int cfd = (int)accept(srv, nullptr, nullptr);
        if (cfd < 0) continue;
        try {
            std::thread([cfd] { clientThread((void*)(intptr_t)cfd); }).detach();
        } catch (...) {
            closesock(cfd);
        }
    }
    return 0;
}
