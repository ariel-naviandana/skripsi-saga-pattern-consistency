package choreography

import (
	"context"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/business"
)

// OrderService wires the order business service to the choreography events.
type OrderService struct {
	Biz    *business.OrderService
	Pub    *Producer
	Brokers []string
}

// CreateOrderAndPublish persists the order and publishes order.created.
func (s *OrderService) CreateOrderAndPublish(ctx context.Context, customerID, productID string, quantity, amount int) (string, string, error) {
	sagaID := business.NewSagaID()
	orderID, err := s.Biz.CreateOrder(ctx, sagaID, customerID, productID, quantity, amount)
	if err != nil {
		return "", "", err
	}
	ev := OrderCreatedEvent{
		SagaID:     sagaID,
		OrderID:    orderID,
		CustomerID: customerID,
		ProductID:  productID,
		Quantity:   quantity,
		Amount:     amount,
	}
	if err := s.Pub.Publish(TopicOrderCreated, sagaID, ev); err != nil {
		return "", "", err
	}
	return sagaID, orderID, nil
}

// Run starts the consumers for the order service. It blocks until ctx is done.
func (s *OrderService) Run(ctx context.Context) error {
	errs := make(chan error, 1)

	// On shipping.scheduled the saga is fully successful.
	go func() {
		errs <- ConsumerGroup(ctx, s.Brokers, "order-success", []string{TopicShippingScheduled}, func(ctx context.Context, msg []byte) error {
			ev, err := decode[ShippingResultEvent](msg)
			if err != nil {
				return err
			}
			return s.Pub.Publish(TopicSagaClosed, ev.SagaID, SagaClosedEvent{SagaID: ev.SagaID, Success: true})
		})
	}()

	// On payment.failed the saga is fully rolled back: compensate order.
	go func() {
		errs <- ConsumerGroup(ctx, s.Brokers, "order-compensate", []string{TopicPaymentFailed}, func(ctx context.Context, msg []byte) error {
			ev, err := decode[PaymentResultEvent](msg)
			if err != nil {
				return err
			}
			if err := s.Biz.CompensateOrder(ctx, ev.SagaID); err != nil {
				return err
			}
			return s.Pub.Publish(TopicSagaClosed, ev.SagaID, SagaClosedEvent{SagaID: ev.SagaID, Success: false})
		})
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errs:
		return err
	}
}