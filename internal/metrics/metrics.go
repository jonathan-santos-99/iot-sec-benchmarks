package metrics

import (
	"cmp"
	"fishSim/internal/ecryption"
	"fishSim/internal/topics"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Service struct {
	client mqtt.Client
}

type Metric struct {
	id        int
	algorithm ecryption.Algorithm
	start     time.Time
	end       *time.Time
	mu        sync.Mutex
	reqs      []struct {
		data      int
		timestamp int64
	}
}

var db = struct {
	data map[int]*Metric
	mu   sync.Mutex
}{
	data: make(map[int]*Metric),
}

type MetricsOutput struct {
	Id   int    `json:"id"`
	Reqs []int  `json:"requests_per_second"`
	Type string `json:"type"`
}

func (s *Service) GetMetrics() []MetricsOutput {
	out := make([]MetricsOutput, 0)
	for _, metric := range db.data {
		if len(metric.reqs) <= 0 {
			continue
		}

		var reqsPerSec []int
		reqsCount := 1
		lastReqTimestamp := metric.reqs[0].timestamp
		var accDuration int64 = 0
		for _, req := range metric.reqs[1:] {
			accDuration += (req.timestamp - lastReqTimestamp)
			if time.Duration(accDuration) >= 1*time.Second {
				reqsPerSec = append(reqsPerSec, reqsCount)
				accDuration = 0
				reqsCount = 1
			} else {
				reqsCount += 1
			}

			lastReqTimestamp = req.timestamp
		}

		reqsPerSec = append(reqsPerSec, reqsCount)

		out = append(out, MetricsOutput{
			Id:   metric.id,
			Type: metric.algorithm.String(),
			Reqs: reqsPerSec,
		})
	}

	return out
}

func NewService(server, username, password, inboundTopic, mqttOutboundConfigFile string) *Service {
	hostname, _ := os.Hostname()
	clientid := "server-" + hostname + strconv.Itoa(time.Now().Second())
	opts := mqtt.
		NewClientOptions().
		AddBroker(server).
		SetClientID(clientid).
		SetCleanSession(true).
		SetUsername(username).
		SetPassword(password).
		SetKeepAlive(2 * time.Second).
		SetPingTimeout(1 * time.Second).
		SetConnectionNotificationHandler(func(client mqtt.Client, n mqtt.ConnectionNotification) {
			switch ntype := n.(type) {
			case mqtt.ConnectionNotificationConnected:
				fmt.Printf("[NOTIFICATION] connected\n")
			case mqtt.ConnectionNotificationConnecting:
				fmt.Printf("[NOTIFICATION] connecting (isReconnect=%t) [%d]\n",
					ntype.IsReconnect, ntype.Attempt)
			case mqtt.ConnectionNotificationFailed:
				fmt.Printf("[NOTIFICATION] connection failed: %v\n", ntype.Reason)
			case mqtt.ConnectionNotificationLost:
				fmt.Printf("[NOTIFICATION] connection lost: %v\n", ntype.Reason)
			case mqtt.ConnectionNotificationBroker:
				fmt.Printf("[NOTIFICATION] broker connection: %s\n", ntype.Broker.String())
			case mqtt.ConnectionNotificationBrokerFailed:
				fmt.Printf("[NOTIFICATION] broker connection failed: %v [%s]\n",
					ntype.Reason, ntype.Broker.String())
			}
		})

	topics.ParseConfigFile(mqttOutboundConfigFile)

	opts.OnConnect = func(c mqtt.Client) {
		for _, topicInfo := range topics.OutboundTopics {
			token := c.Subscribe(topicInfo.Topic, 0, handleMessage)
			if token.Wait() && token.Error() != nil {
				panic(token.Error())
			}
		}
	}

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return &Service{client: c}
}

type MsgType int

const (
	Start MsgType = iota
	Continue
	Stop
)

type outboundMessage struct {
	cmdId     int
	msgType   MsgType
	data      int
	timestamp int64
}

func handleMessage(c mqtt.Client, message mqtt.Message) {
	algorithm, ok := topics.FindAlgorithm(message.Topic())
	if !ok {
		log.Printf("Could not find config for topic '%s'", message.Topic())
		return
	}

	raw := message.Payload()
	parsedMessage, err := parseMessage(algorithm, raw)
	if err != nil {
		log.Printf("Could not parse message '%s': %s", raw, err)
		return
	}

	record, ok := db.data[parsedMessage.cmdId]
	if !ok {
		db.mu.Lock()
		record = new(Metric)
		record.algorithm = ecryption.PlainText
		record.id = parsedMessage.cmdId
		db.data[parsedMessage.cmdId] = record
		db.mu.Unlock()
	}

	record.mu.Lock()
	defer record.mu.Unlock()

	switch parsedMessage.msgType {
	case Start:
		log.Printf("Start gathering data of benchmark %d\n", parsedMessage.cmdId)
		record.start = time.Unix(0, parsedMessage.timestamp)
	case Continue:
		record.reqs = append(record.reqs, struct {
			data      int
			timestamp int64
		}{
			parsedMessage.data,
			parsedMessage.timestamp,
		})
	case Stop:
		log.Printf("Finish gathering data of benchmark %d\n", parsedMessage.cmdId)
		record.end = new(time.Unix(0, parsedMessage.timestamp))
		slices.SortFunc(record.reqs, func(a, b struct {
			data      int
			timestamp int64
		}) int {
			return cmp.Compare(a.timestamp, b.timestamp)
		})
	}
}

func parseMessage(algorithm ecryption.Algorithm, raw []byte) (outboundMessage, error) {
	decrypted, err := ecryption.Decrypt(algorithm, raw)
	if err != nil {
		return outboundMessage{}, err
	}

	log.Printf("Received: %s\n", decrypted)

	const TOTAL_PARTS = 4
	parts := strings.SplitN(string(decrypted), ";", TOTAL_PARTS)
	if len(parts) < TOTAL_PARTS {
		return outboundMessage{}, fmt.Errorf(
			"Message has incompatible number of components. Expected: %d, actual: %d",
			TOTAL_PARTS, len(parts),
		)
	}

	var message outboundMessage
	cmdId, err := strconv.Atoi(parts[0])
	if err != nil {
		return outboundMessage{}, err
	}

	msgType, err := strconv.Atoi(parts[1])
	if err != nil {
		return outboundMessage{}, err
	}

	data, err := strconv.Atoi(parts[2])
	if err != nil {
		return outboundMessage{}, err
	}

	timestamp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return outboundMessage{}, err
	}

	message.cmdId = cmdId
	message.msgType = MsgType(msgType)
	message.data = data
	message.timestamp = timestamp
	return message, nil
}
