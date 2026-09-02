package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
)

func NewConsumerGroup(brokers []string, groupID string) (sarama.ConsumerGroup, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	v, err := kafkaVersion()
	if err != nil {
		return nil, fmt.Errorf("kafka: parse version: %w", err)
	}
	config.Version = v
	cg, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("kafka: new consumer group: %w", err)
	}
	return cg, nil
}
