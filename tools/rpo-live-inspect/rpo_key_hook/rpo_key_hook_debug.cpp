// Debug version - logs everything to stderr
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>

#define REAL_LIB "libcrypto.so.3"

typedef int (*evp_init_ex_t)(void* ctx, void* cipher, void* impl, unsigned char* key, unsigned char* iv);
typedef int (*evp_update_t)(void* ctx, unsigned char* out, int* outlen, const unsigned char* in, int inlen);
typedef void* (*ctx_get_cipher_t)(void* ctx);
typedef const char* (*cipher_get_name_t)(void* cipher);
typedef int (*ctx_get_keylen_t)(void* ctx);
typedef int (*ctx_get_ivlen_t)(void* ctx);

static evp_init_ex_t g_orig_evpinit = nullptr;
static evp_update_t g_orig_evputUpdate = nullptr;
static ctx_get_cipher_t g_orig_ctx_get_cipher = nullptr;
static cipher_get_name_t g_orig_cipher_get_name = nullptr;
static ctx_get_keylen_t g_orig_ctx_get_keylen = nullptr;
static ctx_get_ivlen_t g_orig_ctx_get_ivlen = nullptr;

static void hex_dump(const char* label, const unsigned char* data, int len) {
    fprintf(stderr, "[HOOK] %s (%d bytes): ", label, len);
    for (int i = 0; i < len && i < 32; i++)
        fprintf(stderr, "%02x", data[i]);
    if (len > 32) fprintf(stderr, "...");
    fprintf(stderr, "\n");
}

__attribute__((constructor))
static void init_hook() {
    fprintf(stderr, "[HOOK] Loading...\n");
    
    void* lib = dlopen(REAL_LIB, RTLD_LAZY);
    if (!lib) {
        fprintf(stderr, "[HOOK] ERROR: %s\n", dlerror());
        return;
    }
    
    g_orig_evpinit = (evp_init_ex_t)dlsym(lib, "EVP_EncryptInit_ex");
    g_orig_evputUpdate = (evp_update_t)dlsym(lib, "EVP_EncryptUpdate");
    g_orig_ctx_get_cipher = (ctx_get_cipher_t)dlsym(lib, "EVP_CIPHER_CTX_get0_cipher");
    g_orig_cipher_get_name = (cipher_get_name_t)dlsym(lib, "EVP_CIPHER_get0_name");
    g_orig_ctx_get_keylen = (ctx_get_keylen_t)dlsym(lib, "EVP_CIPHER_CTX_get_key_length");
    g_orig_ctx_get_ivlen = (ctx_get_ivlen_t)dlsym(lib, "EVP_CIPHER_CTX_get_iv_length");
    
    fprintf(stderr, "[HOOK] Loaded:\n");
    fprintf(stderr, "  EVP_EncryptInit_ex: %p\n", g_orig_evpinit);
    fprintf(stderr, "  EVP_EncryptUpdate: %p\n", g_orig_evputUpdate);
    fprintf(stderr, "  EVP_CIPHER_CTX_get0_cipher: %p\n", g_orig_ctx_get_cipher);
    fprintf(stderr, "  EVP_CIPHER_get0_name: %p\n", g_orig_cipher_get_name);
    fprintf(stderr, "  EVP_CIPHER_CTX_get_key_length: %p\n", g_orig_ctx_get_keylen);
    fprintf(stderr, "  EVP_CIPHER_CTX_get_iv_length: %p\n", g_orig_ctx_get_ivlen);
}

extern "C" {

int EVP_EncryptInit_ex(void* ctx, void* cipher, void* impl, unsigned char* key, unsigned char* iv) {
    if (g_orig_evpinit) {
        int ret = g_orig_evpinit(ctx, cipher, impl, key, iv);
        
        const char* cname = "unknown";
        if (g_orig_cipher_get_name && cipher) {
            cname = g_orig_cipher_get_name(cipher);
            if (!cname || strlen(cname) == 0) cname = "unknown";
        }
        
        int key_len = g_orig_ctx_get_keylen ? g_orig_ctx_get_keylen(ctx) : 0;
        int iv_len = g_orig_ctx_get_ivlen ? g_orig_ctx_get_ivlen(ctx) : 0;
        
        fprintf(stderr, "[HOOK] EVP_EncryptInit_ex: cipher=%s key_len=%d iv_len=%d\n", 
                cname, key_len, iv_len);
        
        if (key && key_len > 0) {
            hex_dump("KEY", key, key_len);
        }
        if (iv && iv_len > 0) {
            hex_dump("IV", iv, iv_len);
        }
        
        return ret;
    }
    return 0;
}

int EVP_EncryptUpdate(void* ctx, unsigned char* out, int* outlen, const unsigned char* in, int inlen) {
    if (g_orig_evputUpdate) {
        return g_orig_evputUpdate(ctx, out, outlen, in, inlen);
    }
    return 0;
}

} // extern "C"
