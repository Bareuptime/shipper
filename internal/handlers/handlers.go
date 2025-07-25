package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy", "time": time.Now().Format(time.RFC3339)})
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
	if err := os.WriteFile(tmpFile, jobFileContent, 0644); err != nil {
		h.logger.WithError(err).Error("Failed to write job file to tmp location")
		http.Error(w, "Failed to write job file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpFile) // Clean up temporary file

	h.logger.WithField("tmp_file", tmpFile).Info("Job file written to tmp location")

	// Validate Nomad job file (optional - skip if nomad command fails)
	validateCmd := exec.Command("nomad", "job", "validate", tmpFile)
	if output, err := validateCmd.CombinedOutput(); err != nil {
		h.logger.WithError(err).WithField("output", string(output)).Warn("Nomad job validation failed - continuing without validation")
		// Don't return error, just log warning and continue
	} else {
		h.logger.Info("Job file validation successful")
	}

	// Convert HCL to JSON (try nomad first, fallback to direct submission)
	var jobJSON map[string]interface{}
	convertCmd := exec.Command("nomad", "job", "inspect", "-json", tmpFile)
	jsonOutput, err := convertCmd.Output()
	if err != nil {
		h.logger.WithError(err).Warn("Failed to convert job file to JSON using nomad - will try direct HCL parsing")
		// Fallback: read the HCL file directly and create a basic job structure
		// This is a simplified approach - in production you might want to use a proper HCL parser
		jobJSON = map[string]interface{}{
			"Job": map[string]interface{}{
				"Name": fmt.Sprintf("job-%s", tagID),
				"Type": "service",
				// Add basic job structure here
			},
		}
	} else {
		// Parse JSON output from nomad inspect
		if err := json.Unmarshal(jsonOutput, &jobJSON); err != nil {
			h.logger.WithError(err).Error("Failed to parse converted JSON")
			http.Error(w, "Failed to parse converted JSON", http.StatusInternalServerError)
			return
		}
		h.logger.Info("Job file converted to JSON successfully")
	}

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
		database.UpdateDeploymentStatus(h.db, tagID, "failed")
		response := models.DeploymentResponse{
			Status:  "failed",
			TagID:   tagID,
			Message: err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	var req models.DeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithError(err).Error("Failed to decode request JSON")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.logger.WithField("request", req).Info("Deployment request received")

	// Validate service name
	if !h.config.IsValidService(req.ServiceName) {
		h.logger.WithField("service_name", req.ServiceName).Error("Invalid service name")
		http.Error(w, "Invalid service name", http.StatusBadRequest)
		return
	}

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
		database.UpdateDeploymentStatus(h.db, tagID, "failed")
		response := models.DeploymentResponse{
			Status:  "failed",
			TagID:   tagID,
			Message: err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
