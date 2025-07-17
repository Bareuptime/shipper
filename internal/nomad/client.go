package nomad

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"bastion-deployment/internal/logger"
	"bastion-deployment/internal/models"

	"github.com/sirupsen/logrus"
)

type Client struct {
	URL    string
	Token  string
	client *http.Client
	logger *logrus.Entry
}

func NewClient(url string, skipTLSVerify bool, token string) *Client {
	// Get a logger instance with the nomad client module context
	clientLogger := logger.WithModule("nomad-client")

	// Create custom transport with TLS verification option
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerify,
		},
	}

	return &Client{
		URL:   url,
		Token: token,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		logger: clientLogger,
	}
}

// GetLogger returns the client's logger instance
func (c *Client) GetLogger() *logrus.Entry {
	return c.logger
}

func (c *Client) TriggerDeployment(serviceName, tagID string) (string, error) {
	// Use the existing client logger
	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"tag_id":       tagID,
		"nomad_url":    c.URL,
	}).Info("Starting deployment trigger111111111")

	// Fetch existing job definition from Nomad
	getURL := fmt.Sprintf("%s/v1/job/%s", c.URL, serviceName)

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"get_url":      getURL,
	}).Debug("Fetching existing job definition from Nomad")
	log.Print("Fetching existing job definition from Nomad: ", getURL)

	// Create request with token header
	req, _ := http.NewRequest("GET", getURL, nil)
	req.Header.Add("X-Nomad-Token", c.Token)

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"get_url":      getURL,
			"error":        err.Error(),
		}).Error("Failed to fetch job definition from Nomad")
		return "", fmt.Errorf("failed to fetch job definition from Nomad: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"status_code":  resp.StatusCode,
			"get_url":      getURL,
		}).Error("Nomad returned non-200 status for job fetch")
		return "", fmt.Errorf("failed to fetch job definition, Nomad returned status: %d", resp.StatusCode)
	}

	var jobSpec map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jobSpec); err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"error":        err.Error(),
		}).Error("Failed to decode job definition response")
		return "", fmt.Errorf("failed to decode job definition response: %v", err)
	}

	log.Print("Fetching existing job json definition from Nomad: ", jobSpec)

	// can you print resp.body for debugging
	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"job_spec":     jobSpec,
	}).Debug("Fetched job definition from Nomad")

	// Extract the job definition
	job, ok := jobSpec["Job"].(map[string]interface{})
	if !ok {
		c.logger.WithFields(logrus.Fields{
			"service_name":  serviceName,
			"job_resp_type": fmt.Sprintf("%T", jobSpec["Job"]),
		}).Error("Invalid job definition format - Job field missing or wrong type")
		return "", fmt.Errorf("invalid job definition format")
	}

	// Create or update the Meta field
	newMeta := map[string]interface{}{
		"tag_id":     tagID,
		"timestamp":  fmt.Sprintf("%d", time.Now().Unix()),
		"updated_by": "bastion",
	}

	// If Meta already exists, preserve existing values not overridden
	if job["Meta"] != nil {
		if existingMeta, ok := job["Meta"].(map[string]interface{}); ok {
			for k, v := range existingMeta {
				if _, exists := newMeta[k]; !exists {
					newMeta[k] = v
				}
			}
		}
	}

	// Update job's Meta with the new metadata
	job["Meta"] = newMeta

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"tag_id":       tagID,
		"updated_meta": newMeta,
	}).Info("Applied metadata to job definition")

	// Create the job payload with the updated job definition
	jobPayload := map[string]interface{}{
		"Job": job,
	}

	// Convert to JSON
	payloadBytes, err := json.Marshal(jobPayload)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"error":        err.Error(),
		}).Error("Failed to marshal job payload to JSON")
		return "", fmt.Errorf("failed to marshal job payload: %v", err)
	}

	// Make HTTP request to Nomad
	url := fmt.Sprintf("%s/v1/jobs", c.URL)

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"post_url":     url,
		"payload_size": len(payloadBytes),
	}).Info("Submitting updated job to Nomad")

	// Create POST request with token header
	req, err = http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"post_url":     url,
			"error":        err.Error(),
		}).Error("Failed to create POST request")
		return "", fmt.Errorf("failed to create POST request: %v", err)
	}

	// Set content type and add token header
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Add("X-Nomad-Token", c.Token)
	}

	resp, err = c.client.Do(req)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"post_url":     url,
			"error":        err.Error(),
		}).Error("Failed to submit job to Nomad")
		return "", fmt.Errorf("failed to submit job to Nomad: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"status_code":  resp.StatusCode,
			"post_url":     url,
		}).Error("Nomad returned non-200 status for job submission")
		return "", fmt.Errorf("nomad returned status: %d", resp.StatusCode)
	}

	var nomadResp models.NomadJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&nomadResp); err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"error":        err.Error(),
		}).Error("Failed to decode Nomad job submission response")
		return "", fmt.Errorf("failed to decode Nomad response: %v", err)
	}

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"tag_id":       tagID,
		"eval_id":      nomadResp.EvalID,
		"job_id":       nomadResp.JobID,
	}).Info("Successfully triggered deployment")

	return nomadResp.EvalID, nil
}

func (c *Client) GetJobStatus(evalID string) (string, error) {
	c.logger.WithFields(logrus.Fields{
		"eval_id":   evalID,
		"nomad_url": c.URL,
	}).Info("Starting job status check")

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/v1/evaluation/%s", c.URL, evalID)

	c.logger.WithFields(logrus.Fields{
		"eval_id":    evalID,
		"status_url": url,
		"timeout":    "10s",
	}).Debug("Making request to get evaluation status")

	// Create request with token header
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"eval_id":    evalID,
			"status_url": url,
			"error":      err.Error(),
		}).Error("Failed to create GET request")
		return "", fmt.Errorf("failed to create GET request: %v", err)
	}

	// Add token header if available
	if c.Token != "" {
		req.Header.Add("X-Nomad-Token", c.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"eval_id":    evalID,
			"status_url": url,
			"error":      err.Error(),
		}).Error("Failed to get job status from Nomad")
		return "", fmt.Errorf("failed to get job status from Nomad: %v", err)
	}
	defer resp.Body.Close()

	c.logger.WithFields(logrus.Fields{
		"eval_id":     evalID,
		"status_code": resp.StatusCode,
	}).Debug("Received response from Nomad status check")

	if resp.StatusCode != http.StatusOK {
		c.logger.WithFields(logrus.Fields{
			"eval_id":     evalID,
			"status_code": resp.StatusCode,
			"status_url":  url,
		}).Error("Nomad returned non-200 status for evaluation status")
		return "", fmt.Errorf("nomad returned status: %d", resp.StatusCode)
	}

	var evalResp models.NomadEvalResponse
	if err := json.NewDecoder(resp.Body).Decode(&evalResp); err != nil {
		c.logger.WithFields(logrus.Fields{
			"eval_id": evalID,
			"error":   err.Error(),
		}).Error("Failed to decode Nomad evaluation response")
		return "", fmt.Errorf("failed to decode Nomad response: %v", err)
	}

	c.logger.WithFields(logrus.Fields{
		"eval_id":      evalID,
		"nomad_status": evalResp.Status,
	}).Debug("Successfully decoded Nomad evaluation response")

	// Map Nomad status to our status
	var mappedStatus string
	switch evalResp.Status {
	case "complete":
		mappedStatus = "completed"
	case "failed":
		mappedStatus = "failed"
	case "pending":
		mappedStatus = "running"
	default:
		mappedStatus = "running"
	}

	c.logger.WithFields(logrus.Fields{
		"eval_id":       evalID,
		"nomad_status":  evalResp.Status,
		"mapped_status": mappedStatus,
	}).Info("Successfully mapped job status")

	return mappedStatus, nil
}
