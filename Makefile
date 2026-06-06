include .env

MQTT_OUTBOUND_TOPIC=plain/outbound
MQTT_INBOUND_TOPIC=plain/inbound

build:
	go build -o bin/fishSim ./cmd/api

run: build
	./bin/fishSim -pwfile data/pwfile

add_user: data/add_user.py
	data/add_user.py data/pwfile

mock_mic:
	@echo "Setting up fake microcontroler"
	@go run ./cmd/microcontroler -mqtt_server         $(MQTT_SERVER)         \
								 -mqtt_user           $(MQTT_USER)           \
	 							 -mqtt_pass           $(MQTT_PASS)           \
								 -mqtt_outbound_topic $(MQTT_OUTBOUND_TOPIC) \
								 -mqtt_inbound_topic  $(MQTT_INBOUND_TOPIC)

setup_moquistto:
	@echo "setting up mosquitto container"
	docker compose up
	CONTAINER_NAME=mqtt5
	@source ./mosquitto/setup_moquistto.sh $(CONTAINER_NAME) \
	 								       $(MQTT_USER)      \
	 									   $(MQTT_PASS)

send_message:
	@mosquitto_pub -t $(MQTT_INBOUND_TOPIC) \
				   -m '1;1;2'               \
				   -u $(MQTT_USER)          \
				   -P $(MQTT_PASS)

	@echo "Message sent to topic: $(MQTT_INBOUND_TOPIC)"


subscribe:
	@echo "Subscribing to topic: $(MQTT_OUTBOUND_TOPIC)..."
	@mosquitto_sub -v -t $(MQTT_OUTBOUND_TOPIC) \
				 	  -u $(MQTT_USER)           \
					  -P $(MQTT_PASS)

