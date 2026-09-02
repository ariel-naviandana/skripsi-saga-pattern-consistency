package choreography

import (
	"context"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/business"
)

// PaymentService wires the payment business service to choreography events.
type PaymentService struct {
	Biz     *business.PaymentService
	Pub     *Producer
	Brokers []string
}

// Run starts the consumers for the payment service. It blocks until ctx is done.
func (s *PaymentService) Run(ctx context.Context) error {
	errs := make(chan error, 2)

	// Forward: on order.created process payment.
	go func() {
		errs <- ConsumerGroup(ctx, s.Brokers, "payment-forward", []string{TopicOrderCreated}, func(ctx context.Context, msg []byte) error {
			ev, err := decode[OrderCreatedEvent](msg)
			if err != nil {
				return err
			}
			if err := s.Biz.ProcessPayment(ctx, ev.SagaID, ev.OrderID, ev.Amount); err != nil {
				return err
			}
			return s.Pub.Publish(TopicPaymentProcessed, ev.SagaID, PaymentResultEvent{
				SagaID: ev.SagaID, OrderID: ev.OrderID, Amount: ev.Amount, Success: true,
			})
		})
	}()

	// Reverse: on inventory.failed compensate payment and propagate failure.
	go func() {
		errs <- ConsumerGroup(ctx, s.Brokers, "payment-reverse", []string{TopicInventoryFailed}, func(ctx context.Context, msg []byte) error {
			ev, err := decode[InventoryResultEvent](msg)
			if err != nil {
				return err
			}
			if err := s.Biz.CompensatePayment(ctx, ev.SagaID); err != nil {
				return err
			}
			return s.Pub.Publish(TopicPaymentFailed, ev.SagaID, PaymentResultEvent{
				SagaID: ev.SagaID, OrderID: ev.OrderID, Amount: 0, Success: false,
			})
		})
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errs:
		return err
	}
}