#ifndef _TOPIC_H
#define _TOPIC_H

typedef enum {
    PLAIN_TEXT = 0,
    AES = 1,
    CHACHA20 = 2,
} Algorithm;

typedef struct {
    const char *name;
    const uint8_t key[32] __attribute__((nonstring));
} Topic_Info;

Topic_Info outbond_topics[] = {
    [PLAIN_TEXT] = {
        .name = "outbound/plain",
        .key  = "",
    },
    [AES] = {
        .name = "outbound/aes",
        .key  = "passphrasewhichneedstobe32bytess",
    },
    [CHACHA20] = {
        .name = "outbound/chacha20",
        .key  = "this-is-a-secret-32-byte-key-abc",
    },
};

#endif // _TOPIC_H
