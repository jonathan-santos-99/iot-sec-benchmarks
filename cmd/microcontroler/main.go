package main

import (
	"crypto/sha256"
	"fishSim/internal/ecryption"
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
	inboudTopic  *string
	commandQueue = make(chan Command, 1024)
	c            mqtt.Client
)

type Command struct {
	id        int
	algorithm ecryption.Algorithm
	duration  time.Duration
	checksum  int
}

type MsgType int

const (
	Start MsgType = iota
	Continue
	Stop
)

const TOTAL_PARTS = 4

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
		ecryption.Algorithm(parsedParts[1]),
		time.Duration(parsedParts[2]),
		parsedParts[3],
	}, nil
}

func newMessage(cmdId int, msgType MsgType, data, checksum int) string {
	usec := time.Now().UnixNano()
	payload := fmt.Sprintf("%d;%d;%d;%d", msgType, data, usec, checksum)
	if checksum > 0 {
		digest := sha256.Sum256([]byte(payload))
		return fmt.Sprintf("%d;%x;%s", cmdId, digest, payload)
	}

	return fmt.Sprintf("%d;%s", cmdId, payload)
}

func publish(c mqtt.Client, algorithm ecryption.Algorithm, data string) {
	topicInfo, ok := topics.OutboundTopics[algorithm]
	if !ok {
		log.Printf("Could not find topic for algorithm %d\n", algorithm)
		return
	}

	encrypted, err := ecryption.Encrypt(algorithm, []byte(topicInfo.Key), []byte(data))
	if err != nil {
		log.Printf("Could not encrypt data for algorithm %s: %s\n", algorithm.String(), err)
		return
	}

	c.Publish(topicInfo.Topic, 0, false, encrypted)
}

func onMessageReceived(c mqtt.Client, message mqtt.Message) {
	data := message.Payload()
	cmd, err := parseCmd(data)
	if err != nil {
		log.Printf("Could not parse message '%s': %s\n", data, err)
		return
	}

	// Non-blocking send to queue
	select {
	case commandQueue <- cmd:
		log.Printf("Command queued: %d\n", cmd.id)
	default:
		log.Printf("Command queue full, dropping command: %d\n", cmd.id)
	}
}

func processCommands() {
	for cmd := range commandQueue {
		publish(c, cmd.algorithm, newMessage(cmd.id, Start, 0, cmd.checksum))
		timeout := time.After(cmd.duration * time.Second)
	finish:
		for {
			select {
			case <-timeout:
				break finish
			default:
				data := rand.IntN(100)
				msg := newMessage(cmd.id, Continue, data, cmd.checksum)
				publish(c, cmd.algorithm, msg)
			}
		}

		publish(c, cmd.algorithm, newMessage(cmd.id, Stop, 0, cmd.checksum))
		log.Printf("Finished command: %d\n", cmd.id)
	}
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
	opts := mqtt.
		NewClientOptions().
		AddBroker(*server).
		SetClientID(clientid).
		SetCleanSession(true).
		SetKeepAlive(2 * time.Second).
		SetPingTimeout(1 * time.Second)

	if *username != "" {
		opts.SetUsername(*username)
		if *password != "" {
			opts.SetPassword(*password)
		}
	}

	opts.SetConnectionNotificationHandler(func(client mqtt.Client, notification mqtt.ConnectionNotification) {
		switch n := notification.(type) {
		case mqtt.ConnectionNotificationConnected:
			log.Printf("[NOTIFICATION] connected\n")
		case mqtt.ConnectionNotificationConnecting:
			log.Printf("[NOTIFICATION] connecting (isReconnect=%t) [%d]\n", n.IsReconnect, n.Attempt)
		case mqtt.ConnectionNotificationFailed:
			log.Printf("[NOTIFICATION] connection failed: %v\n", n.Reason)
		case mqtt.ConnectionNotificationLost:
			log.Printf("[NOTIFICATION] connection lost: %v\n", n.Reason)
		case mqtt.ConnectionNotificationBroker:
			log.Printf("[NOTIFICATION] broker connection: %s\n", n.Broker.String())
		case mqtt.ConnectionNotificationBrokerFailed:
			log.Printf("[NOTIFICATION] broker connection failed: %v [%s]\n", n.Reason, n.Broker.String())
		}
	})

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(*inboudTopic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	c = mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	processCommands()
}
