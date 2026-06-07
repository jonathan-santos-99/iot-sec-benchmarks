include .env

MQTT_INBOUND_TOPIC=inbound
MQTT_OUTBOUND_CONFIG=./outbound.json

build:
	go build -o bin/fishSim ./cmd/api

run: build
	@echo "Running API..."
	@./bin/fishSim -pwfile data/pwfile 	 					    \
				   -mqtt_server          $(MQTT_SERVER)         \
				   -mqtt_user            $(MQTT_USER)           \
				   -mqtt_pass            $(MQTT_PASS)           \
				   -mqtt_outbound_config $(MQTT_OUTBOUND_CONFIG)


add_user: data/add_user.py
	data/add_user.py data/pwfile

mock_mic:
	@echo "Setting up fake microcontroler"
	@go run ./cmd/microcontroler -mqtt_server          $(MQTT_SERVER)         \
								 -mqtt_user            $(MQTT_USER)           \
	 							 -mqtt_pass            $(MQTT_PASS)           \
								 -mqtt_inbound_topic   $(MQTT_INBOUND_TOPIC)  \
                				 -mqtt_outbound_config $(MQTT_OUTBOUND_CONFIG)


setup_moquistto:
	@echo "setting up mosquitto container"
	docker compose up
	CONTAINER_NAME=mqtt5
	@source ./mosquitto/setup_moquistto.sh $(CONTAINER_NAME) \
	 								       $(MQTT_USER)      \
	 									   $(MQTT_PASS)

DURATION_SECS := 5

benchmark: benchmark_starter.py
	@python3 benchmark_starter.py $(MQTT_INBOUND_TOPIC) $(DURATION_SECS) $(MQTT_USER) $(MQTT_PASS)

# benchmark_plaintext:
# 	@mosquitto_pub -t $(MQTT_INBOUND_TOPIC)    \
# 				   -m '1;0;$(DURATION_SECS);0' \
# 				   -u $(MQTT_USER)             \
# 				   -P $(MQTT_PASS)

# 	@echo "Sent benchmark for plain text"

# benchmark_aes:
# 	@mosquitto_pub -t $(MQTT_INBOUND_TOPIC)    \
# 				   -m '2;1;$(DURATION_SECS);0' \
# 				   -u $(MQTT_USER)             \
# 				   -P $(MQTT_PASS)

# 	@echo "Sent benchmark for AES"

# benchmark_chacha20:
# 	@mosquitto_pub -t $(MQTT_INBOUND_TOPIC)    \
# 				   -m '3;2;$(DURATION_SECS);0' \
# 				   -u $(MQTT_USER)             \
# 				   -P $(MQTT_PASS)

# 	@echo "Sent benchmark for ChaCha20"

# benchmark_plaintext_checksum:
# 	@mosquitto_pub -t $(MQTT_INBOUND_TOPIC)    \
# 				   -m '4;0;$(DURATION_SECS);1' \
# 				   -u $(MQTT_USER)             \
# 				   -P $(MQTT_PASS)

# 	@echo "Sent benchmark for plain text with checksum"


# benchmark_aes_checksum:
# 	@mosquitto_pub -t $(MQTT_INBOUND_TOPIC)    \
# 				   -m '5;1;$(DURATION_SECS);1' \
# 				   -u $(MQTT_USER)             \
# 				   -P $(MQTT_PASS)

# 	@echo "Sent benchmark for AES with checksum"

# benchmark_chacha20_checksum:
# 	@mosquitto_pub -t $(MQTT_INBOUND_TOPIC)    \
# 				   -m '6;2;$(DURATION_SECS);1' \
# 				   -u $(MQTT_USER)             \
# 				   -P $(MQTT_PASS)

# 	@echo "Sent benchmark for ChaCha20 with checksum"