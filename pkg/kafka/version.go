package kafka

import "github.com/IBM/sarama"

func kafkaVersion() (sarama.KafkaVersion, error) {
	return sarama.ParseKafkaVersion("3.6.0")
}
