package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bastion-deployment/internal/config"
	"bastion-deployment/internal/database"
	"bastion-deployment/internal/models"
	"bastion-deployment/internal/nomad"

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
				h.logger.WithError(updateErr).WithFields(logrus.Fields{
					"tag_id": tagID,
					"status": nomadStatus,
				}).Error("Failed to update deployment status")
			}
			status = nomadStatus
		} else if err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{
				"job_id": jobID,
				"tag_id": tagID,
			}).Error("Failed to get job status from Nomad")
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
