#include <assert.h>

typedef enum {
    PlainText = 0,
    AES,
    ChaCha20
} Algorithm;

char *encrypt_aes(const char *key, const char *plaintext) {
    assert(0 && "TODO: AES not implemented");
    return NULL;
}

char *encrypt_chacha20(const char *key, const char *plaintext) {
    assert(0 && "TODO: ChaCha20 not implemented");
    return NULL;
}

const char *encrypt_data(Algorithm algorithm, const char *key, const char *plaintext) {
    switch (algorithm) {
        case PlainText: return plaintext;
        case AES:       return encrypt_aes(key, plaintext);
        case ChaCha20:  return encrypt_chacha20(key, plaintext);
    }

    return NULL;
}
