package dto

// Response represents a standard API response envelope containing a message and optional data.
type Response[T any] struct {
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}
