// Enhanced RPO key hook v6 - robust version
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>

#define REAL_LIB "libcrypto.so.3"

typedef int (*evp_init_ex_t)(void* ctx, void* cipher, void* impl, unsigned char* key, unsigned char* iv);
typedef int (*evp_update_t)(void* ctx, unsigned char* out, int* outlen, const unsigned char* in, int inlen);

static evp_init_ex_t g_orig_evpinit = nullptr;
static evp_update_t g_orig_evputUpdate = nullptr;
static FILE* g_log = nullptr;
static int g_event_n = 0;

__attribute__((constructor))
static void init_hook() {
    // Open log file
    const char* path = getenv("RPO_KEYS_EXPORT");
    if (!path) path = "/tmp/rpo_keys_v6.json";
    
    g_log = fopen(path, "w");
    if (g_log) fprintf(g_log, "[\n");
    
    void* lib = dlopen(REAL_LIB, RTLD_LAZY);
    if (lib) {
        g_orig_evpinit = (evp_init_ex_t)dlsym(lib, "EVP_EncryptInit_ex");
        g_orig_evputUpdate = (evp_update_t)dlsym(lib, "EVP_EncryptUpdate");
        
        if (g_log) {
            fprintf(g_log, "  {\"type\": \"init\", \"hooks\": {\n");
            fprintf(g_log, "    \"EVP_EncryptInit_ex\": \"%p\",\n", g_orig_evpinit);
            fprintf(g_log, "    \"EVP_EncryptUpdate\": \"%p\"\n", g_orig_evputUpdate);
            fprintf(g_log, "  }}\n");
            fflush(g_log);
        }
        
        fprintf(stderr, "[HOOK-v6] Loaded hooks\n");
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
        
        if (g_log) {
            g_event_n++;
            fprintf(g_log, "  {\"n\": %d, \"type\": \"init\"", g_event_n);
            fprintf(g_log, ", \"key_len\": %ld", key ? 16 : 0);
            fprintf(g_log, ", \"key_ptr\": %p", key);
            fprintf(g_log, ", \"iv_ptr\": %p", iv);
            
            if (key) {
                fprintf(g_log, ", \"key\": \"");
                for (int i = 0; i < 32; i++)
                    fprintf(g_log, "%02x", key[i]);
                fprintf(g_log, "\"");
            }
            
            if (iv) {
                fprintf(g_log, ", \"iv\": \"");
                for (int i = 0; i < 16; i++)
                    fprintf(g_log, "%02x", iv[i]);
                fprintf(g_log, "\"");
            }
            
            fprintf(g_log, "},\n");
            fflush(g_log);
        }
        
        if (key) {
            fprintf(stderr, "[HOOK-v6] InitEx: key_len=16 key=%p\n", key);
            for (int i = 0; i < 16; i++)
                fprintf(stderr, "%02x", key[i]);
            fprintf(stderr, "\n");
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
