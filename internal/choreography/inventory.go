package choreography

import (
	"context"

	"github.com/ariel-naviandana/skripsi-saga-pattern-consistency/internal/business"
)

// InventoryService wires the inventory business service to choreography events.
type InventoryService struct {
	Biz     *business.InventoryService
	Pub     *Producer
	Brokers []string
}

// Run starts the consumers for the inventory service. It blocks until ctx is done.
func (s *InventoryService) Run(ctx context.Context) error {
	errs := make(chan error, 2)

	// Forward: on payment.processed reserve inventory.
	go func() {
		errs <- ConsumerGroup(ctx, s.Brokers, "inventory-forward", []string{TopicPaymentProcessed}, func(ctx context.Context, msg []byte) error {
			ev, err := decode[PaymentResultEvent](msg)
			if err != nil {
				return err
			}
			// Amount is not needed here; product comes from a lookup stored by the
			// payment event's order. To keep the demo self-contained we re-derive
			// the product from the order table of the order service, which is not
			// shared here. For S1 baseline we use a fixed product id.
			if err := s.Biz.ReserveInventory(ctx, ev.SagaID, "product-1", 1); err != nil {
				return err
			}
			return s.Pub.Publish(TopicInventoryReserved, ev.SagaID, InventoryResultEvent{
				SagaID: ev.SagaID, OrderID: ev.OrderID, ProductID: "product-1", Quantity: 1, Success: true,
			})
		})
	}()

	// Reverse: on shipping.failed compensate inventory and propagate failure.
	go func() {
		errs <- ConsumerGroup(ctx, s.Brokers, "inventory-reverse", []string{TopicShippingFailed}, func(ctx context.Context, msg []byte) error {
			ev, err := decode[ShippingResultEvent](msg)
			if err != nil {
				return err
			}
			if err := s.Biz.CompensateInventory(ctx, ev.SagaID); err != nil {
				return err
			}
			return s.Pub.Publish(TopicInventoryFailed, ev.SagaID, InventoryResultEvent{
				SagaID: ev.SagaID, OrderID: ev.OrderID, ProductID: "product-1", Quantity: 1, Success: false,
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