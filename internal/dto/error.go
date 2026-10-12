package dto

// ErrorResponse is the body of every non-2xx response.
// Spec "Structured Error Handling": status, code, message, timestamp.
type ErrorResponse struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"request_id,omitempty"`
}
