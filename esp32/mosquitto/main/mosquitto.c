#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <stdlib.h>
#include <inttypes.h>
#include "esp_system.h"
#include "nvs_flash.h"
#include "esp_event.h"
#include "esp_netif.h"
#include "wifi.c"

#include "esp_log.h"
#include "mqtt_client.h"


static const char *MQTT_TAG = "mqtt_example";

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

typedef struct {
    char *content;
    int len;
} String_View;

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

static void handle_event_data(esp_mqtt_event_handle_t event) {
    String_View sv = {
        .content = event->data,
        .len = event->data_len
    };

    int parts[4] = {0};

    for (int i = 0; i < 4; i++) {
        String_View field = chop_until(&sv, ';');
        if (field.len > 0) {
            char *endptr = field.content + field.len;
            parts[i] = strtol(field.content, &endptr, 10);
        }
    }

    int id = parts[0];
    int algorithm = parts[1];
    int duration = parts[2];
    int checksum = parts[3];
    printf("id = %d, algorithm = %d, duration = %d, checksum = %d", id, algorithm, duration, checksum);
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

static void mqtt_app_start(void)
{
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

void setup(void) {
    ESP_ERROR_CHECK(setup_nvs());
    wifi_connect();
    mqtt_app_start();
}

void app_main(void) {
    setup();
    for (;;) {
        vTaskDelay(500 / portTICK_PERIOD_MS);
    }
}