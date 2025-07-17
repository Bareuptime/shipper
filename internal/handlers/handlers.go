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
	logger *logrus.Logger
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
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate service name
	if !h.config.IsValidService(req.ServiceName) {
		http.Error(w, "Invalid service name", http.StatusBadRequest)
		return
	}

	// Generate unique tag ID
	tagID := req.TagID

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
