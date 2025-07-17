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
	// Health endpoint (unprotected)
	s.router.HandleFunc("/health", s.handler.Health).Methods("GET")

	// Protected routes with secret key validation
	protectedRouter := s.router.PathPrefix("").Subrouter()
	protectedRouter.Use(s.authMiddleware)

	// Deploy endpoint
	protectedRouter.HandleFunc("/deploy", s.handler.Deploy).Methods("POST")

	// Status endpoint
	protectedRouter.HandleFunc("/status/{tag_id}", s.handler.Status).Methods("GET")
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get secret key from header
		secretKey := r.Header.Get("X-Secret-Key")

		// Validate secret key
		if secretKey != s.config.ValidSecret {
			http.Error(w, "Invalid secret key", http.StatusUnauthorized)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	log.Printf("Server starting on port %s", s.config.Port)
	return http.ListenAndServe(":"+s.config.Port, s.router)
}
