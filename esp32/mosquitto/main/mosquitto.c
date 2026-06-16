#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <stdlib.h>
#include <stdbool.h>
#include <inttypes.h>
#include <assert.h>
#include <time.h>
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/queue.h"
#include "driver/gpio.h"
#include "esp_system.h"
#include "nvs_flash.h"
#include "esp_event.h"
#include "esp_sntp.h"
#include "esp_netif.h"
#include "mbedtls/md.h"

#include "encrypt.c"
#include "wifi.c"
#include "topics.h"

#include "esp_log.h"
#include "mqtt_client.h"

static const char *MQTT_TAG = "mqtt";
static const char *hex = "0123456789ABCDEF";
static mbedtls_md_context_t sha256_ctx;

#define COMMAND_QUEUE_MAX_LEN 128
#define MOCK_DATA_SIZE 256 + 1
#define MSG_MAX_SIZE 2*256
#define LED_DEBUG_PIN 4

extern const uint8_t ca_crt_start[] asm("_binary_ca_crt_start");
extern const uint8_t ca_crt_end[]   asm("_binary_ca_crt_end");

extern const uint8_t client_crt_start[] asm("_binary_client_crt_start");
extern const uint8_t client_crt_end[]   asm("_binary_client_crt_end");

extern const uint8_t client_key_start[] asm("_binary_client_key_start");
extern const uint8_t client_key_end[]   asm("_binary_client_key_end");


typedef enum {
    START = 0,
    CONTINUE,
    STOP
} Msg_Type;

typedef struct {
    char *content;
    int len;
} String_View;

typedef struct {
    esp_mqtt_client_handle_t client;
    int id;
    int algorithm;
    int duration_secs;
    int checksum;
    bool tls;
} Command;

QueueHandle_t cmd_queue;

static const char *event_id_to_string(int32_t event_id) {
    switch (event_id) {
        case MQTT_EVENT_ANY:             return "MQTT_EVENT_ANY";
        case MQTT_EVENT_ERROR:           return "MQTT_EVENT_ERROR";
        case MQTT_EVENT_CONNECTED:       return "MQTT_EVENT_CONNECTED";
        case MQTT_EVENT_DISCONNECTED:    return "MQTT_EVENT_DISCONNECTED";
        case MQTT_EVENT_SUBSCRIBED:      return "MQTT_EVENT_SUBSCRIBED";
        case MQTT_EVENT_UNSUBSCRIBED:    return "MQTT_EVENT_UNSUBSCRIBED";
        case MQTT_EVENT_PUBLISHED:       return "MQTT_EVENT_PUBLISHED";
        case MQTT_EVENT_DATA:            return "MQTT_EVENT_DATA";
        case MQTT_EVENT_BEFORE_CONNECT:  return "MQTT_EVENT_BEFORE_CONNECT";
        case MQTT_EVENT_DELETED:         return "MQTT_EVENT_DELETED";
        case MQTT_USER_EVENT:            return "MQTT_USER_EVENT";
        default:                         return "MQTT_UNKNOW_EVENT";
    }
}

static String_View chop_until(String_View *sv, char delimiter) {
    char *start = sv->content;
    while (sv->len > 0 && *sv->content != delimiter) {
        sv->len--;
        sv->content++;
    }

    String_View result = {
        .content = start,
        .len     = sv->content - start
    };

    if (sv->len > 0 && *sv->content == delimiter) {
        sv->len--;
        sv->content++;
    }

    return result;
}

static void parse_command(Command *cmd, char *data, int data_len) {
    String_View sv = { data, data_len };

    int parts[4] = {0};
    for (int i = 0; i < 4; i++) {
        String_View field = chop_until(&sv, ';');
        if (field.len > 0) {
            char *endptr = field.content + field.len;
            parts[i] = strtol(field.content, &endptr, 10);
        }
    }

    cmd->id            = parts[0];
    cmd->algorithm     = parts[1];
    cmd->duration_secs = parts[2];
    cmd->checksum      = parts[3];
}

static void handle_event_data(esp_mqtt_event_handle_t event, bool tls) {
    Command cmd = {0};
    parse_command(&cmd, event->data, event->data_len);
    cmd.client = event->client;
    cmd.tls = tls;
    xQueueSend(cmd_queue, &cmd, 1000 / portTICK_PERIOD_MS);
}

static char *mock_data() {
    static char data[MOCK_DATA_SIZE];
    for (int i = 0; i < MOCK_DATA_SIZE - 1; i += 2) {
        char b = rand() % 255;
        data[i + 0] = hex[(b>>4)&0x0F];
        data[i + 1] = hex[b&0x0F];
    }

    data[MOCK_DATA_SIZE - 1] = '\0';
    return data;
}

static uint64_t time_unix_ns(void) {
    struct timespec ts;

    if (clock_gettime(CLOCK_MONOTONIC, &ts) == 0) {
        return ((uint64_t)ts.tv_sec*1000000000ULL) + (uint64_t)ts.tv_nsec;
    }

    ESP_LOGE(TAG, "Failed to get clock time");
    return 0;
}

static char *fmt_message(const char *fmt, ...) {
	static char buffer[MSG_MAX_SIZE];
    va_list args;
    va_start(args, fmt);

    vsnprintf(buffer, MSG_MAX_SIZE, fmt, args);
    va_end(args);

    return buffer;
}

static char *sha256(const char *payload) {
    // Initialize the message digest context
    static char hex_string[32*2 + 1];
    mbedtls_md_starts(&sha256_ctx);
    unsigned char result[32];
    mbedtls_md_update(&sha256_ctx, (const unsigned char*)payload, strlen(payload));
    mbedtls_md_finish(&sha256_ctx, result);

    int j = 0;
    for (int i = 0; i < 32; i++, j += 2) {
        hex_string[j + 0] = hex[(result[i]>>4)&0x0F];
        hex_string[j + 1] = hex[(result[i]>>0)&0x0F];
    }

    hex_string[j] = '\0';
    return hex_string;
}

static char *new_message(int id, Msg_Type msgtype, const char *data, uint64_t usec, int checksum) {
    if (checksum > 0) {
        char *payload = fmt_message("%d;%s;%"PRIu64";%d", msgtype, data, usec, checksum);
        char *digest = sha256(payload);
        return fmt_message("%d;%s;%d;%s;%"PRIu64";%d", id, digest, msgtype, data, usec, checksum);
    }

    return fmt_message("%d;%d;%s;%"PRIu64";%d", id, msgtype, data, usec, checksum);
}

static void publish(esp_mqtt_client_handle_t client, Algorithm algorithm, char *data, bool tls) {
    Topic_Info topic = outbond_topics[algorithm];
    const char *topic_name =  tls ? topic.tls_name : topic.name;
    if (algorithm == PLAIN_TEXT) {
        esp_mqtt_client_publish(client, topic_name, data, strlen(data), 0, 0);
    } else {
        size_t encrypted_data_size = 0;
        uint8_t *encrypted_data = encrypt_data(algorithm, topic.key, data, &encrypted_data_size);
        esp_mqtt_client_publish(client, topic_name, (const char *) encrypted_data, encrypted_data_size, 0, 0);
        free(encrypted_data);
    }
}

static void debug_led_on(void) {
    gpio_set_level(LED_DEBUG_PIN, 1);
}

static void debug_led_off(void) {
    gpio_set_level(LED_DEBUG_PIN, 0);
}

static void process_commands() {
    Command cmd;
    for (;;) {
        if (xQueueReceive(cmd_queue, &cmd, portMAX_DELAY)) {
            ESP_LOGI(MQTT_TAG, "Starting command %d", cmd.id);
            debug_led_on();

            const uint64_t duration_ns = cmd.duration_secs*1e9;
            uint64_t timer = 0;
            uint64_t last  = time_unix_ns();
            publish(cmd.client, cmd.algorithm, new_message(cmd.id, START, mock_data(), last, cmd.checksum), cmd.tls);
            while (timer <= duration_ns) {
                uint64_t now = time_unix_ns();
                char *msg = new_message(cmd.id, CONTINUE, mock_data(), now, cmd.checksum);
                publish(cmd.client, cmd.algorithm, msg, cmd.tls);
                timer += now - last;
                last = now;
            }

            ESP_LOGI(MQTT_TAG, "Ending command %d", cmd.id);
            publish(cmd.client, cmd.algorithm, new_message(cmd.id, STOP , mock_data(), last, cmd.checksum), cmd.tls);
            debug_led_off();
        }
    }
}

static void mqtt_event_handler(void *handler_args, esp_event_base_t base, int32_t event_id, void *event_data) {
    ESP_LOGD(TAG, "Event dispatched from event loop base=%s, event_id=%" PRIi32 "", base, event_id);
    esp_mqtt_event_handle_t event = event_data;
    esp_mqtt_client_handle_t client = event->client;
    bool tls = *(bool *)handler_args;

    ESP_LOGI(MQTT_TAG, "%s", event_id_to_string(event->event_id));
    switch ((esp_mqtt_event_id_t)event_id) {
        case MQTT_EVENT_CONNECTED: {
            char *inbound_topic;
            if (tls) {
                inbound_topic = "tls/inbound";
            } else {
                inbound_topic = "inbound";
            }

            int msg_id = esp_mqtt_client_subscribe(client, inbound_topic, 0);
            ESP_LOGI(TAG, "sent subscribe to topic '%s' successful, msg_id=%d", inbound_topic, msg_id);
        } break;

        case MQTT_EVENT_DATA: handle_event_data(event, tls); break;

        default: break;
    }
}

const static bool TLS  = 1;
const static bool NTLS = 0;

static void mqtt_app_start(void) {
    esp_mqtt_client_config_t mqtt_cfg = { .broker.address.uri = CONFIG_BROKER_URL };

    esp_mqtt_client_handle_t client = esp_mqtt_client_init(&mqtt_cfg);
    esp_mqtt_client_register_event(client, ESP_EVENT_ANY_ID, mqtt_event_handler, (void*)&NTLS);
    esp_mqtt_client_start(client);

    esp_mqtt_client_config_t mqtt_tls_cfg = {
        .broker = {
            .address.uri = CONFIG_BROKER_TLS_URL,
            .verification = {
                .certificate = (const char *)ca_crt_start,
                .skip_cert_common_name_check = true,
            },
        },
        .credentials = {
            .client_id = "esp32-tls",
            .authentication = {
                .certificate = (const char *)client_crt_start,
                .key = (const char *)client_key_start,
            }
        }
    };

    esp_mqtt_client_handle_t client_tls = esp_mqtt_client_init(&mqtt_tls_cfg);
    esp_mqtt_client_register_event(client_tls, ESP_EVENT_ANY_ID, mqtt_event_handler, (void*)&TLS);
    esp_mqtt_client_start(client_tls);
}

static esp_err_t setup_nvs(void) {
    esp_err_t ret = nvs_flash_init();

    if (ret == ESP_ERR_NVS_NO_FREE_PAGES || ret == ESP_ERR_NVS_NEW_VERSION_FOUND) {
      ESP_ERROR_CHECK(nvs_flash_erase());
      ret = nvs_flash_init();
    }

    return ret;
}

void app_main(void) {
    ESP_ERROR_CHECK(setup_nvs());
    wifi_connect();
    esp_sntp_init();
    encrypt_init();

    gpio_set_level(LED_DEBUG_PIN, 0);
    gpio_reset_pin(LED_DEBUG_PIN);
    gpio_set_direction(LED_DEBUG_PIN, GPIO_MODE_OUTPUT);

    mbedtls_md_init(&sha256_ctx);
    mbedtls_md_setup(&sha256_ctx, mbedtls_md_info_from_type(MBEDTLS_MD_SHA256), 0);

    mqtt_app_start();
    srand(time(NULL));

    cmd_queue = xQueueCreate(64, sizeof(Command));
    assert(cmd_queue != NULL && "Could not create command queue!");

    process_commands();
}