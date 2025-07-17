package server

import (
	"database/sql"
	"log"
	"net/http"

	"bastion-deployment/internal/config"
	"bastion-deployment/internal/handlers"
	"bastion-deployment/internal/nomad"

	"github.com/gorilla/mux"
)

type Server struct {
	config  *config.Config
	db      *sql.DB
	handler *handlers.Handler
	router  *mux.Router
}

func NewServer(cfg *config.Config, db *sql.DB) *Server {
	nomadClient := nomad.NewClient(cfg.NomadURL)
	handler := handlers.NewHandler(db, cfg, nomadClient)

	s := &Server{
		config:  cfg,
		db:      db,
		handler: handler,
		router:  mux.NewRouter(),
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Health endpoint
	s.router.HandleFunc("/health", s.handler.Health).Methods("GET")

	// Deploy endpoint
	s.router.HandleFunc("/deploy", s.handler.Deploy).Methods("POST")

	// Status endpoint
	s.router.HandleFunc("/status/{tag_id}", s.handler.Status).Methods("GET")
}

func (s *Server) Start() error {
	log.Printf("Server starting on port %s", s.config.Port)
	return http.ListenAndServe(":"+s.config.Port, s.router)
}
