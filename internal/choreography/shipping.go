package choreography

import (
	"context"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/business"
)

// ShippingService wires the shipping business service to choreography events.
type ShippingService struct {
	Biz     *business.ShippingService
	Pub     *Producer
	Brokers []string
}

// Run starts the consumers for the shipping service. It blocks until ctx is done.
func (s *ShippingService) Run(ctx context.Context) error {
	return ConsumerGroup(ctx, s.Brokers, "shipping-forward", []string{TopicInventoryReserved}, func(ctx context.Context, msg []byte) error {
		ev, err := decode[InventoryResultEvent](msg)
		if err != nil {
			return err
		}
		if err := s.Biz.ScheduleShipping(ctx, ev.SagaID, ev.OrderID); err != nil {
			return err
		}
		return s.Pub.Publish(TopicShippingScheduled, ev.SagaID, ShippingResultEvent{
			SagaID: ev.SagaID, OrderID: ev.OrderID, Success: true,
		})
	})
}