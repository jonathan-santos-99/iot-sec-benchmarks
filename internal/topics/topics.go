package topics

import (
	"crypto/aes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fishSim/internal/ecryption"
	"log"
	"os"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
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

func newMqttOpts(clientid, server, username, password string) *mqtt.ClientOptions {
	opts := mqtt.
		NewClientOptions().
		AddBroker(server).
		SetClientID(clientid).
		SetCleanSession(true).
		SetKeepAlive(2 * time.Second).
		SetPingTimeout(1 * time.Second).
		SetUsername(username).
		SetPassword(password)

	opts.SetConnectionNotificationHandler(func(client mqtt.Client, n mqtt.ConnectionNotification) {
		switch nt := n.(type) {
		case mqtt.ConnectionNotificationConnected:
			log.Printf("[NOTIFICATION] connected\n")
		case mqtt.ConnectionNotificationConnecting:
			log.Printf("[NOTIFICATION] connecting (isReconnect=%t) [%d]\n",
				nt.IsReconnect, nt.Attempt)
		case mqtt.ConnectionNotificationFailed:
			log.Printf("[NOTIFICATION] connection failed: %v\n", nt.Reason)
		case mqtt.ConnectionNotificationLost:
			log.Printf("[NOTIFICATION] connection lost: %v\n", nt.Reason)
		case mqtt.ConnectionNotificationBroker:
			log.Printf("[NOTIFICATION] broker connection: %s\n", nt.Broker.String())
		case mqtt.ConnectionNotificationBrokerFailed:
			log.Printf("[NOTIFICATION] broker connection failed: %v [%s]\n",
				nt.Reason, nt.Broker.String())
		}
	})

	return opts
}

func ClientId(prefix string) string {
	hostname, _ := os.Hostname()
	return prefix + hostname + strconv.Itoa(time.Now().Second())
}

func StartClient(
	server,
	username,
	password,
	clientid string,
) mqtt.Client {
	opts := newMqttOpts(clientid, "tcp://"+server+":1883", username, password)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return c
}

func StartTLSClient(cafile, clientCrt, clientKey, server, username, password, clientid string) mqtt.Client {
	certpool := x509.NewCertPool()
	pemCerts, err := os.ReadFile(cafile)
	if err != nil {
		panic(err)
	}

	certpool.AppendCertsFromPEM(pemCerts)

	cert, err := tls.LoadX509KeyPair(clientCrt, clientKey)
	if err != nil {
		panic(err)
	}

	tlsconfig := &tls.Config{
		RootCAs:            certpool,
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	opts := newMqttOpts(clientid, "ssl://"+server+":8883", username, password)
	opts.SetTLSConfig(tlsconfig)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return c
}
