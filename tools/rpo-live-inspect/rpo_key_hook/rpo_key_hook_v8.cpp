// Enhanced RPO key hook v8 - forçamos captura de 32 bytes da key
// Ignora o key_length retornado pelo contexto e lemos diretamente
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>

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

static FILE* g_log = nullptr;
static char g_json_path[512] = "/tmp/rpo_keys.json";
static int g_event_n = 0;

__attribute__((constructor))
static void init_hook() {
    const char* env = getenv("RPO_KEYS_EXPORT");
    if (env) strncpy(g_json_path, env, sizeof(g_json_path)-1);
    
    g_log = fopen(g_json_path, "w");
    if (g_log) fprintf(g_log, "[\n");
    
    void* lib = dlopen(REAL_LIB, RTLD_LAZY);
    if (lib) {
        g_orig_evpinit = (evp_init_ex_t)dlsym(lib, "EVP_EncryptInit_ex");
        g_orig_evputUpdate = (evp_update_t)dlsym(lib, "EVP_EncryptUpdate");
        g_orig_ctx_get_cipher = (ctx_get_cipher_t)dlsym(lib, "EVP_CIPHER_CTX_get0_cipher");
        g_orig_cipher_get_name = (cipher_get_name_t)dlsym(lib, "EVP_CIPHER_get0_name");
        g_orig_ctx_get_keylen = (ctx_get_keylen_t)dlsym(lib, "EVP_CIPHER_CTX_get_key_length");
        g_orig_ctx_get_ivlen = (ctx_get_ivlen_t)dlsym(lib, "EVP_CIPHER_CTX_get_iv_length");
        
        fprintf(stderr, "[HOOK-v8] Hooks carregados:\n");
        fprintf(stderr, "  EVP_EncryptInit_ex: %p\n", g_orig_evpinit);
        fprintf(stderr, "  EVP_CIPHER_CTX_get_key_length: %p\n", g_orig_ctx_get_keylen);
        fprintf(stderr, "  Output: %s\n", g_json_path);
    }
}

__attribute__((destructor))
static void fini_hook() {
    if (g_log) {
        fprintf(g_log, "]\n");
        fclose(g_log);
    }
}

extern "C" {

int EVP_EncryptInit_ex(void* ctx, void* cipher, void* impl, unsigned char* key, unsigned char* iv) {
    if (g_orig_evpinit) {
        int ret = g_orig_evpinit(ctx, cipher, impl, key, iv);
        
        // Obter nome da cifra
        const char* cname = "unknown";
        if (g_orig_cipher_get_name && cipher) {
            cname = g_orig_cipher_get_name(cipher);
            if (!cname || strlen(cname) == 0) cname = "unknown";
        }
        
        // Obter key_length do contexto
        int reported_keylen = g_orig_ctx_get_keylen ? g_orig_ctx_get_keylen(ctx) : 0;
        int reported_ivlen = g_orig_ctx_get_ivlen ? g_orig_ctx_get_ivlen(ctx) : 0;
        
        // FORÇAR: capturar sempre 32 bytes se key for AES-256
        int key_len_to_capture = 16; // default
        if (key) {
            if (strstr(cname, "aes-256") || strstr(cname, "aes_256")) {
                key_len_to_capture = 32;
            } else if (strstr(cname, "aes-192") || strstr(cname, "aes_192")) {
                key_len_to_capture = 24;
            } else if (strstr(cname, "aes-128") || strstr(cname, "aes_128")) {
                key_len_to_capture = 16;
            } else {
                // Usar reported ou default
                key_len_to_capture = reported_keylen > 0 ? reported_keylen : 16;
            }
        }
        
        // Limitar a 64 bytes máximo
        if (key_len_to_capture > 64) key_len_to_capture = 64;
        if (reported_keylen > 0 && reported_keylen < key_len_to_capture) {
            // Se o contexto diz que é menor, confiar no contexto
            key_len_to_capture = reported_keylen;
        }
        
        int iv_len = reported_ivlen > 0 && reported_ivlen <= 32 ? reported_ivlen : 0;
        
        // Capturar key
        unsigned char key_buf[64] = {};
        unsigned char iv_buf[32] = {};
        char key_hex[129] = {}, iv_hex[65] = {};
        
        if (key && key_len_to_capture > 0) {
            memcpy(key_buf, key, key_len_to_capture);
            for (int i = 0; i < key_len_to_capture && i < 64; i++)
                sprintf(key_hex + strlen(key_hex), "%02x", key_buf[i]);
        }
        
        if (iv && iv_len > 0) {
            memcpy(iv_buf, iv, iv_len);
            for (int i = 0; i < iv_len && i < 32; i++)
                sprintf(iv_hex + strlen(iv_hex), "%02x", iv_buf[i]);
        }
        
        // Log
        fprintf(stderr, "[HOOK-v8] InitEx: cipher=%s reported_keylen=%d captured_keylen=%d key=%s\n",
                cname, reported_keylen, key_len_to_capture, key_hex);
        
        // Escrever no JSON
        if (g_log) {
            g_event_n++;
            fprintf(g_log, "  {\"n\": %d, \"type\": \"init\", \"cipher\": \"%s\",", 
                    g_event_n, cname);
            fprintf(g_log, "\"reported_key_len\": %d, \"captured_key_len\": %d,",
                    reported_keylen, key_len_to_capture);
            fprintf(g_log, "\"key\": \"%s\", \"iv\": \"%s\"},\n", key_hex, iv_hex);
            fflush(g_log);
        }
        
        return ret;
    }
    return 0;
}

int EVP_EncryptUpdate(void* ctx, unsigned char* out, int* outlen, const unsigned char* in, int inlen) {
    if (g_orig_evputUpdate)
        return g_orig_evputUpdate(ctx, out, outlen, in, inlen);
    return 0;
}

} // extern "C"
