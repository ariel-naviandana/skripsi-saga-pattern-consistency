package common

const (
	StatePending      = "pending"
	StateCommitted    = "committed"
	StateCompensating = "compensating"
	StateCompensated  = "compensated"
	StateFailed       = "failed"
)

type SagaStep string

const (
	StepOrder     SagaStep = "order"
	StepPayment   SagaStep = "payment"
	StepInventory SagaStep = "inventory"
	StepShipping  SagaStep = "shipping"
)

type OrderRequest struct {
	CustomerID string `json:"customer_id"`
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	Amount     int    `json:"amount"`
}

type SagaLog struct {
	ID        int64  `json:"id"`
	SagaID    string `json:"saga_id"`
	Step      string `json:"step"`
	Status    string `json:"status"`
	Snapshot  string `json:"snapshot"`
	CreatedAt string `json:"created_at"`
}
