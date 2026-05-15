package todo

import "time"

// Todo is a task stored by the API.
type Todo struct {
	ID        string    `json:"id"`
	Task      string    `json:"task"`
	DueDate   time.Time `json:"due_date"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateRequest is the JSON body for POST /todos.
type CreateRequest struct {
	Task    string `json:"task"`
	DueDate string `json:"due_date"` // RFC3339
}

// UpdateRequest is the JSON body for PUT /todos/{id}.
type UpdateRequest struct {
	Task      *string `json:"task"`
	DueDate   *string `json:"due_date"` // RFC3339
	Completed *bool   `json:"completed"`
}
