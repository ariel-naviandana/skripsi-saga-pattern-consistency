package choreography

import (
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/pkg/kafka"
)

// MessageHandler processes one consumed event. The handler must be safe for
// concurrent invocation.
type MessageHandler func(ctx context.Context, msg []byte) error

// ConsumerGroup runs a consumer group that subscribes to topics and dispatches
// messages to handler. It blocks until ctx is cancelled.
func ConsumerGroup(ctx context.Context, brokers []string, groupID string, topics []string, handler MessageHandler) error {
	cg, err := kafka.NewConsumerGroup(brokers, groupID)
	if err != nil {
		return err
	}
	defer cg.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Consume returns an error if the group session is interrupted, which is
		// expected on rebalance; keep looping.
		func() {
			handlerErr := func(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
				for msg := range claim.Messages() {
					if err := handler(context.Background(), msg.Value); err != nil {
						log.Printf("choreography: handler error (topic=%s): %v", msg.Topic, err)
					}
					sess.MarkMessage(msg, "")
				}
				return nil
			}
			// nolint:errcheck // errors here are normal (rebalance/close)
			_ = cg.Consume(ctx, topics, &groupHandler{fn: handlerErr})
		}()
	}
}

type groupHandler struct {
	fn func(sarama.ConsumerGroupSession, sarama.ConsumerGroupClaim) error
}

func (h *groupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *groupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }
func (h *groupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	return h.fn(sess, claim)
}

func decode[T any](msg []byte) (T, error) {
	var v T
	err := json.Unmarshal(msg, &v)
	return v, err
}