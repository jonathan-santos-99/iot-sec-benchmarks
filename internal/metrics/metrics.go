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
	recvStart time.Time
	end       *time.Time
	recvEnd   *time.Time
	mu        sync.Mutex
	checksum  bool
	tls       bool
	reqs      []int64 // server timestamps
	recvReqs  []int64 // server receive timestamps
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
	RecvReqs []int  `json:"received_requests_per_second"`
	Type     string `json:"type"`
	Checksum bool   `json:"checksum"`
	TLS      bool   `json:"tls"`
}

func bucketizePerSecond(timestamps []int64, start, end int64) []int {
	if len(timestamps) == 0 || end <= start {
		return []int{}
	}

	oneSec := int64(time.Second)
	seconds := int((end - start) / oneSec)
	out := make([]int, seconds)
	for _, ts := range timestamps {
		if ts < start || ts >= end {
			continue
		}

		idx := int((ts - start) / oneSec)
		if idx >= 0 && idx < len(out) {
			out[idx]++
		}
	}

	return out
}

func GetMetrics() []MetricsOutput {
	db.mu.Lock()
	records := make([]*Metric, 0, len(db.data))
	for _, m := range db.data {
		records = append(records, m)
	}
	db.mu.Unlock()

	out := make([]MetricsOutput, 0, len(records))
	for _, metric := range db.data {
		metric.mu.Lock()
		reqs := append([]int64(nil), metric.reqs...)
		recvReqs := append([]int64(nil), metric.recvReqs...)
		start := metric.start
		var end *time.Time
		if metric.end != nil {
			t := *metric.end
			end = &t
		}
		recvStart := metric.recvStart
		var recvEnd *time.Time
		if metric.recvEnd != nil {
			t := *metric.recvEnd
			recvEnd = &t
		}
		id := metric.id
		algo := metric.algorithm
		checksum := metric.checksum
		tls := metric.tls

		metric.mu.Unlock()

		if len(reqs) == 0 && len(recvReqs) == 0 {
			continue
		}

		slices.Sort(reqs)
		slices.Sort(recvReqs)

		var reqsPerSec []int
		if end != nil {
			reqsPerSec = bucketizePerSecond(reqs, start.UnixNano(), end.UnixNano())
		}

		var recvPerSec []int
		if recvEnd != nil {
			recvPerSec = bucketizePerSecond(recvReqs, recvStart.UnixNano(), recvEnd.UnixNano())
		}

		out = append(out, MetricsOutput{
			Id:       id,
			Type:     algo.String(),
			Reqs:     reqsPerSec,
			RecvReqs: recvPerSec,
			Checksum: checksum,
			TLS:      tls,
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
		record.recvStart = time.Now()
		record.tls = tls
		db.data[parsedMessage.cmdId] = record
		log.Printf("Start gathering data of benchmark %d\n", parsedMessage.cmdId)

	case Continue:
		db.mu.Lock()
		record, ok := db.data[parsedMessage.cmdId]
		db.mu.Unlock()
		if !ok {
			log.Printf("Ignoring data for unknown benchmark id=%d", parsedMessage.cmdId)
			return
		}

		record.mu.Lock()
		defer record.mu.Unlock()
		if record.end != nil {
			log.Printf("Ignoring data because end benchmark already arrived")
			return
		}

		record.reqs = append(record.reqs, parsedMessage.timestamp)
		record.recvReqs = append(record.recvReqs, time.Now().UnixNano())

	case Stop:
		db.mu.Lock()
		record, ok := db.data[parsedMessage.cmdId]
		db.mu.Unlock()
		if !ok {
			log.Printf("Ignoring stop for unknown benchmark id=%d", parsedMessage.cmdId)
			return
		}

		log.Printf("Finish gathering data of benchmark %d\n", parsedMessage.cmdId)
		record.end = new(time.Unix(0, parsedMessage.timestamp))
		record.recvEnd = new(time.Now())
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
