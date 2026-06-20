package metrics

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fishSim/internal/ecryption"
	"fishSim/internal/topics"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type _db struct {
	data map[int]*Metric
	mu   sync.Mutex
}

type Service struct {
	db       *_db
	dataFile string
}

type Metric struct {
	Id        int                 `json:"id"`
	Algorithm ecryption.Algorithm `json:"algorithm"`
	Start     int64               `json:"start"`
	End       *int64              `json:"end"`
	Checksum  bool                `json:"checksum"`
	Tls       bool                `json:"tls"`
	Reqs      []int64             `json:"reqs"`
}

type MetricsOutput struct {
	Id       int    `json:"id"`
	Reqs     []int  `json:"requests_per_second"`
	Type     string `json:"type"`
	Checksum bool   `json:"checksum"`
	TLS      bool   `json:"tls"`
}

func NewService(dataFile string) *Service {
	service := &Service{
		db:       load(dataFile),
		dataFile: dataFile,
	}

	return service
}

type outboundMessage struct {
	cmdId     int
	msgType   MsgType
	timestamp int64
	checksum  bool
}

type MsgType int

const (
	Start MsgType = iota
	Continue
	Stop
)

func (m *Metric) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Id        int     `json:"id"`
		Algorithm string  `json:"algorithm"`
		Start     int64   `json:"start"`
		End       *int64  `json:"end"`
		Checksum  bool    `json:"checksum"`
		Tls       bool    `json:"tls"`
		Reqs      []int64 `json:"reqs"`
	}{
		Id:        m.Id,
		Algorithm: m.Algorithm.String(),
		Start:     m.Start,
		End:       m.End,
		Checksum:  m.Checksum,
		Tls:       m.Tls,
		Reqs:      m.Reqs,
	})
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

func (s *Service) GetMetrics() []MetricsOutput {
	s.db.mu.Lock()
	records := make([]Metric, 0, len(s.db.data))
	for _, m := range s.db.data {
		records = append(records, *m)
	}
	s.db.mu.Unlock()

	out := make([]MetricsOutput, 0, len(records))
	for _, metric := range records {
		reqs := append([]int64(nil), metric.Reqs...)
		start := metric.Start
		var end *int64
		if metric.End != nil {
			t := *metric.End
			end = &t
		}

		id := metric.Id
		algo := metric.Algorithm
		checksum := metric.Checksum
		tls := metric.Tls

		slices.Sort(reqs)

		var reqsPerSec []int
		if end != nil {
			reqsPerSec = bucketizePerSecond(reqs, start, *end)
		}

		out = append(out, MetricsOutput{
			Id:       id,
			Type:     algo.String(),
			Reqs:     reqsPerSec,
			Checksum: checksum,
			TLS:      tls,
		})
	}

	slices.SortFunc(out, func(a, b MetricsOutput) int {
		return cmp.Compare(a.Id, b.Id)
	})

	return out
}

func persist(dataFile string, db *_db) {
	db.mu.Lock()
	data := make([]Metric, 0)
	for _, v := range db.data {
		data = append(data, *v)
	}
	db.mu.Unlock()

	raw, err := json.Marshal(data)
	if err != nil {
		log.Printf("Could not marshal metric json because of %s\n", err)
	}

	err = os.WriteFile(dataFile, raw, 0644)
	if err != nil {
		log.Printf("Could not write metric json file because of %s\n", err)
	}

	log.Printf("Persisted metrics in file %s\n", dataFile)
}

func load(dataFile string) *_db {
	data := make([]Metric, 0)
	file, err := os.OpenFile(dataFile, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("Could not open file %s because of %s\n", dataFile, err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Could not read file %s because of %s\n", dataFile, err)
	}

	if len(content) == 0 {
		return &_db{data: make(map[int]*Metric)}
	}

	err = json.Unmarshal(content, &data)
	if err != nil {
		log.Fatalf("Could not unmarshall file %s because of %s\n", dataFile, err)
	}

	db := &_db{data: make(map[int]*Metric, len(data))}
	for _, metric := range data {
		db.data[metric.Id] = &metric
	}

	return db
}

func (s *Service) HandleMessage(c mqtt.Client, message mqtt.Message) {
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
		record := new(Metric)
		record.Algorithm = algorithm
		record.Id = parsedMessage.cmdId
		record.Checksum = parsedMessage.checksum
		record.Start = parsedMessage.timestamp
		record.Tls = tls
		s.db.mu.Lock()
		s.db.data[parsedMessage.cmdId] = record
		s.db.mu.Unlock()
		log.Printf("Start gathering data of benchmark %d\n", parsedMessage.cmdId)

	case Continue:
		s.db.mu.Lock()
		defer s.db.mu.Unlock()
		record, ok := s.db.data[parsedMessage.cmdId]
		if !ok {
			log.Printf("Ignoring data for unknown benchmark id=%d", parsedMessage.cmdId)
			return
		}

		if record.End != nil {
			log.Printf("Ignoring data because end benchmark already arrived")
			return
		}

		record.Reqs = append(record.Reqs, parsedMessage.timestamp)

	case Stop:
		s.db.mu.Lock()
		record, ok := s.db.data[parsedMessage.cmdId]
		s.db.mu.Unlock()

		if !ok {
			log.Printf("Ignoring stop for unknown benchmark id=%d", parsedMessage.cmdId)
			return
		}

		t := parsedMessage.timestamp
		record.End = &t
		log.Printf("Finish gathering data of benchmark %d\n", parsedMessage.cmdId)
		persist(s.dataFile, s.db)
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
