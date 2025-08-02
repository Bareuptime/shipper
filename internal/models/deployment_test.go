package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeploymentRequest(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected DeploymentRequest
		wantErr  bool
	}{
		{
			name:     "valid deployment request",
			jsonData: `{"service_name": "test-service", "tag_id": "test-123"}`,
			expected: DeploymentRequest{
				ServiceName: "test-service",
				TagID:       "test-123",
			},
			wantErr: false,
		},
		{
			name:     "missing service_name",
			jsonData: `{"tag_id": "test-123"}`,
			expected: DeploymentRequest{
				TagID: "test-123",
			},
			wantErr: false,
		},
		{
			name:     "missing tag_id",
			jsonData: `{"service_name": "test-service"}`,
			expected: DeploymentRequest{
				ServiceName: "test-service",
			},
			wantErr: false,
		},
		{
			name:     "invalid json",
			jsonData: `{"service_name": "test-service", "tag_id":}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req DeploymentRequest
			err := json.Unmarshal([]byte(tt.jsonData), &req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if req.ServiceName != tt.expected.ServiceName {
				t.Errorf("ServiceName = %v, want %v", req.ServiceName, tt.expected.ServiceName)
			}

			if req.TagID != tt.expected.TagID {
				t.Errorf("TagID = %v, want %v", req.TagID, tt.expected.TagID)
			}
		})
	}
}

func TestDeploymentResponse(t *testing.T) {
	response := DeploymentResponse{
		Status:  "success",
		TagID:   "test-123",
		JobID:   "job-456",
		Message: "deployment successful",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var unmarshaled DeploymentResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if unmarshaled.Status != response.Status {
		t.Errorf("Status = %v, want %v", unmarshaled.Status, response.Status)
	}

	if unmarshaled.TagID != response.TagID {
		t.Errorf("TagID = %v, want %v", unmarshaled.TagID, response.TagID)
	}

	if unmarshaled.JobID != response.JobID {
		t.Errorf("JobID = %v, want %v", unmarshaled.JobID, response.JobID)
	}

	if unmarshaled.Message != response.Message {
		t.Errorf("Message = %v, want %v", unmarshaled.Message, response.Message)
	}
}

func TestStatusResponse(t *testing.T) {
	response := StatusResponse{
		Status:  "running",
		TagID:   "test-123",
		JobID:   "job-456",
		Message: "deployment in progress",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	expected := `{"status":"running","tag_id":"test-123","job_id":"job-456","message":"deployment in progress"}`
	if string(jsonData) != expected {
		t.Errorf("JSON = %v, want %v", string(jsonData), expected)
	}
}

func TestDeployment(t *testing.T) {
	now := time.Now()
	deployment := Deployment{
		ID:          1,
		TagID:       "test-123",
		ServiceName: "test-service",
		JobID:       "job-456",
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(deployment)
	if err != nil {
		t.Fatalf("failed to marshal deployment: %v", err)
	}

	var unmarshaled Deployment
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal deployment: %v", err)
	}

	if unmarshaled.ID != deployment.ID {
		t.Errorf("ID = %v, want %v", unmarshaled.ID, deployment.ID)
	}

	if unmarshaled.TagID != deployment.TagID {
		t.Errorf("TagID = %v, want %v", unmarshaled.TagID, deployment.TagID)
	}

	if unmarshaled.ServiceName != deployment.ServiceName {
		t.Errorf("ServiceName = %v, want %v", unmarshaled.ServiceName, deployment.ServiceName)
	}

	if unmarshaled.JobID != deployment.JobID {
		t.Errorf("JobID = %v, want %v", unmarshaled.JobID, deployment.JobID)
	}

	if unmarshaled.Status != deployment.Status {
		t.Errorf("Status = %v, want %v", unmarshaled.Status, deployment.Status)
	}
}
