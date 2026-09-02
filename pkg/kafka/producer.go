package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
)

func NewSyncProducer(brokers []string) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	v, err := kafkaVersion()
	if err != nil {
		return nil, fmt.Errorf("kafka: parse version: %w", err)
	}
	config.Version = v
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("kafka: new producer: %w", err)
	}
	return producer, nil
}
