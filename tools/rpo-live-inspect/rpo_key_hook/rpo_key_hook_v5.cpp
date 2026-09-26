// Enhanced RPO key hook v5 - debug version
// Captura key do parâmetro diretamente
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>

#define REAL_LIB "libcrypto.so.3"

typedef int (*evp_init_ex_t)(void* ctx, void* cipher, void* impl, unsigned char* key, unsigned char* iv);
typedef int (*evp_update_t)(void* ctx, unsigned char* out, int* outlen, const unsigned char* in, int inlen);

static evp_init_ex_t g_orig_evpinit = nullptr;
static evp_update_t g_orig_evputUpdate = nullptr;

static char g_json_path[512] = "/tmp/rpo_keys.json";
static int g_event_n = 0;

__attribute__((constructor))
static void init_hook() {
    const char* env = getenv("RPO_KEYS_EXPORT");
    if (env) strncpy(g_json_path, env, sizeof(g_json_path)-1);
    
    void* lib = dlopen(REAL_LIB, RTLD_LAZY);
    if (lib) {
        g_orig_evpinit = (evp_init_ex_t)dlsym(lib, "EVP_EncryptInit_ex");
        g_orig_evputUpdate = (evp_update_t)dlsym(lib, "EVP_EncryptUpdate");
        fprintf(stderr, "[HOOK-v5] EVP_EncryptInit_ex: %p\n", g_orig_evpinit);
        fprintf(stderr, "[HOOK-v5] EVP_EncryptUpdate: %p\n", g_orig_evputUpdate);
    }
}

extern "C" {

int EVP_EncryptInit_ex(void* ctx, void* cipher, void* impl, unsigned char* key, unsigned char* iv) {
    if (g_orig_evpinit) {
        int ret = g_orig_evpinit(ctx, cipher, impl, key, iv);
        
        // Log key e iv
        fprintf(stderr, "[HOOK-v5] InitEx: key=%p iv=%p\n", key, iv);
        if (key) {
            fprintf(stderr, "[HOOK-v5] Key bytes: ");
            for (int i = 0; i < 32 && i < 16; i++)
                fprintf(stderr, "%02x", key[i]);
            fprintf(stderr, "\n");
        }
        
        // Escrever no JSON
        int fd = open(g_json_path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
        if (fd >= 0) {
            char buf[1024];
            g_event_n++;
            snprintf(buf, sizeof(buf), 
                "{\"n\": %d, \"type\": \"init\", \"key\": \"", g_event_n);
            if (key) {
                for (int i = 0; i < 32; i++)
                    snprintf(buf + strlen(buf), sizeof(buf) - strlen(buf), "%02x", key[i]);
            }
            strcat(buf, "\", \"iv\": \"");
            if (iv) {
                for (int i = 0; i < 16; i++)
                    snprintf(buf + strlen(buf), sizeof(buf) - strlen(buf), "%02x", iv[i]);
            }
            strcat(buf, "\"}\n");
            write(fd, buf, strlen(buf));
            close(fd);
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
