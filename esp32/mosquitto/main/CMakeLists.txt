idf_component_register(SRCS "wifi.c" "mosquitto.c"
                    PRIV_REQUIRES esp_wifi nvs_flash
                    INCLUDE_DIRS ".")
