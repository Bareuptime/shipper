package test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"shipper-deployment/internal/models"
)

func TestDeploymentRequest(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected models.DeploymentRequest
		wantErr  bool
	}{
		{
			name:     "valid deployment request",
			jsonData: `{"service_name": "test-service", "tag_id": "test-123"}`,
			expected: models.DeploymentRequest{
				ServiceName: "test-service",
				TagID:       "test-123",
			},
			wantErr: false,
		},
		{
			name:     "missing service_name",
			jsonData: `{"tag_id": "test-123"}`,
			expected: models.DeploymentRequest{
				TagID: "test-123",
			},
			wantErr: false,
		},
		{
			name:     "missing tag_id",
			jsonData: `{"service_name": "test-service"}`,
			expected: models.DeploymentRequest{
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
			var req models.DeploymentRequest
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
	response := models.DeploymentResponse{
		Status:  "success",
		TagID:   "test-123",
		JobID:   "job-456",
		Message: "deployment successful",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var unmarshaled models.DeploymentResponse
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
	response := models.StatusResponse{
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
	deployment := models.Deployment{
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

	var unmarshaled models.Deployment
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

func TestStreamEvent(t *testing.T) {
	event := models.StreamEvent{
		Status:       "running",
		Message:      "Deployment running: 2/3 healthy",
		EvalID:       "eval-123",
		DeploymentID: "deploy-456",
		Healthy:      2,
		Total:        3,
		Unhealthy:    0,
		Timestamp:    "2024-01-01T00:00:00Z",
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal stream event: %v", err)
	}

	var unmarshaled models.StreamEvent
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal stream event: %v", err)
	}

	if unmarshaled.Status != event.Status {
		t.Errorf("Status = %v, want %v", unmarshaled.Status, event.Status)
	}
	if unmarshaled.DeploymentID != event.DeploymentID {
		t.Errorf("DeploymentID = %v, want %v", unmarshaled.DeploymentID, event.DeploymentID)
	}
	if unmarshaled.Healthy != event.Healthy {
		t.Errorf("Healthy = %v, want %v", unmarshaled.Healthy, event.Healthy)
	}
	if unmarshaled.Total != event.Total {
		t.Errorf("Total = %v, want %v", unmarshaled.Total, event.Total)
	}
}

func TestStreamEventOmitsEmpty(t *testing.T) {
	event := models.StreamEvent{
		Status:    "completed",
		Message:   "Done",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal stream event: %v", err)
	}

	jsonStr := string(jsonData)
	if strings.Contains(jsonStr, "eval_id") {
		t.Error("Expected eval_id to be omitted when empty")
	}
	if strings.Contains(jsonStr, "deployment_id") {
		t.Error("Expected deployment_id to be omitted when empty")
	}
}

func TestDeleteResponse(t *testing.T) {
	response := models.DeleteResponse{
		Status:      "stopped",
		ServiceName: "my-service",
		Message:     "Job stopped successfully",
		EvalID:      "eval-789",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal delete response: %v", err)
	}

	var unmarshaled models.DeleteResponse
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal delete response: %v", err)
	}

	if unmarshaled.Status != response.Status {
		t.Errorf("Status = %v, want %v", unmarshaled.Status, response.Status)
	}
	if unmarshaled.ServiceName != response.ServiceName {
		t.Errorf("ServiceName = %v, want %v", unmarshaled.ServiceName, response.ServiceName)
	}
	if unmarshaled.EvalID != response.EvalID {
		t.Errorf("EvalID = %v, want %v", unmarshaled.EvalID, response.EvalID)
	}
}

func TestNomadEvalResponseExtended(t *testing.T) {
	jsonData := `{
		"ID": "eval-123",
		"Status": "complete",
		"StatusDescription": "all allocations placed",
		"JobID": "my-job",
		"DeploymentID": "deploy-456",
		"BlockedEval": "",
		"FailedTGAllocs": null
	}`

	var evalResp models.NomadEvalResponse
	if err := json.Unmarshal([]byte(jsonData), &evalResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if evalResp.ID != "eval-123" {
		t.Errorf("ID = %v, want eval-123", evalResp.ID)
	}
	if evalResp.Status != "complete" {
		t.Errorf("Status = %v, want complete", evalResp.Status)
	}
	if evalResp.DeploymentID != "deploy-456" {
		t.Errorf("DeploymentID = %v, want deploy-456", evalResp.DeploymentID)
	}
	if evalResp.JobID != "my-job" {
		t.Errorf("JobID = %v, want my-job", evalResp.JobID)
	}
}

func TestNomadDeploymentResponse(t *testing.T) {
	jsonData := `{
		"ID": "deploy-123",
		"Status": "running",
		"StatusDescription": "Deployment is running",
		"JobID": "my-job",
		"TaskGroups": {
			"web": {
				"DesiredCanaries": 0,
				"DesiredTotal": 3,
				"HealthyAllocs": 2,
				"UnhealthyAllocs": 0,
				"PlacedAllocs": 3,
				"AutoRevert": false
			}
		}
	}`

	var deployResp models.NomadDeploymentResponse
	if err := json.Unmarshal([]byte(jsonData), &deployResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if deployResp.Status != "running" {
		t.Errorf("Status = %v, want running", deployResp.Status)
	}
	if len(deployResp.TaskGroups) != 1 {
		t.Fatalf("Expected 1 task group, got %d", len(deployResp.TaskGroups))
	}

	webTG := deployResp.TaskGroups["web"]
	if webTG.DesiredTotal != 3 {
		t.Errorf("DesiredTotal = %v, want 3", webTG.DesiredTotal)
	}
	if webTG.HealthyAllocs != 2 {
		t.Errorf("HealthyAllocs = %v, want 2", webTG.HealthyAllocs)
	}
}
