package nomad

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bastion-deployment/internal/models"

	"github.com/sirupsen/logrus"
)

type Client struct {
	URL    string
	client *http.Client
	logger *logrus.Logger
}

func NewClient(url string) *Client {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	return &Client{
		URL:    url,
		client: &http.Client{Timeout: 30 * time.Second},
		logger: logger,
	}
}

// SetLogLevel sets the logging level for the client
func (c *Client) SetLogLevel(level logrus.Level) {
	c.logger.SetLevel(level)
}

// SetLogFormatter sets the logging formatter for the client
func (c *Client) SetLogFormatter(formatter logrus.Formatter) {
	c.logger.SetFormatter(formatter)
}

// Helper function to get map keys for logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (c *Client) TriggerDeployment(serviceName, tagID string) (string, error) {
	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"tag_id":       tagID,
		"nomad_url":    c.URL,
	}).Info("Starting deployment trigger")

	// Fetch existing job definition from Nomad
	getURL := fmt.Sprintf("%s/v1/job/%s", c.URL, serviceName)

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"get_url":      getURL,
	}).Debug("Fetching existing job definition from Nomad")

	resp, err := c.client.Get(getURL)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"get_url":      getURL,
			"error":        err.Error(),
		}).Error("Failed to fetch job definition from Nomad")
		return "", fmt.Errorf("failed to fetch job definition from Nomad: %v", err)
	}
	defer resp.Body.Close()

	c.logger.WithFields(logrus.Fields{
		"service_name":   serviceName,
		"status_code":    resp.StatusCode,
		"content_type":   resp.Header.Get("Content-Type"),
		"content_length": resp.Header.Get("Content-Length"),
	}).Debug("Received response from Nomad job fetch")

	if resp.StatusCode != http.StatusOK {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"status_code":  resp.StatusCode,
			"get_url":      getURL,
		}).Error("Nomad returned non-200 status for job fetch")
		return "", fmt.Errorf("failed to fetch job definition, Nomad returned status: %d", resp.StatusCode)
	}

	var jobResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"error":        err.Error(),
		}).Error("Failed to decode job definition response")
		return "", fmt.Errorf("failed to decode job definition response: %v", err)
	}

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"job_keys":     getMapKeys(jobResp),
	}).Debug("Successfully decoded job definition response")

	// Extract the job definition
	job, ok := jobResp["Job"].(map[string]interface{})
	if !ok {
		c.logger.WithFields(logrus.Fields{
			"service_name":  serviceName,
			"job_resp_type": fmt.Sprintf("%T", jobResp["Job"]),
		}).Error("Invalid job definition format - Job field missing or wrong type")
		return "", fmt.Errorf("invalid job definition format")
	}

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"job_id":       job["ID"],
		"job_name":     job["Name"],
		"job_type":     job["Type"],
		"job_keys":     getMapKeys(job),
	}).Debug("Successfully extracted job definition")

	// Append tag_id to Meta
	if job["Meta"] == nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
		}).Debug("Job Meta field is nil, creating new meta map")
		job["Meta"] = make(map[string]string)
	}

	meta, ok := job["Meta"].(map[string]interface{})
	if !ok {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"meta_type":    fmt.Sprintf("%T", job["Meta"]),
		}).Warn("Job Meta field has wrong type, reinitializing")
		job["Meta"] = make(map[string]string)
		meta = job["Meta"].(map[string]interface{})
	}

	// Log existing meta before modification
	c.logger.WithFields(logrus.Fields{
		"service_name":  serviceName,
		"existing_meta": meta,
	}).Debug("Current job metadata before adding tag_id")

	meta["tag_id"] = tagID

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"tag_id":       tagID,
		"updated_meta": meta,
	}).Info("Successfully added tag_id to job metadata")

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

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"payload_size": len(payloadBytes),
	}).Debug("Successfully marshaled job payload")

	// Make HTTP request to Nomad
	url := fmt.Sprintf("%s/v1/jobs", c.URL)

	c.logger.WithFields(logrus.Fields{
		"service_name": serviceName,
		"post_url":     url,
		"payload_size": len(payloadBytes),
	}).Info("Submitting updated job to Nomad")

	resp, err = c.client.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"service_name": serviceName,
			"post_url":     url,
			"error":        err.Error(),
		}).Error("Failed to submit job to Nomad")
		return "", fmt.Errorf("failed to submit job to Nomad: %v", err)
	}
	defer resp.Body.Close()

	c.logger.WithFields(logrus.Fields{
		"service_name":   serviceName,
		"status_code":    resp.StatusCode,
		"content_type":   resp.Header.Get("Content-Type"),
		"content_length": resp.Header.Get("Content-Length"),
	}).Debug("Received response from Nomad job submission")

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

	resp, err := client.Get(url)
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
		"eval_id":        evalID,
		"status_code":    resp.StatusCode,
		"content_type":   resp.Header.Get("Content-Type"),
		"content_length": resp.Header.Get("Content-Length"),
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
