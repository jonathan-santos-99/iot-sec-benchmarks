#include <assert.h>
#include <stdlib.h>
#include <string.h>
#include <sodium/crypto_stream_chacha20.h>
#include "aes/esp_aes.h"
#include "topics.h"

#define BLOCK_SIZE 16

static esp_aes_context aes_ctx;

static void random_byte_array(uint8_t *buffer, int size) {
    for (int i = 0; i < size; i++) {
        buffer[i] = rand() % 255;
    }
}

static void add_pkcs7_padding(uint8_t *input, size_t input_len, size_t padded_len) {
    uint8_t padding_value = padded_len - input_len;
    for (size_t i = input_len; i < padded_len; i++) {
        input[i] = padding_value;
    }
}

void encrypt_init(void) {
    esp_aes_init(&aes_ctx);
    assert(esp_aes_setkey(&aes_ctx, outbond_topics[AES].key, 256) == 0);
}

static uint8_t *encrypt_aes(const char *plaintext, size_t *output_size) {
    uint8_t iv_encrypt[BLOCK_SIZE];
    random_byte_array(iv_encrypt, BLOCK_SIZE);

    size_t plaintext_len = strlen(plaintext);
    size_t padded_len = ((plaintext_len/BLOCK_SIZE) + 1) * BLOCK_SIZE;

    uint8_t input_buffer[padded_len];
    memcpy(input_buffer, plaintext, plaintext_len);
    add_pkcs7_padding(input_buffer, plaintext_len, padded_len);

    uint8_t *output_buffer = malloc(sizeof(uint8_t)*(padded_len+BLOCK_SIZE));
    memcpy(output_buffer, iv_encrypt, BLOCK_SIZE);
    if (output_size) {
        *output_size = padded_len+BLOCK_SIZE;
    }

    assert(
        esp_aes_crypt_cbc(
            &aes_ctx,
            ESP_AES_ENCRYPT,
            padded_len,
            iv_encrypt,
            input_buffer,
            output_buffer + BLOCK_SIZE
        ) == 0
    );

    return output_buffer;
}

static uint8_t *encrypt_chacha20(const uint8_t key[32], const char *plaintext, size_t *output_size) {
    uint8_t nonce[crypto_stream_chacha20_ietf_NONCEBYTES];
    random_byte_array(nonce, crypto_stream_chacha20_ietf_NONCEBYTES);

    size_t input_len = strlen(plaintext);
    size_t _output_size = input_len + crypto_stream_chacha20_ietf_NONCEBYTES;
    uint8_t *output_buffer = malloc(sizeof(uint64_t)*_output_size);
    if (output_size) {
        *output_size = _output_size;
    }

    memcpy(output_buffer, nonce, crypto_stream_chacha20_ietf_NONCEBYTES);

    crypto_stream_chacha20_ietf_xor(
        output_buffer+crypto_stream_chacha20_ietf_NONCEBYTES,
        (const unsigned char *)plaintext,
        input_len,
        nonce, key
    );

    return output_buffer;
}

uint8_t *encrypt_data(Algorithm algorithm, const uint8_t key[32], char *plaintext, size_t *output_size) {
    switch (algorithm) {
        case PLAIN_TEXT: {
            if (output_size) *output_size = strlen(plaintext);
            return (uint8_t *) strdup(plaintext);
        }
        case AES:      return encrypt_aes(plaintext, output_size);
        case CHACHA20: return encrypt_chacha20(key, plaintext, output_size);
    }

    assert(0 && "unreacheable");
    return NULL;
}
