package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"shipper-deployment/internal/config"
	"shipper-deployment/internal/database"
	"shipper-deployment/internal/models"
	"shipper-deployment/internal/nomad"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	db     *sql.DB
	config *config.Config
	nomad  *nomad.Client
	logger *logrus.Entry
}

func NewHandler(db *sql.DB, cfg *config.Config, nomadClient *nomad.Client) *Handler {
	// Use the same logger as the nomad client for consistency
	return &Handler{
		db:     db,
		config: cfg,
		nomad:  nomadClient,
		logger: nomadClient.GetLogger(),
	}
}

// writeJSONResponse is a helper function to write JSON responses with error handling
func (h *Handler) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.WithError(err).Error("Failed to encode JSON response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSONResponse(w, map[string]string{"status": "healthy", "time": time.Now().Format(time.RFC3339)})
}

func (h *Handler) DeployJob(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form data (max 1MB)
	err := r.ParseMultipartForm(1024 * 1024) // 1MB
	if err != nil {
		h.logger.WithError(err).Error("Failed to parse multipart form")
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	// Debug: Log all form values and files
	h.logger.WithFields(logrus.Fields{
		"form_values": r.Form,
		"post_form":   r.PostForm,
		"multipart":   r.MultipartForm != nil,
	}).Debug("Parsed multipart form")

	if r.MultipartForm != nil {
		h.logger.WithFields(logrus.Fields{
			"files": func() map[string][]string {
				files := make(map[string][]string)
				for key, fileHeaders := range r.MultipartForm.File {
					filenames := make([]string, len(fileHeaders))
					for i, fh := range fileHeaders {
						filenames[i] = fh.Filename
					}
					files[key] = filenames
				}
				return files
			}(),
			"values": r.MultipartForm.Value,
		}).Debug("Multipart form details")
	}

	// Get tag_id from form
	tagID := r.FormValue("tag_id")
	if tagID == "" {
		h.logger.Error("Tag ID is missing in request")
		http.Error(w, "Tag ID is required", http.StatusBadRequest)
		return
	}

	h.logger.WithField("tag_id", tagID).Info("Job deployment request received")

	// Get the uploaded job file
	file, fileHeader, err := r.FormFile("job_file")
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"available_files": func() []string {
				var files []string
				if r.MultipartForm != nil {
					for key := range r.MultipartForm.File {
						files = append(files, key)
					}
				}
				return files
			}(),
		}).Error("Job file is missing in request")
		http.Error(w, "Job file is required111", http.StatusBadRequest)
		return
	}
	defer file.Close()

	h.logger.WithFields(logrus.Fields{
		"filename": fileHeader.Filename,
		"size":     fileHeader.Size,
	}).Info("Job file received")

	// Check file size limit (1MB)
	if fileHeader.Size > 1024*1024 {
		h.logger.WithField("size", fileHeader.Size).Error("Job file exceeds 1MB limit")
		http.Error(w, "Job file exceeds 1MB limit", http.StatusBadRequest)
		return
	}

	// Read file content
	jobFileContent, err := io.ReadAll(file)
	if err != nil {
		h.logger.WithError(err).Error("Failed to read job file content")
		http.Error(w, "Failed to read job file content", http.StatusInternalServerError)
		return
	}

	h.logger.WithField("content_length", len(jobFileContent)).Info("Job file content read successfully")

	// Check if deployment already exists
	_, _, _, err = database.GetDeployment(h.db, tagID)
	if err == nil {
		// Deployment exists
		h.logger.WithField("tag_id", tagID).Error("Deployment with this tag_id already exists")
		http.Error(w, fmt.Sprintf("A deployment with tag_id %s already exists", tagID), http.StatusConflict)
		return
	}

	// Create temporary file in /tmp location
	tmpFile := fmt.Sprintf("/tmp/nomad-job-%s.hcl", tagID)
	if err := os.WriteFile(tmpFile, jobFileContent, 0600); err != nil {
		h.logger.WithError(err).Error("Failed to write job file to tmp location")
		http.Error(w, "Failed to write job file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpFile) // Clean up temporary file

	h.logger.WithField("tmp_file", tmpFile).Info("Job file written to tmp location")

	// Validate Nomad job file using Nomad's parse API
	jobJSON, err := h.parseJobFileWithNomadAPI(string(jobFileContent), tagID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to parse job file using Nomad API")
		http.Error(w, fmt.Sprintf("Failed to parse job file: %v", err), http.StatusBadRequest)
		return
	}

	fmt.Println("Parsed job JSON:", jobJSON)

	// Store initial deployment record (without service name for job deployments)
	if err := database.InsertDeployment(h.db, tagID, "", "", "pending"); err != nil {
		h.logger.WithError(err).Error("Database error inserting deployment")
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	// Submit job to Nomad
	jobID, err := h.nomad.SubmitJobFile(jobJSON, tagID)
	if err != nil {
		h.logger.WithError(err).WithField("tag_id", tagID).Error("Nomad job submission failed")
		if updateErr := database.UpdateDeploymentStatus(h.db, tagID, "failed"); updateErr != nil {
			h.logger.WithError(updateErr).Error("Failed to update deployment status")
		}
		response := models.DeploymentResponse{
			Status:  "failed",
			TagID:   tagID,
			Message: err.Error(),
		}
		h.writeJSONResponse(w, response)
		return
	}

	// Update with job ID
	if err := database.UpdateDeploymentJobID(h.db, tagID, jobID, "running"); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"tag_id": tagID,
			"job_id": jobID,
		}).Error("Failed to update job ID in database")
		// Continue with response even if database update fails
	}

	response := models.DeploymentResponse{
		Status: "running",
		TagID:  tagID,
		JobID:  jobID,
	}

	h.writeJSONResponse(w, response)
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	var req models.DeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Error("Failed to decode request JSON")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.logger.WithField("request", req).Info("Deployment request received")
	// Check if tagID is empty or doesn't exist
	tagID := req.TagID
	if tagID == "" {
		h.logger.Error("Tag ID is missing in request")
		http.Error(w, "Tag ID is required", http.StatusBadRequest)
		return
	}

	// Check if deployment already exists
	_, _, _, err := database.GetDeployment(h.db, tagID)
	if err == nil {
		// Deployment exists
		h.logger.WithField("tag_id", tagID).Error("Deployment with this tag_id already exists")
		http.Error(w, fmt.Sprintf("A deployment with tag_id %s already exists", tagID), http.StatusConflict)
		return
	}

	// Store initial deployment record
	if err := database.InsertDeployment(h.db, tagID, req.ServiceName, "", "pending"); err != nil {
		h.logger.WithError(err).Error("Database error inserting deployment")
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	// Trigger Nomad deployment
	jobID, err := h.nomad.TriggerDeployment(req.ServiceName, tagID)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"service": req.ServiceName,
			"tag_id":  tagID,
		}).Error("Nomad deployment failed")
		if updateErr := database.UpdateDeploymentStatus(h.db, tagID, "failed"); updateErr != nil {
			h.logger.WithError(updateErr).Error("Failed to update deployment status")
		}
		response := models.DeploymentResponse{
			Status:  "failed",
			TagID:   tagID,
			Message: err.Error(),
		}
		h.writeJSONResponse(w, response)
		return
	}

	// Update with job ID
	if err := database.UpdateDeploymentJobID(h.db, tagID, jobID, "running"); err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"tag_id": tagID,
			"job_id": jobID,
		}).Error("Failed to update job ID in database")
		// Continue with response even if database update fails
	}

	response := models.DeploymentResponse{
		Status: "running",
		TagID:  tagID,
		JobID:  jobID,
	}

	h.writeJSONResponse(w, response)
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tagID := vars["tag_id"]

	_, jobID, status, err := database.GetDeployment(h.db, tagID)
	if err != nil {
		h.logger.WithError(err).WithField("tag_id", tagID).Error("Failed to get deployment")
		http.Error(w, fmt.Sprintf("Deployment not found: %v", err), http.StatusNotFound)
		return
	}

	// Check current status from Nomad if job is running
	if status == "running" && jobID != "" {
		nomadStatus, err := h.nomad.GetJobStatus(jobID)
		if err == nil && nomadStatus != status {
			if updateErr := database.UpdateDeploymentStatus(h.db, tagID, nomadStatus); updateErr != nil {
				h.logger.WithError(updateErr).Error("Failed to update deployment status")
			}
			status = nomadStatus
		} else if err != nil {
			h.logger.WithError(err).Error("Failed to get job status from Nomad")
		}
	}

	response := models.StatusResponse{
		Status: status,
		TagID:  tagID,
		JobID:  jobID,
	}

	h.writeJSONResponse(w, response)
}

func (h *Handler) StreamDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tagID := vars["tag_id"]

	h.logger.WithField("tag_id", tagID).Info("Stream deployment status request received")

	// Look up deployment in DB
	_, jobID, status, err := database.GetDeployment(h.db, tagID)
	if err != nil {
		h.logger.WithError(err).WithField("tag_id", tagID).Error("Deployment not found")
		http.Error(w, fmt.Sprintf("Deployment not found: %v", err), http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// If already in terminal state, send single event and close
	if status == "completed" || status == "failed" {
		h.sendSSE(w, flusher, models.StreamEvent{
			Status:    status,
			Message:   fmt.Sprintf("Deployment already %s", status),
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	// If no eval ID available, can't track
	if jobID == "" {
		h.sendSSE(w, flusher, models.StreamEvent{
			Status:    "error",
			Message:   "No evaluation ID available for tracking",
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	// Extend write deadline for long-lived SSE connection
	rc := http.NewResponseController(w)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Maximum stream duration of 10 minutes
	timeout := time.NewTimer(10 * time.Minute)
	defer timeout.Stop()

	var deploymentID string
	var lastStatus string

	// Send initial event
	h.sendSSE(w, flusher, models.StreamEvent{
		Status:    "tracking",
		Message:   "Connected to deployment stream",
		EvalID:    jobID,
		Timestamp: time.Now().Format(time.RFC3339),
	})

	for {
		select {
		case <-r.Context().Done():
			h.logger.WithField("tag_id", tagID).Info("Client disconnected from stream")
			return
		case <-timeout.C:
			h.sendSSE(w, flusher, models.StreamEvent{
				Status:    "timeout",
				Message:   "Stream timeout reached, deployment may still be in progress. Use /status endpoint to check.",
				Timestamp: time.Now().Format(time.RFC3339),
			})
			return
		case <-ticker.C:
			// Extend write deadline for this tick
			if err := rc.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
				h.logger.WithError(err).Warn("Failed to set write deadline")
			}

			// If we have a deployment ID, track the deployment directly
			if deploymentID != "" {
				deploy, err := h.nomad.GetDeploymentStatus(deploymentID)
				if err != nil {
					h.sendSSE(w, flusher, models.StreamEvent{
						Status:       "error",
						Message:      fmt.Sprintf("Failed to get deployment status: %v", err),
						DeploymentID: deploymentID,
						Timestamp:    time.Now().Format(time.RFC3339),
					})
					continue
				}

				// Calculate totals across task groups
				var totalHealthy, totalUnhealthy, totalDesired int
				for _, tg := range deploy.TaskGroups {
					totalHealthy += tg.HealthyAllocs
					totalUnhealthy += tg.UnhealthyAllocs
					totalDesired += tg.DesiredTotal
				}

				currentStatus := mapDeploymentStatus(deploy.Status)
				message := deploy.StatusDescription
				if message == "" {
					message = fmt.Sprintf("Deployment %s: %d/%d healthy", deploy.Status, totalHealthy, totalDesired)
				}

				event := models.StreamEvent{
					Status:       currentStatus,
					Message:      message,
					DeploymentID: deploymentID,
					Healthy:      totalHealthy,
					Unhealthy:    totalUnhealthy,
					Total:        totalDesired,
					Timestamp:    time.Now().Format(time.RFC3339),
				}

				h.sendSSE(w, flusher, event)

				// Update DB if status changed
				if currentStatus != lastStatus {
					lastStatus = currentStatus
					if updateErr := database.UpdateDeploymentStatus(h.db, tagID, currentStatus); updateErr != nil {
						h.logger.WithError(updateErr).Error("Failed to update deployment status in DB")
					}
				}

				// If terminal, close stream
				if currentStatus == "completed" || currentStatus == "failed" {
					return
				}
				continue
			}

			// Check evaluation status to find deployment ID
			eval, err := h.nomad.GetEvaluation(jobID)
			if err != nil {
				h.sendSSE(w, flusher, models.StreamEvent{
					Status:    "error",
					Message:   fmt.Sprintf("Failed to get evaluation status: %v", err),
					EvalID:    jobID,
					Timestamp: time.Now().Format(time.RFC3339),
				})
				continue
			}

			// Check if eval has created a deployment
			if eval.DeploymentID != "" {
				deploymentID = eval.DeploymentID
				h.sendSSE(w, flusher, models.StreamEvent{
					Status:       "deploying",
					Message:      "Deployment created, tracking progress",
					EvalID:       jobID,
					DeploymentID: deploymentID,
					Timestamp:    time.Now().Format(time.RFC3339),
				})
				lastStatus = "deploying"
				continue
			}

			// Handle evaluation terminal states
			switch eval.Status {
			case "complete":
				// Eval complete but no deployment - simple job update without update stanza
				h.sendSSE(w, flusher, models.StreamEvent{
					Status:    "completed",
					Message:   "Evaluation complete, job scheduled successfully",
					EvalID:    jobID,
					Timestamp: time.Now().Format(time.RFC3339),
				})
				if updateErr := database.UpdateDeploymentStatus(h.db, tagID, "completed"); updateErr != nil {
					h.logger.WithError(updateErr).Error("Failed to update deployment status in DB")
				}
				return
			case "failed":
				msg := eval.StatusDescription
				if msg == "" {
					msg = "Evaluation failed"
				}
				if len(eval.FailedTGAllocs) > 0 {
					msg += fmt.Sprintf(" (failed task group allocations: %v)", eval.FailedTGAllocs)
				}
				h.sendSSE(w, flusher, models.StreamEvent{
					Status:    "failed",
					Message:   msg,
					EvalID:    jobID,
					Timestamp: time.Now().Format(time.RFC3339),
				})
				if updateErr := database.UpdateDeploymentStatus(h.db, tagID, "failed"); updateErr != nil {
					h.logger.WithError(updateErr).Error("Failed to update deployment status in DB")
				}
				return
			case "canceled":
				h.sendSSE(w, flusher, models.StreamEvent{
					Status:    "failed",
					Message:   "Evaluation was canceled",
					EvalID:    jobID,
					Timestamp: time.Now().Format(time.RFC3339),
				})
				if updateErr := database.UpdateDeploymentStatus(h.db, tagID, "failed"); updateErr != nil {
					h.logger.WithError(updateErr).Error("Failed to update deployment status in DB")
				}
				return
			case "blocked":
				if eval.Status != lastStatus {
					h.sendSSE(w, flusher, models.StreamEvent{
						Status:    "blocked",
						Message:   fmt.Sprintf("Evaluation blocked: %s", eval.StatusDescription),
						EvalID:    jobID,
						Timestamp: time.Now().Format(time.RFC3339),
					})
					lastStatus = eval.Status
				}
			default:
				// pending or other
				if eval.Status != lastStatus {
					h.sendSSE(w, flusher, models.StreamEvent{
						Status:    "evaluating",
						Message:   fmt.Sprintf("Evaluation %s", eval.Status),
						EvalID:    jobID,
						Timestamp: time.Now().Format(time.RFC3339),
					})
					lastStatus = eval.Status
				}
			}
		}
	}
}

// sendSSE writes a Server-Sent Event to the response
func (h *Handler) sendSSE(w http.ResponseWriter, flusher http.Flusher, event models.StreamEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.WithError(err).Error("Failed to marshal SSE event")
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// mapDeploymentStatus maps Nomad deployment status to internal status
func mapDeploymentStatus(nomadStatus string) string {
	switch nomadStatus {
	case "successful":
		return "completed"
	case "failed":
		return "failed"
	case "cancelled":
		return "failed"
	default:
		return "running"
	}
}

func (h *Handler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["service_name"]

	h.logger.WithField("service_name", serviceName).Info("Delete deployment request received")

	if serviceName == "" {
		http.Error(w, "Service name is required", http.StatusBadRequest)
		return
	}

	stopResp, err := h.nomad.StopJob(serviceName, false)
	if err != nil {
		h.logger.WithError(err).WithField("service_name", serviceName).Error("Failed to stop job in Nomad")
		http.Error(w, fmt.Sprintf("Failed to stop job: %v", err), http.StatusInternalServerError)
		return
	}

	response := models.DeleteResponse{
		Status:      "stopped",
		ServiceName: serviceName,
		Message:     fmt.Sprintf("Job '%s' has been stopped successfully", serviceName),
		EvalID:      stopResp.EvalID,
	}

	h.writeJSONResponse(w, response)
}

// parseJobFileWithNomadAPI converts HCL job content to JSON using Nomad's parse API
func (h *Handler) parseJobFileWithNomadAPI(jobHCL, tagID string) (map[string]interface{}, error) {
	h.logger.WithField("tag_id", tagID).Info("Parsing job file using Nomad API")

	// Prepare the request payload
	parseRequest := map[string]interface{}{
		"JobHCL":       jobHCL,
		"Variables":    "",
		"Canonicalize": true,
	}

	// Convert to JSON
	payloadBytes, err := json.Marshal(parseRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parse request: %v", err)
	}

	// Create request to Nomad parse API
	url := fmt.Sprintf("%s/v1/jobs/parse?namespace=*", h.config.NomadURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create parse request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Accept", "*/*")
	if h.config.NomadToken != "" {
		req.Header.Set("X-Nomad-Token", h.config.NomadToken)
	}

	// Create HTTP client with TLS configuration
	client := h.nomad.GetHTTPClient()

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Nomad parse API: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		h.logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"error_body":  bodyStr,
		}).Error("Nomad parse API returned non-200 status")
		return nil, fmt.Errorf("nomad parse API returned status %d: %s", resp.StatusCode, bodyStr)
	}

	// Parse the response
	var jobSpec map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jobSpec); err != nil {
		return nil, fmt.Errorf("failed to decode Nomad parse response: %v", err)
	}

	h.logger.WithField("tag_id", tagID).Info("Job file parsed successfully using Nomad API")

	// Wrap in the expected format for job submission
	return map[string]interface{}{
		"Job": jobSpec,
	}, nil
}
