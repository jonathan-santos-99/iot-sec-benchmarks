package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func onMessageReceived(_ mqtt.Client, message mqtt.Message) {
	fmt.Printf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())
}

func main2() {
	mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)

	server := flag.String("mqtt_server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	username := flag.String("mqtt_user", "", "A username to authenticate to the MQTT server")
	password := flag.String("mqtt_pass", "", "Password to match the MQTT username")
	flag.Parse()

	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())
	opts := mqtt.NewClientOptions().AddBroker(*server).SetClientID(clientid).SetCleanSession(true)
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	if *username != "" {
		opts.SetUsername(*username)
		if *password != "" {
			opts.SetPassword(*password)
		}
	}

	opts.SetConnectionNotificationHandler(func(client mqtt.Client, notification mqtt.ConnectionNotification) {
		switch n := notification.(type) {
		case mqtt.ConnectionNotificationConnected:
			fmt.Printf("[NOTIFICATION] connected\n")
		case mqtt.ConnectionNotificationConnecting:
			fmt.Printf("[NOTIFICATION] connecting (isReconnect=%t) [%d]\n", n.IsReconnect, n.Attempt)
		case mqtt.ConnectionNotificationFailed:
			fmt.Printf("[NOTIFICATION] connection failed: %v\n", n.Reason)
		case mqtt.ConnectionNotificationLost:
			fmt.Printf("[NOTIFICATION] connection lost: %v\n", n.Reason)
		case mqtt.ConnectionNotificationBroker:
			fmt.Printf("[NOTIFICATION] broker connection: %s\n", n.Broker.String())
		case mqtt.ConnectionNotificationBrokerFailed:
			fmt.Printf("[NOTIFICATION] broker connection failed: %v [%s]\n", n.Reason, n.Broker.String())
		}
	})

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe("hello/topic", 0, onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	for {
	}
}
