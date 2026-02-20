package middleware

// ErrorResponse is the standard error format for BananaKit HTTP services.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
