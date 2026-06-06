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
	mqtt_server := flag.String("mqtt_server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	mqtt_username := flag.String("mqtt_user", "", "A username to authenticate to the MQTT server")
	mqtt_password := flag.String("mqtt_pass", "", "Password to match the MQTT username")
	mqtt_ouboundTopic := flag.String("mqtt_outbound_topic", "", "Outbound topic (where metrics will be sended)")
	mqtt_inboudTopic := flag.String("mqtt_inbound_topic", "", "Inbound topic (where the commands are sended)")

	flag.Parse()

	authService := auth.NewService(*pwfile)
	metricsService := metrics.NewService(
		*mqtt_server,
		*mqtt_username,
		*mqtt_password,
		*mqtt_ouboundTopic,
		*mqtt_inboudTopic,
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
