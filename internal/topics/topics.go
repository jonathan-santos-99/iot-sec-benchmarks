package topics

import (
	"crypto/aes"
	"encoding/json"
	"fishSim/internal/ecryption"
	"log"
	"os"
)

var OutboundTopics = make(map[ecryption.Algorithm]struct {
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

	for alg, inf := range OutboundTopics {
		switch alg {
		case ecryption.AES:
			{
				block, err := aes.NewCipher([]byte(inf.Key))
				if err != nil {
					log.Panicf("Error creating AES block cypher: %s\n", err)
				}

				ecryption.Cyphers[ecryption.AES] = block
			}
		}
	}
}

func FindAlgorithm(topic string) (ecryption.Algorithm, bool) {
	for algo, topicInfo := range OutboundTopics {
		if topicInfo.Topic == topic {
			return algo, true
		}
	}

	return ecryption.Algorithm(0), false
}
