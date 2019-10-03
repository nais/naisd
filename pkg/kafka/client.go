package kafka

import (
	"crypto/tls"
	"fmt"
	deployment "github.com/nais/naisd/pkg/event"

	"github.com/Shopify/sarama"
)

// Client is a Kafka client
type Client struct {
	RecvQ         chan sarama.ConsumerMessage
	Producer      sarama.SyncProducer
	ProducerTopic string
	SendQueue     chan deployment.Event
}

func tlsConfig(t TLS) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: t.Insecure,
	}
}

// NewClient takes a Kafka config object and returns a new client, or an error if the config is invalid.
func NewClient(cfg *Config) (*Client, error) {
	var err error
	client := &Client{}

	producerCfg := sarama.NewConfig()
	producerCfg.ClientID = fmt.Sprintf("%s-producer", cfg.ClientID)
	producerCfg.Net.SASL.Enable = cfg.SASL.Enabled
	producerCfg.Net.SASL.User = cfg.SASL.Username
	producerCfg.Net.SASL.Password = cfg.SASL.Password
	producerCfg.Net.SASL.Handshake = cfg.SASL.Handshake
	producerCfg.Producer.Return.Successes = true
	producerCfg.Producer.RequiredAcks = sarama.WaitForAll
	producerCfg.Producer.Return.Successes = true
	producerCfg.Net.TLS.Enable = cfg.TLS.Enabled
	producerCfg.Net.TLS.Config = tlsConfig(cfg.TLS)

	client.Producer, err = sarama.NewSyncProducer(cfg.Brokers, producerCfg)
	if err != nil {
		return nil, fmt.Errorf("while setting up Kafka producer: %s", err)
	}

	client.SendQueue = make(chan deployment.Event, 4096)

	client.ProducerTopic = cfg.Topic

	return client, nil
}
