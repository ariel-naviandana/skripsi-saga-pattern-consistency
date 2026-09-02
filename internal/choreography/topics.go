package choreography

// Topic names for the choreography approach. Must match deployments/setup.sh.
const (
	TopicOrderCreated     = "saga.order.created"
	TopicPaymentProcessed = "saga.payment.processed"
	TopicPaymentFailed    = "saga.payment.failed"
	TopicInventoryReserved = "saga.inventory.reserved"
	TopicInventoryFailed  = "saga.inventory.failed"
	TopicShippingScheduled = "saga.shipping.scheduled"
	TopicShippingFailed   = "saga.shipping.failed"
	TopicSagaClosed       = "saga.closed"
)
