package models

type DecisionUser struct {
	Key        string `validate:"required"`
	Attributes map[string]string
}

type DecisionResource struct {
	Type       string `validate:"required"`
	Attributes map[string]string
}

type DecisionRequest struct {
	User     DecisionUser     `validate:"required"`
	Action   string           `validate:"required"`
	Resource DecisionResource `validate:"required"`
}

type DecisionResponse struct {
	DecisionID string      `json:"decision_id"`
	Result     interface{} `json:"result"`
}
