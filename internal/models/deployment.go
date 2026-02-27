package models

import "time"

type DeploymentRequest struct {
	ServiceName string `json:"service_name"`
	TagID       string `json:"tag_id"` // Support tag_id format
}

type DeploymentResponse struct {
	Status  string `json:"status"`
	TagID   string `json:"tag_id"`
	JobID   string `json:"job_id,omitempty"`
	Message string `json:"message,omitempty"`
}

type StatusResponse struct {
	Status  string `json:"status"`
	TagID   string `json:"tag_id"`
	JobID   string `json:"job_id"`
	Message string `json:"message,omitempty"`
}

type Deployment struct {
	ID          int       `json:"id"`
	TagID       string    `json:"tag_id"`
	ServiceName string    `json:"service_name"`
	JobID       string    `json:"job_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type StreamEvent struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	EvalID       string `json:"eval_id,omitempty"`
	DeploymentID string `json:"deployment_id,omitempty"`
	Healthy      int    `json:"healthy,omitempty"`
	Total        int    `json:"total,omitempty"`
	Unhealthy    int    `json:"unhealthy,omitempty"`
	Timestamp    string `json:"timestamp"`
}

type DeleteResponse struct {
	Status      string `json:"status"`
	ServiceName string `json:"service_name"`
	Message     string `json:"message"`
	EvalID      string `json:"eval_id,omitempty"`
}
