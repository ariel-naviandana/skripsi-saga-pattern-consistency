package choreography

// OrderCreatedEvent is published by the order service to start a saga.
type OrderCreatedEvent struct {
	SagaID     string `json:"saga_id"`
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	Amount     int    `json:"amount"`
}

// PaymentResultEvent is published by the payment service.
type PaymentResultEvent struct {
	SagaID  string `json:"saga_id"`
	OrderID string `json:"order_id"`
	Amount  int    `json:"amount"`
	Success bool   `json:"success"`
}

// InventoryResultEvent is published by the inventory service.
type InventoryResultEvent struct {
	SagaID    string `json:"saga_id"`
	OrderID   string `json:"order_id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Success   bool   `json:"success"`
}

// ShippingResultEvent is published by the shipping service.
type ShippingResultEvent struct {
	SagaID  string `json:"saga_id"`
	OrderID string `json:"order_id"`
	Success bool   `json:"success"`
}

// SagaClosedEvent signals the saga has reached its final state.
type SagaClosedEvent struct {
	SagaID  string `json:"saga_id"`
	Success bool   `json:"success"`
}
