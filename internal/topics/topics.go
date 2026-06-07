package topics

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type Algorithm int

const (
	PlainText Algorithm = iota
	AES
)

func (a *Algorithm) UnmarshalText(text []byte) error {
	name := string(text)
	switch strings.ToUpper(name) {
	case "PLAIN_TEXT":
		*a = PlainText
	case "AES":
		*a = AES
	default:
		return fmt.Errorf("Could not parse %s as algorithm", text)
	}

	return nil
}

func (a Algorithm) String() string {
	return [...]string{"PLAIN_TEXT", "AES"}[a]
}

var OutboundTopics = make(map[Algorithm]struct {
	Topic string
	Key   string
})

func ParseConfigFile(file string) {
	raw, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("Error opening %s: %s", file, err)
	}

	err = json.Unmarshal(raw, &OutboundTopics)
	if err != nil {
		log.Fatalf("Error opening %s: %s", file, err)
	}
}

func FindAlgorithm(topic string) (Algorithm, bool) {
	for algo, topicInfo := range OutboundTopics {
		if topicInfo.Topic == topic {
			return algo, true
		}
	}

	return Algorithm(0), false
}
