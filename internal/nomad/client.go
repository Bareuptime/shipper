package nomad

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bastion-deployment/internal/models"
)

type Client struct {
	URL    string
	client *http.Client
}

func NewClient(url string) *Client {
	return &Client{
		URL:    url,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) TriggerDeployment(serviceName, tagID string) (string, error) {
	// Create Nomad job payload
	jobPayload := map[string]interface{}{
		"Job": map[string]interface{}{
			"ID":   fmt.Sprintf("%s-%s", serviceName, tagID[:8]),
			"Name": serviceName,
			"Type": "service",
			"Meta": map[string]string{
				"tag_id": tagID,
			},
			"TaskGroups": []map[string]interface{}{
				{
					"Name": serviceName,
					"Tasks": []map[string]interface{}{
						{
							"Name":   serviceName,
							"Driver": "docker",
							"Config": map[string]interface{}{
								"image": fmt.Sprintf("%s:latest", serviceName),
							},
						},
					},
				},
			},
		},
	}

	// Convert to JSON
	payloadBytes, err := json.Marshal(jobPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal job payload: %v", err)
	}

	// Make HTTP request to Nomad
	url := fmt.Sprintf("%s/v1/jobs", c.URL)

	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to submit job to Nomad: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Nomad returned status: %d", resp.StatusCode)
	}

	var nomadResp models.NomadJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&nomadResp); err != nil {
		return "", fmt.Errorf("failed to decode Nomad response: %v", err)
	}

	return nomadResp.EvalID, nil
}

func (c *Client) GetJobStatus(evalID string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/v1/evaluation/%s", c.URL, evalID)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get job status from Nomad: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Nomad returned status: %d", resp.StatusCode)
	}

	var evalResp models.NomadEvalResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		return "", fmt.Errorf("failed to decode Nomad response: %v", err)
	}

	// Map Nomad status to our status
	switch evalResp.Status {
	case "complete":
		return "completed", nil
	case "failed":
		return "failed", nil
	case "pending":
		return "running", nil
	default:
		return "running", nil
	}
}
