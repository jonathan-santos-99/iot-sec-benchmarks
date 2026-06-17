package main

import (
	"flag"
	"log"
	"os"

	"fishSim/internal/auth"
	"fishSim/internal/metrics"
	"fishSim/internal/topics"
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
	dataFile := flag.String("db", "data/db", "Path where metrics will be persisted")
	mqttServer := flag.String("mqtt_server", "tcp://127.0.0.1:1883",
		"The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	mqttUsername := flag.String("mqtt_user", "", "A username to authenticate to the MQTT server")
	mqttPassword := flag.String("mqtt_pass", "", "Password to match the MQTT username")
	mqttOutboundConfigFile := flag.String("mqtt_outbound_config", "",
		"JSON file with information abount outbound topics")

	cafile := flag.String("mqtt_ca_file", "", "TLS: Path to CA CRT file")
	clientCrt := flag.String("mqtt_crt_file", "", "TLS: Path to CRT file")
	clientKey := flag.String("mqtt_key_file", "", "TLS: Path to KEY file")

	flag.Parse()

	topics.ParseConfigFile(*mqttOutboundConfigFile)

	authService := auth.NewService(*pwfile)
	metriecsService := metrics.NewService(*dataFile)
	renderer, err := views.NewRenderer()
	if err != nil {
		log.Fatal(err)
	}

	app := application{
		authService:    authService,
		metricsService: metriecsService,
		renderer:       renderer,
		sessions:       make(map[string]string),
	}

	c := topics.StartClient(
		*mqttServer,
		*mqttUsername,
		*mqttPassword,
		topics.ClientId("server-"),
	)

	for _, topicInfo := range topics.OutboundTopics {
		token := c.Subscribe(topicInfo.Topic, 0, func(_c mqtt.Client, m mqtt.Message) {
			app.metricsService.HandleMessage(_c, m)
		})

		if token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	tlsC := topics.StartTLSClient(
		*cafile,
		*clientCrt,
		*clientKey,
		*mqttServer,
		*mqttUsername,
		*mqttPassword,
		topics.ClientId("server-tls-"),
	)
	for _, topicInfo := range topics.OutboundTopics {
		token := tlsC.Subscribe("tls/"+topicInfo.Topic, 0, func(_c mqtt.Client, m mqtt.Message) {
			app.metricsService.HandleMessage(_c, m)
		})

		if token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	app.serve()
}
