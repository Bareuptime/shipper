package models

import "time"

type DeploymentRequest struct {
	ServiceName string `json:"service_name"`
	SecretKey   string `json:"secret_key"`
	TagID       string `json:"tag_id"`
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
