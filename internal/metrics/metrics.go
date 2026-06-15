package metrics

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fishSim/internal/ecryption"
	"fishSim/internal/topics"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Metric struct {
	id        int
	algorithm ecryption.Algorithm
	start     time.Time
	end       *time.Time
	mu        sync.Mutex
	checksum  bool
	tls       bool
	reqs      []int64
}

var db = struct {
	data map[int]*Metric
	mu   sync.Mutex
}{
	data: make(map[int]*Metric),
}

type MetricsOutput struct {
	Id       int    `json:"id"`
	Reqs     []int  `json:"requests_per_second"`
	Type     string `json:"type"`
	Checksum bool   `json:"checksum"`
	TLS      bool   `json:"tls"`
}

func GetMetrics() []MetricsOutput {
	out := make([]MetricsOutput, 0)
	for _, metric := range db.data {
		if len(metric.reqs) <= 0 {
			continue
		}

		var reqsPerSec []int
		reqsCount := 1
		lastReqTimestamp := metric.reqs[0]
		var accDuration int64 = 0
		for _, timestamp := range metric.reqs[1:] {
			accDuration += (timestamp - lastReqTimestamp)
			if time.Duration(accDuration) >= 1*time.Second {
				reqsPerSec = append(reqsPerSec, reqsCount)
				accDuration = 0
				reqsCount = 1
			} else {
				reqsCount += 1
			}

			lastReqTimestamp = timestamp
		}

		reqsPerSec = append(reqsPerSec, reqsCount)

		out = append(out, MetricsOutput{
			Id:       metric.id,
			Type:     metric.algorithm.String(),
			Reqs:     reqsPerSec,
			Checksum: metric.checksum,
			TLS:      metric.tls,
		})
	}

	return out
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
	timestamp int64
	checksum  bool
}

func HandleMessage(c mqtt.Client, message mqtt.Message) {
	topic, tls := strings.CutPrefix(message.Topic(), "tls/")

	algorithm, ok := topics.FindAlgorithm(topic)
	if !ok {
		log.Printf("Could not find config for topic '%s'", message.Topic())
		return
	}

	raw := message.Payload()
	parsedMessage, err := parseMessage(algorithm, raw)
	if err != nil {
		log.Printf("Could not parse message: %s", err)
		return
	}

	switch parsedMessage.msgType {
	case Start:
		db.mu.Lock()
		defer db.mu.Unlock()

		record := new(Metric)
		record.algorithm = algorithm
		record.id = parsedMessage.cmdId
		record.checksum = parsedMessage.checksum
		record.start = time.Unix(0, parsedMessage.timestamp)
		record.tls = tls
		db.data[parsedMessage.cmdId] = record
		log.Printf("Start gathering data of benchmark %d\n", parsedMessage.cmdId)

	case Continue:
		record := db.data[parsedMessage.cmdId]
		record.mu.Lock()
		defer record.mu.Unlock()
		if record.end != nil {
			log.Printf("Ignoring data because end benchmark already arrived")
			return
		}

		record.reqs = append(record.reqs, parsedMessage.timestamp)
	case Stop:
		record := db.data[parsedMessage.cmdId]
		record.mu.Lock()
		defer record.mu.Unlock()

		log.Printf("Finish gathering data of benchmark %d\n", parsedMessage.cmdId)
		record.end = new(time.Unix(0, parsedMessage.timestamp))
		slices.Sort(record.reqs)
	}
}

func parseMessage(algorithm ecryption.Algorithm, raw []byte) (outboundMessage, error) {
	topicInfo, ok := topics.OutboundTopics[algorithm]
	if !ok {
		return outboundMessage{},
			fmt.Errorf("Could not found info about algorithm %s", algorithm.String())
	}

	decrypted, err := ecryption.Decrypt(algorithm, []byte(topicInfo.Key), raw)
	if err != nil {
		return outboundMessage{}, err
	}

	// log.Printf("Received: %s\n", decrypted)

	const TOTAL_PARTS = 5
	parts := strings.Split(string(decrypted), ";")
	if len(parts) > TOTAL_PARTS {
		return parseMessageWithDigest(parts)
	}

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

	timestamp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return outboundMessage{}, err
	}

	checksum, err := strconv.Atoi(parts[4])
	if err != nil {
		return outboundMessage{}, err
	}

	message.cmdId = cmdId
	message.msgType = MsgType(msgType)
	message.timestamp = timestamp
	message.checksum = checksum > 0
	return message, nil
}

func parseMessageWithDigest(parts []string) (outboundMessage, error) {
	const TOTAL_PARTS = 6
	if len(parts) != TOTAL_PARTS {
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

	msgType, err := strconv.Atoi(parts[2])
	if err != nil {
		return outboundMessage{}, err
	}

	// data, err :=
	// if err != nil {
	// 	return outboundMessage{}, err
	// }

	timestamp, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return outboundMessage{}, err
	}

	checksum, err := strconv.Atoi(parts[5])
	if err != nil {
		return outboundMessage{}, err
	}

	sum, err := hex.DecodeString(parts[1])
	if err != nil {
		return outboundMessage{}, err
	}

	if len(sum) != 32 {
		return outboundMessage{}, fmt.Errorf("Invalid hash length %d: %x", len(sum), sum)
	}

	payload := fmt.Sprintf("%d;%s;%d;%d", msgType, parts[3], timestamp, checksum)
	digest := sha256.Sum256([]byte(payload))
	if !bytes.Equal(sum, digest[:]) {
		return outboundMessage{}, fmt.Errorf("Hashes dont match!")
	}

	message.cmdId = cmdId
	message.msgType = MsgType(msgType)
	message.timestamp = timestamp
	message.checksum = checksum > 0
	return message, nil
}
