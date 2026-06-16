idf_component_register(
        SRCS "encrypt.c" "wifi.c" "mosquitto.c"
        PRIV_REQUIRES esp_wifi nvs_flash esp_driver_gpio mbedtls
        INCLUDE_DIRS "."
        EMBED_TXTFILES
            "certs/ca.crt"
            "certs/client.crt"
            "certs/client.key")
