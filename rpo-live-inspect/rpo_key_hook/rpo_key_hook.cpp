// rpo_key_hook.cpp — LD_PRELOAD hook para capturar ao vivo a chave/IV
// mestre (tCryptoEVP::SetKey) e, para CADA operação real de cifra
// (EVP_EncryptInit_ex + EVP_EncryptUpdate, ambos símbolos C planos
// exportados por libaplinux.so — OpenSSL está estaticamente embutido,
// não é uma libcrypto.so separada), a chave/IV/cifra REALMENTE usados e
// o plaintext correspondente. Ver docs/rpo-format-sonnet.md para a
// investigação (gdb) que este hook automatiza sem precisar de gdb.
//
// Uso:
//   g++ -shared -fPIC -o rpo_key_hook.so rpo_key_hook.cpp -ldl -std=c++17
//   LD_PRELOAD=./rpo_key_hook.so appsrvlinux -compile ...
//
// Saída: /tmp/rpo_keys_export.json — lista de eventos, formato
// compatível com pkg/rpo/cipher_dispatch_test.go (campos "type"/"n"/
// "cipher"/"key"/"iv"/"plaintext"), sobrescrita a cada evento novo
// (append lógico, não apenas a última chave).
//
// ---------------------------------------------------------------
// [!] CORREÇÃO (2026-09-19): a versão anterior deste arquivo NUNCA
// funcionou. Ela definia a função de interceptação com o nome C puro
// `tCryptoEVP_SetKey` — mas LD_PRELOAD só substitui uma chamada quando
// o SÍMBOLO tem o MESMO NOME que o binário original chama, e
// `tCryptoEVP::SetKey` é uma função C++, cujo nome real (mangled,
// Itanium ABI) é outro completamente diferente. Um `extern "C"` com
// nome arbitrário não intercepta nada — o hook nunca era chamado.
// Além disso, as 3 tentativas de fallback em `dlsym()` para achar a
// função original usavam nomes mangled INVENTADOS (com o comprimento
// errado do nome da classe/método, faltando "6SetKey" com o tamanho
// certo, etc.) — nenhuma batia com o símbolo real, então mesmo se o
// hook fosse chamado, nunca encontraria a função original pra
// repassar a chamada, quebrando o appserver em vez de só observá-lo.
//
// Nome mangled REAL, confirmado via
// `readelf --dyn-syms -W libaplinux.so | grep tCryptoEVP.*SetKey`:
//   _ZN10tCryptoEVP6SetKeyEPKciS1_iS1_
// que corresponde a `tCryptoEVP::SetKey(char const*, int, char const*,
// int, char const*)` — 5 parâmetros, ABI SysV x86-64 com `this` em
// rdi: rsi=key, rdx=keylen, rcx=iv, r8=ivlen, r9=quinto parâmetro
// (sempre NULL em todas as capturas observadas, propósito desconhecido
// — NÃO é "cipher"/"cipherlen" como a versão anterior deste arquivo
// afirmava, e o IV NÃO é sempre null — é o oposto: o quinto parâmetro
// é que é sempre null, o IV é real e não-nulo na grande maioria dos
// casos observados).
// ---------------------------------------------------------------

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <cstdint>
#include <ctime>
#include <dlfcn.h>
#include <mutex>
#include <sstream>
#include <iomanip>
#include <map>
#include <vector>

// ===================== utilitários =====================

static std::mutex g_mtx;
static int g_event_counter = 0;

// Caminho de saída — configurável via RPO_KEYS_OUTPUT (usado por
// tools/build-integration/advpl-build-wrapper.sh), com o padrão de
// sempre /tmp/rpo_keys_export.json se a variável não estiver setada.
static std::string output_path() {
    const char* env = getenv("RPO_KEYS_OUTPUT");
    return (env && env[0]) ? std::string(env) : std::string("/tmp/rpo_keys_export.json");
}

static std::string bytes_to_hex(const unsigned char* buf, int n) {
    std::ostringstream oss;
    for (int i = 0; i < n; ++i)
        oss << std::hex << std::setfill('0') << std::setw(2) << (int)buf[i];
    return oss.str();
}
static std::string bytes_to_hex(const char* buf, int n) {
    return bytes_to_hex(reinterpret_cast<const unsigned char*>(buf), n);
}

static std::string json_escape(const std::string& s) {
    std::string out;
    for (char c : s) {
        switch (c) {
            case '"':  out += "\\\""; break;
            case '\\': out += "\\\\"; break;
            case '\n': out += "\\n"; break;
            case '\r': out += "\\r"; break;
            case '\t': out += "\\t"; break;
            default:   out += c;
        }
    }
    return out;
}

// escreve o arquivo de saída inteiro a cada evento (atomically via
// write-then-rename) — simples e robusto, sem precisar reabrir/parsear
// o JSON existente a cada chamada.
struct Event {
    int n;
    std::string type;   // "setkey" | "evpinit" | "encrypt" | "rsakey"
    std::string cipher; // só em "evpinit"
    std::string key, iv, plaintext;
    std::string password;    // só em "rsakey" — ex.: "manezinho"
    std::string private_pem; // só em "rsakey" — PEM criptografado (DES-EDE3-CBC), texto com \n reais
    std::string public_pem;  // só em "rsakey"
};
static std::vector<Event> g_events;

static void flush_events() {
    std::ostringstream j;
    j << "[\n";
    for (size_t i = 0; i < g_events.size(); ++i) {
        const Event& e = g_events[i];
        j << "  {\"n\": " << e.n << ", \"type\": \"" << json_escape(e.type) << "\"";
        if (!e.cipher.empty())    j << ", \"cipher\": \"" << json_escape(e.cipher) << "\"";
        if (!e.key.empty())       j << ", \"key\": \"" << e.key << "\"";
        if (!e.iv.empty())        j << ", \"iv\": \"" << e.iv << "\"";
        if (!e.plaintext.empty()) j << ", \"plaintext\": \"" << e.plaintext << "\"";
        if (!e.password.empty())    j << ", \"password\": \"" << json_escape(e.password) << "\"";
        if (!e.private_pem.empty()) j << ", \"private_pem\": \"" << json_escape(e.private_pem) << "\"";
        if (!e.public_pem.empty())  j << ", \"public_pem\": \"" << json_escape(e.public_pem) << "\"";
        j << "}" << (i + 1 < g_events.size() ? "," : "") << "\n";
    }
    j << "]\n";

    std::string out = output_path();
    std::string tmp = out + ".tmp";
    FILE* fp = fopen(tmp.c_str(), "w");
    if (!fp) {
        std::fprintf(stderr, "[rpo_key_hook] ERRO: não pôde escrever %s: %s\n", tmp.c_str(), strerror(errno));
        return;
    }
    std::fwrite(j.str().c_str(), 1, j.str().size(), fp);
    fclose(fp);
    rename(tmp.c_str(), out.c_str());
}

// ===================== hook 1: tCryptoEVP::SetKey =====================
// Nome C++ mangled real — usar __asm__ para forçar o linker a emitir
// esta função sob o símbolo exato que o appserver chama (a única forma
// portável de fazer LD_PRELOAD interceptar um símbolo C++ mangled).

typedef void (*SetKeyFn)(void*, const char*, int, const char*, int, const char*);
static SetKeyFn g_orig_setkey = nullptr;
static bool g_orig_setkey_resolved = false;

extern "C" void hooked_SetKey(void* self, const char* key, int keylen,
                               const char* iv, int ivlen, const char* arg5)
    asm("_ZN10tCryptoEVP6SetKeyEPKciS1_iS1_");

void hooked_SetKey(void* self, const char* key, int keylen,
                    const char* iv, int ivlen, const char* arg5) {
    if (!g_orig_setkey_resolved) {
        std::lock_guard<std::mutex> lock(g_mtx);
        if (!g_orig_setkey_resolved) {
            g_orig_setkey = (SetKeyFn)dlsym(RTLD_NEXT, "_ZN10tCryptoEVP6SetKeyEPKciS1_iS1_");
            g_orig_setkey_resolved = true;
            if (!g_orig_setkey) {
                std::fprintf(stderr,
                    "[rpo_key_hook] AVISO: dlsym não achou a SetKey original — "
                    "isso quebraria o appserver se acontecesse (não chamaríamos "
                    "a implementação real). Não deveria acontecer: o símbolo é "
                    "verificado via readelf, ver comentário no topo do arquivo.\n");
            }
        }
    }

    {
        std::lock_guard<std::mutex> lock(g_mtx);
        Event e;
        e.n = ++g_event_counter;
        e.type = "setkey";
        if (keylen > 0) e.key = bytes_to_hex(key, keylen);
        if (ivlen > 0)  e.iv  = bytes_to_hex(iv, ivlen);
        g_events.push_back(e);
        flush_events();
        std::fprintf(stderr, "[rpo_key_hook] SetKey: key(%dB)=%s iv(%dB)=%s arg5=%s\n",
                     keylen, e.key.c_str(), ivlen, e.iv.c_str(),
                     arg5 ? "presente (inesperado — investigar)" : "NULL (esperado)");
    }

    if (g_orig_setkey) {
        g_orig_setkey(self, key, keylen, iv, ivlen, arg5);
    }
    // Se g_orig_setkey for null, NÃO HÁ COMO prosseguir sem quebrar o
    // appserver — mas isso indicaria o símbolo real mudou de nome
    // (build diferente), não um bug deste hook. Preferível travar de
    // forma visível a mascarar silenciosamente.
}

// ===================== hook 1b: tCryptoRSA::SetKey =====================
// Nome mangled real confirmado via readelf:
//   _ZN10tCryptoRSA6SetKeyEPKcS1_S1_
// = tCryptoRSA::SetKey(char const*, char const*, char const*) — 3
// parâmetros char*, this em rdi: rsi=chave privada PEM (criptografada
// DES-EDE3-CBC), rdx=chave pública PEM correspondente, rcx=SENHA da
// chave privada em texto puro (ver docs/rpo-format.md, Fase 14 — a
// senha real observada em todas as capturas até hoje é "manezinho").
// Esta chave RSA-4096 é real, mas busca exaustiva (Fase 15) não achou
// nenhuma relação dela com a cifra do conteúdo do RPO — capturamos
// aqui por completude/registro, não porque ela decodifique o RPO.

typedef void (*RSASetKeyFn)(void*, const char*, const char*, const char*);
static RSASetKeyFn g_orig_rsa_setkey = nullptr;
static bool g_orig_rsa_setkey_resolved = false;

extern "C" void hooked_RSASetKey(void* self, const char* priv_pem,
                                  const char* pub_pem, const char* password)
    asm("_ZN10tCryptoRSA6SetKeyEPKcS1_S1_");

void hooked_RSASetKey(void* self, const char* priv_pem,
                       const char* pub_pem, const char* password) {
    if (!g_orig_rsa_setkey_resolved) {
        std::lock_guard<std::mutex> lock(g_mtx);
        if (!g_orig_rsa_setkey_resolved) {
            g_orig_rsa_setkey = (RSASetKeyFn)dlsym(RTLD_NEXT, "_ZN10tCryptoRSA6SetKeyEPKcS1_S1_");
            g_orig_rsa_setkey_resolved = true;
        }
    }

    {
        std::lock_guard<std::mutex> lock(g_mtx);
        Event e;
        e.n = ++g_event_counter;
        e.type = "rsakey";
        if (priv_pem) e.private_pem = priv_pem;
        if (pub_pem)  e.public_pem = pub_pem;
        if (password) e.password = password;
        g_events.push_back(e);
        flush_events();
        std::fprintf(stderr, "[rpo_key_hook] tCryptoRSA::SetKey: password=%s\n",
                     password ? password : "(null)");
    }

    if (g_orig_rsa_setkey) {
        g_orig_rsa_setkey(self, priv_pem, pub_pem, password);
    }
}

// ===================== hook 2: EVP_EncryptInit_ex =====================
// Símbolo C puro, exportado diretamente por libaplinux.so (OpenSSL
// embutido estaticamente) — LD_PRELOAD funciona da forma normal aqui,
// sem os problemas de mangling do hook 1.

typedef int (*EVPInitFn)(void*, const void*, void*, const unsigned char*, const unsigned char*);
static EVPInitFn g_orig_evpinit = nullptr;
static bool g_orig_evpinit_resolved = false;

// candidatos conhecidos do OpenSSL para identificar a cifra pelo
// ponteiro retornado (mesma técnica usada em
// pkg/rpo/cipher_dispatch.go / docs/rpo-format-sonnet.md via gdb) —
// resolvidos uma vez, comparados por igualdade de ponteiro.
struct CipherCandidate { const char* name; void* (*ctor)(void); };
typedef void* (*CipherCtorFn)(void);

static std::map<const void*, std::string> g_cipher_names; // preenchido lazy
static std::map<const void*, int> g_ctx_to_n;              // ctx -> índice do evento evpinit correspondente
static std::map<const void*, std::string> g_ctx_key, g_ctx_iv;

static void resolve_cipher_names() {
    static const char* names[] = {
        "EVP_des_cbc", "EVP_des_ecb", "EVP_des_ofb", "EVP_des_cfb64",
        "EVP_des_ede_cbc", "EVP_des_ede_ecb", "EVP_des_ede_ofb", "EVP_des_ede_cfb64",
        "EVP_rc4",
        "EVP_rc2_cbc", "EVP_rc2_ecb", "EVP_rc2_ofb", "EVP_rc2_cfb64",
        "EVP_rc5_32_12_16_cbc", "EVP_rc5_32_12_16_ecb", "EVP_rc5_32_12_16_ofb", "EVP_rc5_32_12_16_cfb64",
        "EVP_cast5_cbc", "EVP_cast5_ecb", "EVP_cast5_ofb", "EVP_cast5_cfb64",
        "EVP_bf_cbc", "EVP_bf_ecb", "EVP_bf_ofb", "EVP_bf_cfb64",
        "EVP_idea_cbc", "EVP_idea_ecb", "EVP_idea_ofb", "EVP_idea_cfb64",
        nullptr,
    };
    for (int i = 0; names[i]; ++i) {
        CipherCtorFn f = (CipherCtorFn)dlsym(RTLD_DEFAULT, names[i]);
        if (f) {
            void* ptr = f();
            // nome do dispatcher: "des_cbc_cipher" (minusculo, sem "EVP_", "_cipher" no fim)
            std::string n(names[i] + 4); // remove "EVP_"
            n += "_cipher";
            g_cipher_names[ptr] = n;
        }
    }
}

static std::string identify_cipher(const void* cipher_ptr) {
    if (g_cipher_names.empty()) resolve_cipher_names();
    auto it = g_cipher_names.find(cipher_ptr);
    if (it != g_cipher_names.end()) return it->second;
    return "desconhecido";
}

extern "C" int EVP_EncryptInit_ex(void* ctx, const void* cipher, void* impl,
                                   const unsigned char* key, const unsigned char* iv) {
    if (!g_orig_evpinit_resolved) {
        std::lock_guard<std::mutex> lock(g_mtx);
        if (!g_orig_evpinit_resolved) {
            g_orig_evpinit = (EVPInitFn)dlsym(RTLD_NEXT, "EVP_EncryptInit_ex");
            g_orig_evpinit_resolved = true;
        }
    }

    int ret = g_orig_evpinit ? g_orig_evpinit(ctx, cipher, impl, key, iv) : 0;

    {
        std::lock_guard<std::mutex> lock(g_mtx);
        std::string cname = identify_cipher(cipher);
        std::string khex, ivhex;
        // Sem EVP_CIPHER_key_length/iv_length aqui pra evitar mais uma
        // dependência de símbolo — usamos os tamanhos já conhecidos por
        // convenção observada (16 bytes de chave, 8 de IV cobre todas
        // as cifras reais confirmadas exceto DES puro/RC4, que usam
        // menos bytes da mesma chave; capturar 16/8 sempre e deixar o
        // consumidor (pkg/rpo) truncar é seguro e simples).
        if (key) khex = bytes_to_hex(reinterpret_cast<const char*>(key), 16);
        if (iv)  ivhex = bytes_to_hex(reinterpret_cast<const char*>(iv), 8);

        Event e;
        e.n = ++g_event_counter;
        e.type = "evpinit";
        e.cipher = cname;
        e.key = khex;
        e.iv = ivhex;
        g_events.push_back(e);
        g_ctx_to_n[ctx] = e.n;
        g_ctx_key[ctx] = khex;
        g_ctx_iv[ctx] = ivhex;
        flush_events();
        std::fprintf(stderr, "[rpo_key_hook] EVP_EncryptInit_ex: cipher=%s key=%s iv=%s\n",
                     cname.c_str(), khex.c_str(), ivhex.c_str());
    }
    return ret;
}

// ===================== hook 3: EVP_EncryptUpdate =====================
// Captura o plaintext de entrada — junto com o hook 2 (correlacionado
// pelo ponteiro de contexto `ctx`), reproduz exatamente os mesmos três
// dados (cipher, key/iv, plaintext) que os scripts gdb desta
// investigação capturavam manualmente.

typedef int (*EVPUpdateFn)(void*, unsigned char*, int*, const unsigned char*, int);
static EVPUpdateFn g_orig_evpupdate = nullptr;
static bool g_orig_evpupdate_resolved = false;

extern "C" int EVP_EncryptUpdate(void* ctx, unsigned char* out, int* outl,
                                  const unsigned char* in, int inl) {
    if (!g_orig_evpupdate_resolved) {
        std::lock_guard<std::mutex> lock(g_mtx);
        if (!g_orig_evpupdate_resolved) {
            g_orig_evpupdate = (EVPUpdateFn)dlsym(RTLD_NEXT, "EVP_EncryptUpdate");
            g_orig_evpupdate_resolved = true;
        }
    }

    int ret = g_orig_evpupdate ? g_orig_evpupdate(ctx, out, outl, in, inl) : 0;

    if (in && inl > 0 && inl <= 65536) {
        std::lock_guard<std::mutex> lock(g_mtx);
        Event e;
        e.n = ++g_event_counter;
        e.type = "encrypt";
        e.plaintext = bytes_to_hex(reinterpret_cast<const char*>(in), inl);
        auto itKey = g_ctx_key.find(ctx);
        auto itIv = g_ctx_iv.find(ctx);
        if (itKey != g_ctx_key.end()) e.key = itKey->second;
        if (itIv != g_ctx_iv.end())   e.iv = itIv->second;
        g_events.push_back(e);
        flush_events();
    }
    return ret;
}

__attribute__((constructor))
static void hook_register(void) {
    std::fprintf(stderr,
        "[rpo_key_hook] hooks ativos: tCryptoEVP::SetKey, tCryptoRSA::SetKey, "
        "EVP_EncryptInit_ex, EVP_EncryptUpdate -> %s\n", output_path().c_str());
}
