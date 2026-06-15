package main

import (
	"crypto/rand"
	"crypto/sha256"
	"fishSim/internal/ecryption"
	"fishSim/internal/topics"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	inboudTopic  *string
	commandQueue = make(chan struct {
		cmd Command
		c   mqtt.Client
	}, 1024)
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

func newMessage(cmdId int, msgType MsgType, data []byte, checksum int) string {
	usec := time.Now().UnixNano()
	payload := fmt.Sprintf("%d;%x;%d;%d", msgType, data, usec, checksum)
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

	sendTo := topicInfo.Topic
	opts := c.OptionsReader()
	if opts.TLSConfig() != nil {
		sendTo = "tls/" + sendTo
	}

	c.Publish(sendTo, 0, false, encrypted)
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
	case commandQueue <- struct {
		cmd Command
		c   mqtt.Client
	}{cmd, c}:
		log.Printf("Command queued: %d\n", cmd.id)
	default:
		log.Printf("Command queue full, dropping command: %d\n", cmd.id)
	}
}

func mockdata() []byte {
	data := make([]byte, 1024)
	_, err := rand.Read(data)
	if err != nil {
		panic(err)
	}

	return data
}

func processCommands() {
	for q := range commandQueue {
		publish(q.c, q.cmd.algorithm, newMessage(q.cmd.id, Start, mockdata(), q.cmd.checksum))
		timeout := time.After(q.cmd.duration * time.Second)
	finish:
		for {
			select {
			case <-timeout:
				break finish
			default:
				msg := newMessage(q.cmd.id, Continue, mockdata(), q.cmd.checksum)
				publish(q.c, q.cmd.algorithm, msg)
			}
		}

		publish(q.c, q.cmd.algorithm, newMessage(q.cmd.id, Stop, mockdata(), q.cmd.checksum))
		log.Printf("Finished command: %d\n", q.cmd.id)
	}
}

func main() {
	// mqtt.DEBUG = log.New(os.Stdout, "[DEBUG] ", 0)
	mqtt.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
	mqtt.CRITICAL = log.New(os.Stdout, "[CRIT] ", 0)
	mqtt.WARN = log.New(os.Stdout, "[WARN]  ", 0)

	server := flag.String("mqtt_server", "127.0.0.1",
		"The full url of the MQTT server to connect")
	username := flag.String("mqtt_user", "", "A username to authenticate to the MQTT server")
	password := flag.String("mqtt_pass", "", "Password to match the MQTT username")
	cafile := flag.String("mqtt_ca_file", "", "TLS: Path to CA CRT file")
	clientCrt := flag.String("mqtt_crt_file", "", "TLS: Path to CRT file")
	clientKey := flag.String("mqtt_key_file", "", "TLS: Path to KEY file")

	inboudTopic = flag.String("mqtt_inbound_topic", "",
		"Inbound topic (where the commands are received)")
	mqttOutboundConfigFile := flag.String("mqtt_outbound_config", "",
		"JSON file with information abount outbound topics")

	flag.Parse()
	topics.ParseConfigFile(*mqttOutboundConfigFile)

	c := topics.StartClient(
		*server,
		*username,
		*password,
		topics.ClientId("mock-microcontroler-"),
	)

	token := c.Subscribe(*inboudTopic, 0, onMessageReceived)
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	c = topics.StartTLSClient(
		*cafile,
		*clientCrt,
		*clientKey,
		*server,
		*username,
		*password,
		topics.ClientId("mock-microcontroler-tls"),
	)

	token = c.Subscribe("tls/"+*inboudTopic, 0, onMessageReceived)
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	processCommands()
}
