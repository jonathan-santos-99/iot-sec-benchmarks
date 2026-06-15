#include <assert.h>
#include <stdlib.h>
#include <string.h>
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

static uint8_t *encrypt_chacha20(const uint8_t key[32], const char *plaintext) {
    assert(0 && "TODO: CHACHA20 not implemented");
    return NULL;
}

uint8_t *encrypt_data(Algorithm algorithm, const uint8_t key[32], char *plaintext, size_t *output_size) {
    switch (algorithm) {
        case PLAIN_TEXT: {
            if (output_size) *output_size = strlen(plaintext);
            return (uint8_t *) strdup(plaintext);
        }
        case AES:      return encrypt_aes(plaintext, output_size);
        case CHACHA20: return encrypt_chacha20(key, plaintext);
    }

    assert(0 && "unreacheable");
    return NULL;
}
