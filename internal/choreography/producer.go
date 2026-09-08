package choreography

import (
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/common"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/pkg/kafka"
)

// Producer wraps a Sarama SyncProducer for publishing saga events.
type Producer struct {
	producer sarama.SyncProducer
	Fault    *common.FaultConfig
}

func NewProducer(brokers []string, fault *common.FaultConfig) (*Producer, error) {
	p, err := kafka.NewSyncProducer(brokers)
	if err != nil {
		return nil, err
	}
	return &Producer{producer: p, Fault: fault}, nil
}

// Publish serializes v as JSON and sends it to topic with saga_id as key
// to guarantee ordering per saga (with a single partition this is moot, but
// keeping the key is good practice).
func (p *Producer) Publish(topic string, key string, v any) error {
	if p.Fault != nil && p.Fault.ShouldDropEvent(topic) {
		// S8: the event is silently dropped while the caller reports success,
		// simulating the dual-write / non-transactional queuing failure mode.
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data),
	}
	_, _, err = p.producer.SendMessage(msg)
	return err
}

func (p *Producer) Close() error {
	return p.producer.Close()
}
