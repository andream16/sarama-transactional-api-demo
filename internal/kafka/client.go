package kafka

import (
	"log"
	"time"

	"github.com/Shopify/sarama"
)

func NewClient(config *sarama.Config) sarama.Client {
	client, err := sarama.NewClient([]string{"kafka:9092"}, config)
	if err != nil {
		log.Printf("could not create sarama client: %v. Retrying\n", err)
		time.Sleep(2 * time.Second)
		return NewClient(config)
	}

	return client
}
