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

DURATION_SECS := 10

benchmark: benchmark_starter.py
	@python3 benchmark_starter.py $(MQTT_INBOUND_TOPIC) $(DURATION_SECS) $(MQTT_USER) $(MQTT_PASS)
