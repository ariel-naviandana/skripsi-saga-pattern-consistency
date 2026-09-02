package orchestration

// StepRequest is the payload sent by the orchestrator to each service.
type StepRequest struct {
	SagaID     string `json:"saga_id"`
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	Amount     int    `json:"amount"`
}

// StepResponse is the payload returned by each service.
type StepResponse struct {
	OrderID string `json:"order_id,omitempty"`
	Status  string `json:"status"`
}

// StartRequest is the payload to start a new orchestrated saga.
type StartRequest struct {
	CustomerID string `json:"customer_id"`
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	Amount     int    `json:"amount"`
}

// StartResponse is returned after the saga attempt.
type StartResponse struct {
	SagaID string `json:"saga_id"`
	Status string `json:"status"`
}