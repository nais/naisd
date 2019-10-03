package kafka

import (
	"fmt"
	"github.com/golang/glog"
	"time"

	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"
	"github.com/nais/naisd/pkg/event"
)

// Send sends a message to Kafka
func (client *Client) Send(event deployment.Event) {
	client.SendQueue <- event
}

// ProducerLoop sends messages from the event queue in perpetuity
func (client *Client) ProducerLoop() {
	for message := range client.SendQueue {
		if err := client.send(message); err != nil {
			glog.Errorf("while sending deployment event to kafka: %s", err)
		}
	}
}

func (client *Client) send(event deployment.Event) error {
	payload, err := proto.Marshal(&event)
	if err != nil {
		return fmt.Errorf("while encoding Protobuf: %s", err)
	}

	reply := &sarama.ProducerMessage{
		Topic:     client.ProducerTopic,
		Timestamp: time.Now(),
		Value:     sarama.StringEncoder(payload),
	}

	_, offset, err := client.Producer.SendMessage(reply)
	if err != nil {
		return fmt.Errorf("while sending reply over Kafka: %s", err)
	}

	glog.Infof("Deployment event sent successfully; offset %d", offset)

	return nil
}
