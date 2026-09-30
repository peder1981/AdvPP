#include <cstdio>
#include <cstring>
#include <dlfcn.h>
#include <sstream>
#include <iomanip>
#include <fstream>
#include <map>

static std::ofstream g_out;
static int g_event_counter = 0;
static std::map<void*, std::string> g_ctx_cipher;
static std::map<void*, std::string> g_ctx_key;
static std::map<void*, std::string> g_ctx_iv;

static std::string bytes_to_hex(const char* buf, int n) {
    std::ostringstream oss;
    for (int i = 0; i < n && i < 32; ++i)
        oss << std::hex << std::setfill('0') << std::setw(2) << (int)(unsigned char)buf[i];
    return oss.str();
}

static std::string identify_cipher(const void* cipher_ptr) {
    if (!cipher_ptr) return "unknown";
    
    auto test = [](const char* name) {
        return (void*(*)(void))dlsym(RTLD_DEFAULT, name);
    };
    
    #define CHECK(name) if (cipher_ptr == test("EVP_" #name)) return #name
    CHECK(des_cbc); CHECK(des_ecb); CHECK(des_ede_cbc);
    CHECK(rc4); CHECK(cast5_cbc); CHECK(bf_cbc);
    CHECK(rc2_cbc); CHECK(des_ede3_cbc);
    #undef CHECK
    
    return "unknown";
}

typedef void (*SetKeyFn)(void*, const char*, int, const char*, int, const char*);
static SetKeyFn orig_setkey = nullptr;

extern "C" void hooked_SetKey(void* self, const char* key, int keylen,
                               const char* iv, int ivlen, const char* arg5)
    asm("_ZN10tCryptoEVP6SetKeyEPKciS1_iS1_");

void hooked_SetKey(void* self, const char* key, int keylen,
                    const char* iv, int ivlen, const char* arg5) {
    static bool installed = false;
    if (!installed) {
        orig_setkey = (SetKeyFn)dlsym(RTLD_NEXT, "_ZN10tCryptoEVP6SetKeyEPKciS1_iS1_");
        installed = true;
        fprintf(stderr, "[COMPLETE_HOOK] SetKey installed\n");
    }
    
    if (orig_setkey) orig_setkey(self, key, keylen, iv, ivlen, arg5);
    
    if (g_out.is_open()) {
        g_out << "  {\"n\": " << ++g_event_counter 
              << ", \"type\": \"setkey\","
              << "\"key\": \"" << bytes_to_hex(key, keylen) << "\","
              << "\"iv\": \"" << bytes_to_hex(iv, ivlen) << "\""
              << "},\n";
        g_out.flush();
    }
}

typedef int (*EVPInitFn)(void*, const void*, void*, const unsigned char*, const unsigned char*);
static EVPInitFn orig_evpinit = nullptr;

extern "C" int EVP_EncryptInit_ex(void* ctx, const void* cipher, void* impl,
                                   const unsigned char* key, const unsigned char* iv) {
    static bool installed = false;
    if (!installed) {
        orig_evpinit = (EVPInitFn)dlsym(RTLD_NEXT, "EVP_EncryptInit_ex");
        installed = true;
        fprintf(stderr, "[COMPLETE_HOOK] EVP_Init installed\n");
    }
    
    int ret = orig_evpinit ? orig_evpinit(ctx, cipher, impl, key, iv) : 0;
    
    std::string cname = identify_cipher(cipher);
    g_ctx_cipher[ctx] = cname;
    if (key) g_ctx_key[ctx] = bytes_to_hex((const char*)key, 16);
    if (iv) g_ctx_iv[ctx] = bytes_to_hex((const char*)iv, 8);
    
    if (g_out.is_open()) {
        g_out << "  {\"n\": " << ++g_event_counter 
              << ", \"type\": \"evpinit\","
              << "\"cipher\": \"" << cname << "\","
              << "\"key\": \"" << g_ctx_key[ctx] << "\","
              << "\"iv\": \"" << g_ctx_iv[ctx] << "\""
              << "},\n";
        g_out.flush();
    }
    
    return ret;
}

typedef int (*EVPUpdateFn)(void*, unsigned char*, int*, const unsigned char*, int);
static EVPUpdateFn orig_evpupdate = nullptr;

extern "C" int EVP_EncryptUpdate(void* ctx, unsigned char* out, int* outl,
                                  const unsigned char* in, int inl) {
    static bool installed = false;
    if (!installed) {
        orig_evpupdate = (EVPUpdateFn)dlsym(RTLD_NEXT, "EVP_EncryptUpdate");
        installed = true;
        fprintf(stderr, "[COMPLETE_HOOK] EVP_Update installed\n");
    }
    
    int ret = orig_evpupdate ? orig_evpupdate(ctx, out, outl, in, inl) : 0;
    
    if (g_out.is_open() && in && inl > 0) {
        g_out << "  {\"n\": " << ++g_event_counter 
              << ", \"type\": \"encrypt\","
              << "\"plaintext\": \"" << bytes_to_hex((const char*)in, inl) << "\""
              << "},\n";
        g_out.flush();
    }
    
    return ret;
}

extern "C" __attribute__((constructor))
void init() {
    g_out.open("/tmp/rpo_complete_capture.json", std::ios::app);
    if (g_out.is_open()) {
        g_out << "[\n";
        fprintf(stderr, "[COMPLETE_HOOK] Loaded! Output: /tmp/rpo_complete_capture.json\n");
    }
}

extern "C" __attribute__((destructor))
void fini() {
    if (g_out.is_open()) {
        g_out << "]\n";
        g_out.close();
        fprintf(stderr, "[COMPLETE_HOOK] Closed\n");
    }
}
