package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"bastion-deployment/internal/config"
	"bastion-deployment/internal/database"
	"bastion-deployment/internal/models"
	"bastion-deployment/internal/nomad"

	"github.com/gorilla/mux"
)

type Handler struct {
	db     *sql.DB
	config *config.Config
	nomad  *nomad.Client
}

func NewHandler(db *sql.DB, cfg *config.Config, nomadClient *nomad.Client) *Handler {
	return &Handler{
		db:     db,
		config: cfg,
		nomad:  nomadClient,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	var req models.DeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate secret key
	if req.SecretKey != h.config.ValidSecret {
		http.Error(w, "Invalid secret key", http.StatusUnauthorized)
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
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Trigger Nomad deployment
	jobID, err := h.nomad.TriggerDeployment(req.ServiceName, tagID)
	if err != nil {
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
		// Log error but don't fail the request
		// In a production system, you'd want proper logging here
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
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	// Check current status from Nomad if job is running
	if status == "running" && jobID != "" {
		nomadStatus, err := h.nomad.GetJobStatus(jobID)
		if err == nil && nomadStatus != status {
			database.UpdateDeploymentStatus(h.db, tagID, nomadStatus)
			status = nomadStatus
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
