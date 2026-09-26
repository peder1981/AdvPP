// Enhanced RPO key hook v4 - captura key passada como parâmetro
// Se key tem 16 bytes mas cipher é AES-256, tentar expandir
// Compilar: g++ -O2 -shared -fPIC rpo_key_hook_v4.cpp -o rpo_key_hook_v4.so -ldl

#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <pthread.h>
#include <stdint.h>
#include <fcntl.h>

#define REAL_LIB "libcrypto.so.3"

typedef struct {
    int n;
    char type[32];
    char cipher[128];
    char key[128];
    char iv[64];
    char plaintext[512];
} Event;

static Event* g_events = nullptr;
static int g_event_count = 0;
static int g_buffer_size = 4096;
static pthread_mutex_t g_mutex = PTHREAD_MUTEX_INITIALIZER;
static char g_json_path[512] = "/tmp/rpo_keys.json";
static int g_next_n = 1;
static char g_last_cipher[128] = "unknown";
static unsigned char g_last_key[64] = {};
static int g_last_key_len = 0;
static unsigned char g_last_iv[32] = {};
static int g_last_iv_len = 0;

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

static void* g_libcrypto = nullptr;

static void hex_encode(const unsigned char* data, int len, char* out, int out_size) {
    int max_hex = (out_size - 1) / 2;
    int n = len < max_hex ? len : max_hex;
    out[0] = '\0';
    for (int i = 0; i < n; i++) {
        snprintf(out + strlen(out), out_size - strlen(out), "%02x", data[i]);
    }
}

static void write_json() {
    int fd = open(g_json_path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
    if (fd < 0) return;
    
    char buf[65536];
    int pos = 0;
    pos += snprintf(buf + pos, sizeof(buf) - pos, "[\n");
    
    for (int i = 0; i < g_event_count; i++) {
        Event* e = &g_events[i];
        pos += snprintf(buf + pos, sizeof(buf) - pos, "  {\"n\": %d, \"type\": \"%s\"", e->n, e->type);
        if (e->cipher[0]) pos += snprintf(buf + pos, sizeof(buf) - pos, ", \"cipher\": \"%s\"", e->cipher);
        if (e->key[0]) pos += snprintf(buf + pos, sizeof(buf) - pos, ", \"key\": \"%s\"", e->key);
        if (e->iv[0]) pos += snprintf(buf + pos, sizeof(buf) - pos, ", \"iv\": \"%s\"", e->iv);
        if (e->plaintext[0]) pos += snprintf(buf + pos, sizeof(buf) - pos, ", \"plaintext\": \"%s\"", e->plaintext);
        pos += snprintf(buf + pos, sizeof(buf) - pos, "%c\n", (i < g_event_count - 1) ? ',' : ' ');
    }
    pos += snprintf(buf + pos, sizeof(buf) - pos, "]\n");
    
    write(fd, buf, pos);
    close(fd);
}

__attribute__((constructor))
static void init_hook() {
    const char* env_path = getenv("RPO_KEYS_EXPORT");
    if (env_path && strlen(env_path) > 0) {
        strncpy(g_json_path, env_path, sizeof(g_json_path) - 1);
    }
    
    g_events = (Event*)malloc(g_buffer_size * sizeof(Event));
    
    g_libcrypto = dlopen(REAL_LIB, RTLD_LAZY);
    if (g_libcrypto) {
        g_orig_evpinit = (evp_init_ex_t)dlsym(g_libcrypto, "EVP_EncryptInit_ex");
        g_orig_evputUpdate = (evp_update_t)dlsym(g_libcrypto, "EVP_EncryptUpdate");
        g_orig_ctx_get_cipher = (ctx_get_cipher_t)dlsym(g_libcrypto, "EVP_CIPHER_CTX_get0_cipher");
        g_orig_cipher_get_name = (cipher_get_name_t)dlsym(g_libcrypto, "EVP_CIPHER_get0_name");
        g_orig_ctx_get_keylen = (ctx_get_keylen_t)dlsym(g_libcrypto, "EVP_CIPHER_CTX_get_key_length");
        g_orig_ctx_get_ivlen = (ctx_get_ivlen_t)dlsym(g_libcrypto, "EVP_CIPHER_CTX_get_iv_length");
        
        fprintf(stderr, "[rpo_key_hook_v4] Hooks:\n");
        fprintf(stderr, "  EVP_EncryptInit_ex: %p\n", g_orig_evpinit);
        fprintf(stderr, "  EVP_CIPHER_CTX_get_key_length: %p\n", g_orig_ctx_get_keylen);
        fprintf(stderr, "  Output: %s\n", g_json_path);
    }
}

__attribute__((destructor))
static void fini_hook() {
    write_json();
    free(g_events);
}

static void add_event(const char* type, const char* cipher, 
                      const unsigned char* key, int key_len,
                      const unsigned char* iv, int iv_len,
                      const unsigned char* plaintext, int plen) {
    pthread_mutex_lock(&g_mutex);
    
    if (g_event_count >= g_buffer_size) {
        g_buffer_size *= 2;
        g_events = (Event*)realloc(g_events, g_buffer_size * sizeof(Event));
    }
    
    Event* e = &g_events[g_event_count++];
    e->n = g_next_n++;
    strncpy(e->type, type, sizeof(e->type)-1);
    if (cipher) strncpy(e->cipher, cipher, sizeof(e->cipher)-1);
    
    if (key && key_len > 0) {
        hex_encode(key, key_len, e->key, sizeof(e->key));
    }
    
    if (iv && iv_len > 0) {
        hex_encode(iv, iv_len, e->iv, sizeof(e->iv));
    }
    
    if (plaintext && plen > 0) {
        int max_print = plen < 256 ? plen : 256;
        for (int i = 0; i < max_print; i++)
            snprintf(e->plaintext + strlen(e->plaintext), sizeof(e->plaintext) - strlen(e->plaintext), "%02x", plaintext[i]);
    }
    
    write_json();
    pthread_mutex_unlock(&g_mutex);
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
        strncpy(g_last_cipher, cname, sizeof(g_last_cipher)-1);
        
        // Obter key_len do contexto
        int key_len = g_orig_ctx_get_keylen ? g_orig_ctx_get_keylen(ctx) : 0;
        int iv_len = g_orig_ctx_get_ivlen ? g_orig_ctx_get_ivlen(ctx) : 0;
        
        // Se key_len for 0 ou incompatível, usar tamanho baseado no cipher name
        if (key_len == 0 || key_len > 64) {
            if (strstr(cname, "aes-256") || strstr(cname, "aes_256")) {
                key_len = 32;
            } else if (strstr(cname, "aes-192") || strstr(cname, "aes_192")) {
                key_len = 24;
            } else if (strstr(cname, "aes-128") || strstr(cname, "aes_128")) {
                key_len = 16;
            } else {
                key_len = 16; // default
            }
        }
        
        // Capturar key (usar parâmetro ou zero-pad)
        if (key) {
            if (key_len <= 64) {
                memcpy(g_last_key, key, key_len > 16 ? 16 : key_len); // Copiar primeiro bytes
                if (key_len > 16 && strlen((char*)key) == 16) {
                    // Key tem 16 bytes mas precisa de mais — tentar expandir
                    memcpy(g_last_key + 16, key, key_len - 16); // Repetir key
                }
                g_last_key_len = key_len;
            }
        }
        
        if (iv && iv_len > 0 && iv_len <= 32) {
            memcpy(g_last_iv, iv, iv_len);
            g_last_iv_len = iv_len;
        }
        
        fprintf(stderr, "[rpo_key_hook_v4] InitEx: cipher=%s key_len=%d key_ptr=%p\n", 
                cname, key_len, key);
        
        add_event("init", g_last_cipher, g_last_key, g_last_key_len, g_last_iv, g_last_iv_len, nullptr, 0);
        
        return ret;
    }
    return 0;
}

int EVP_EncryptUpdate(void* ctx, unsigned char* out, int* outlen, const unsigned char* in, int inlen) {
    if (g_orig_evputUpdate) {
        int ret = g_orig_evputUpdate(ctx, out, outlen, in, inlen);
        add_event("encrypt", g_last_cipher, g_last_key, g_last_key_len, g_last_iv, g_last_iv_len, in, inlen);
        return ret;
    }
    return 0;
}

} // extern "C"
