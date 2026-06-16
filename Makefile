include .env

MQTT_SERVER          := 127.0.0.1
MQTT_INBOUND_TOPIC   := inbound
MQTT_OUTBOUND_CONFIG := ./outbound.json

MQTT_CA_FILE           := ./certs/rootCA-crt.pem
MQTT_SENSOR_CRT_FILE   := ./certs/sensor-001-crt.pem
MQTT_SENSOR_KEY_FILE   := ./certs/sensor-001-key.pem
MQTT_BANCKEND_CRT_FILE := ./certs/backend-crt.pem
MQTT_BANCKEND_KEY_FILE := ./certs/backend-key.pem

BENCHMARK_DURATION_SECS := 10

build:
	go build -o bin/fishSim ./cmd/api

run: build
	@echo "Running API..."
	@./bin/fishSim -pwfile data/pwfile 	 					       \
				   -mqtt_server          $(MQTT_SERVER)            \
				   -mqtt_user            $(MQTT_USER)              \
				   -mqtt_pass            $(MQTT_PASS)              \
				   -mqtt_outbound_config $(MQTT_OUTBOUND_CONFIG)   \
				   -mqtt_ca_file         $(MQTT_CA_FILE)           \
                   -mqtt_crt_file        $(MQTT_BANCKEND_CRT_FILE) \
                   -mqtt_key_file        $(MQTT_BANCKEND_KEY_FILE)



add_user: data/add_user.py
	data/add_user.py data/pwfile

mock_mic:
	@echo "Setting up fake microcontroler"
	@go run ./cmd/microcontroler -mqtt_server          $(MQTT_SERVER)          \
								 -mqtt_user            $(MQTT_USER)            \
	 							 -mqtt_pass            $(MQTT_PASS)            \
								 -mqtt_inbound_topic   $(MQTT_INBOUND_TOPIC)   \
                				 -mqtt_outbound_config $(MQTT_OUTBOUND_CONFIG) \
								 -mqtt_ca_file         $(MQTT_CA_FILE)         \
                                 -mqtt_crt_file        $(MQTT_SENSOR_CRT_FILE) \
                                 -mqtt_key_file        $(MQTT_SENSOR_KEY_FILE)

setup_moquistto:
	@echo "setting up mosquitto container"
	docker compose up
	CONTAINER_NAME=mqtt5
	@source ./mosquitto/setup_moquistto.sh $(CONTAINER_NAME) \
	 								       $(MQTT_USER)      \
	 									   $(MQTT_PASS)

benchmark: ./scripts/benchmark_starter.py
	@python3 ./scripts/benchmark_starter.py $(MQTT_INBOUND_TOPIC)      \
											$(BENCHMARK_DURATION_SECS) \
											$(MQTT_USER)               \
											$(MQTT_PASS)               \
											$(MQTT_CA_FILE)

generate_topics_c_file: $(MQTT_OUTBOUND_CONFIG)
	@python3 ./scripts/generate_topics_c_file.py $(MQTT_OUTBOUND_CONFIG)