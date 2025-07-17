package server

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	"bastion-deployment/internal/config"
	"bastion-deployment/internal/handlers"
	"bastion-deployment/internal/nomad"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	config  *config.Config
	db      *sql.DB
	handler *handlers.Handler
	router  *mux.Router
}

func NewServer(cfg *config.Config, db *sql.DB) *Server {
	nomadClient := nomad.NewClient(cfg.NomadURL)

	// Configure logger based on environment
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case "debug":
		nomadClient.SetLogLevel(logrus.DebugLevel)
	case "info":
		nomadClient.SetLogLevel(logrus.InfoLevel)
	case "warn", "warning":
		nomadClient.SetLogLevel(logrus.WarnLevel)
	case "error":
		nomadClient.SetLogLevel(logrus.ErrorLevel)
	default:
		nomadClient.SetLogLevel(logrus.InfoLevel)
	}

	// Configure log format
	logFormat := strings.ToLower(os.Getenv("LOG_FORMAT"))
	if logFormat == "text" {
		nomadClient.SetLogFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	} else {
		// Default to JSON format
		nomadClient.SetLogFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	}

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
