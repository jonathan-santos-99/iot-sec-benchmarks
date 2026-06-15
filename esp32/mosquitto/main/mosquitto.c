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
#include "encrypt.c"
#include "esp_system.h"
#include "nvs_flash.h"
#include "esp_event.h"
#include "esp_netif.h"
#include "wifi.c"

#include "esp_log.h"
#include "mqtt_client.h"

static const char *MQTT_TAG = "mqtt";

#define COMMAND_QUEUE_MAX_LEN 128
#define MOCK_DATA_SIZE 1024
#define MSG_MAX_SIZE 2*1024

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

static void handle_event_data(esp_mqtt_event_handle_t event) {
    Command cmd = {0};
    parse_command(&cmd, event->data, event->data_len);
    cmd.client = event->client;
    xQueueSend(cmd_queue, &cmd, 1000 / portTICK_PERIOD_MS);
}

static char *mock_data() {
    static char data[MOCK_DATA_SIZE];
    for (int i = 0; i < MOCK_DATA_SIZE - 1; i++) {
        data[i] = 'O';
    }

    data[MOCK_DATA_SIZE - 1] = '\0';

    return data;
}

static uint64_t time_unix_ns(void) {
    struct timespec ts;

    if (clock_gettime(CLOCK_REALTIME, &ts) == 0) {
        return ((uint64_t)ts.tv_sec*1000000000ULL) + (uint64_t)ts.tv_nsec;
    }

    ESP_LOGE(TAG, "Failed to get clock time");
    return 0;
}

static char *fmt_message(const char *fmt, ...) {
	static char buffer[MSG_MAX_SIZE];
    va_list args;
    va_start(args, fmt);

    vsprintf(buffer, fmt, args);
    va_end(args);

    return buffer;
}

static char *new_message(int id, Msg_Type msgtype, const char *data, uint64_t usec, int checksum) {
    if (checksum > 0) {
        assert(0 && "TODO: checksum not implemented");
	}

    return fmt_message("%d;%d;%s;%"PRIu64";%d", id, msgtype, data, usec, checksum);
}

static void publish(esp_mqtt_client_handle_t client, Algorithm algorithm, char *data) {
    const char *encrypted_data = encrypt_data(algorithm, "", data);
    esp_mqtt_client_publish(client, "outbound/plain", encrypted_data, strlen(encrypted_data), 0, 0);
}

static void process_commands(void *args) {
    Command cmd;
    for (;;) {
        if(xQueueReceive(cmd_queue, &cmd, portMAX_DELAY)) {
            ESP_LOGI(MQTT_TAG, "Starting command %d", cmd.id);
            publish(
                cmd.client,
                cmd.algorithm,
                new_message(cmd.id, START, mock_data(), time_unix_ns(), cmd.checksum)
            );

            const uint64_t duration_ns = cmd.duration_secs*1e9;
            uint64_t timer = 0;
            uint64_t last  = time_unix_ns();
            while (timer <= duration_ns) {
                uint64_t now = time_unix_ns();
                char *msg = new_message(cmd.id, CONTINUE, mock_data(), now, cmd.checksum);
                publish(cmd.client, cmd.algorithm, msg);
                timer += now - last;
                last = now;
            }

            ESP_LOGI(MQTT_TAG, "Ending command %d", cmd.id);
            publish(cmd.client, cmd.algorithm, new_message(cmd.id, STOP , mock_data(), time_unix_ns(), cmd.checksum));
        }
    }
}

static void mqtt_event_handler(void *handler_args, esp_event_base_t base, int32_t event_id, void *event_data) {
    ESP_LOGD(TAG, "Event dispatched from event loop base=%s, event_id=%" PRIi32 "", base, event_id);
    esp_mqtt_event_handle_t event = event_data;
    esp_mqtt_client_handle_t client = event->client;
    int msg_id;
    (void) msg_id;
    (void) client;

    ESP_LOGI(MQTT_TAG, "%s", event_id_to_string(event->event_id));
    switch ((esp_mqtt_event_id_t)event_id) {
        case MQTT_EVENT_CONNECTED: {
            msg_id = esp_mqtt_client_subscribe(client, "inbound", 0);
            ESP_LOGI(TAG, "sent subscribe successful, msg_id=%d", msg_id);
        } break;

        case MQTT_EVENT_DATA: handle_event_data(event); break;

        default: break;
    }
}

static void mqtt_app_start(void) {
    esp_mqtt_client_config_t mqtt_cfg = { .broker.address.uri = CONFIG_BROKER_URL };
    esp_mqtt_client_handle_t client = esp_mqtt_client_init(&mqtt_cfg);
    esp_mqtt_client_register_event(client, ESP_EVENT_ANY_ID, mqtt_event_handler, NULL);
    esp_mqtt_client_start(client);
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
    mqtt_app_start();

    cmd_queue = xQueueCreate(64, sizeof(Command));
    assert(cmd_queue != NULL && "Could not create command queue!");

    xTaskCreate(process_commands, "comamnd_processor", 2048, NULL, 1, NULL);
}