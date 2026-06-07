package main

import (
	"flag"
	"log"
	"os"

	"fishSim/internal/auth"
	"fishSim/internal/metrics"
	"fishSim/views"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type application struct {
	authService    *auth.Service
	metricsService *metrics.Service
	renderer       *views.Renderer
	sessions       map[string]string
}

func main() {
	mqtt.ERROR = log.New(os.Stdout, "[MQTT_ERROR] ", 0)
	mqtt.CRITICAL = log.New(os.Stdout, "[MQTT_CRIT] ", 0)
	mqtt.WARN = log.New(os.Stdout, "[MQTT_WARN]  ", 0)

	pwfile := flag.String("pwfile", "data/pwfile", "The full path for users file")
	mqttServer := flag.String("mqtt_server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	mqttUsername := flag.String("mqtt_user", "", "A username to authenticate to the MQTT server")
	mqttPassword := flag.String("mqtt_pass", "", "Password to match the MQTT username")
	mqttInboundTopic := flag.String("mqtt_inbound_topic", "", "Inbound topic")
	mqttOutboundConfigFile := flag.String("mqtt_outbound_config", "", "JSON file with information abount outbound topics")
	flag.Parse()

	authService := auth.NewService(*pwfile)
	metricsService := metrics.NewService(
		*mqttServer,
		*mqttUsername,
		*mqttPassword,
		*mqttInboundTopic,
		*mqttOutboundConfigFile,
	)

	renderer, err := views.NewRenderer()
	if err != nil {
		log.Fatal(err)
	}

	app := application{
		authService:    authService,
		metricsService: metricsService,
		renderer:       renderer,
		sessions:       make(map[string]string),
	}

	app.serve()
}
