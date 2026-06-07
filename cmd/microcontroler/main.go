package main

import (
	"fishSim/internal/topics"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	inboudTopic *string
)

type Command struct {
	id        int
	algorithm topics.Algorithm
	duration  time.Duration
}

type MsgType int

const (
	Start MsgType = iota
	Continue
	Stop
)

const TOTAL_PARTS = 3

func parseCmd(raw []byte) (Command, error) {
	parts := strings.Split(string(raw), ";")
	if len(parts) != TOTAL_PARTS {
		return Command{}, fmt.Errorf(
			"Message has incompatible number of components. Expected: %d, actual: %d",
			TOTAL_PARTS, len(parts),
		)
	}

	var parsedParts [TOTAL_PARTS]int
	for i, rawdata := range parts {
		data, err := strconv.Atoi(rawdata)
		if err != nil {
			return Command{}, err
		}

		parsedParts[i] = data
	}

	return Command{
		parsedParts[0],
		topics.Algorithm(parsedParts[1]),
		time.Duration(parsedParts[2]),
	}, nil
}

func newMessage(cmdId int, msgType MsgType, data int) string {
	usec := time.Now().UnixNano()
	message := fmt.Sprintf("%d;%d;%d;%d", cmdId, msgType, data, usec)
	return message
}

func publish(c mqtt.Client, algorithm topics.Algorithm, data string) {
	var encrypted string
	switch algorithm {
	case topics.PlainText:
		encrypted = data
	case topics.AES:

	}

	topicInfo, ok := topics.OutboundTopics[algorithm]
	if !ok {
		log.Printf("Could not find topic for algorithm %d\n", algorithm)
		return
	}

	c.Publish(topicInfo.Topic, 0, false, encrypted)
}

func onMessageReceived(c mqtt.Client, message mqtt.Message) {
	data := message.Payload()
	cmd, err := parseCmd(data)
	if err != nil {
		fmt.Printf("Could not parse message '%s': %s", data, err)
		return
	}

	publish(c, cmd.algorithm, newMessage(cmd.id, Start, 0))
	timeout := time.After(cmd.duration * time.Second)
finish:
	for {
		select {
		case <-timeout:
			break finish
		default:
			data := rand.IntN(100)
			publish(c, cmd.algorithm, newMessage(cmd.id, Continue, data))
		}
	}

	publish(c, cmd.algorithm, newMessage(cmd.id, Stop, 0))
}

func main() {
	mqtt.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
	mqtt.CRITICAL = log.New(os.Stdout, "[CRIT] ", 0)
	mqtt.WARN = log.New(os.Stdout, "[WARN]  ", 0)

	server := flag.String(
		"mqtt_server",
		"tcp://127.0.0.1:1883", "The full url of the MQTT server to connect")
	username := flag.String("mqtt_user", "", "A username to authenticate to the MQTT server")
	password := flag.String("mqtt_pass", "", "Password to match the MQTT username")
	inboudTopic = flag.String("mqtt_inbound_topic", "",
		"Inbound topic (where the commands are received)")

	mqttOutboundConfigFile := flag.String("mqtt_outbound_config", "", "JSON file with information abount outbound topics")

	flag.Parse()

	topics.ParseConfigFile(*mqttOutboundConfigFile)

	hostname, _ := os.Hostname()
	clientid := "mock-microcontroler-" + hostname + strconv.Itoa(time.Now().Second())
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
		if token := c.Subscribe(*inboudTopic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
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
