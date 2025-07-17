package server

import (
	"database/sql"
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
	logger  *logrus.Logger
}

func NewServer(cfg *config.Config, db *sql.DB) *Server {
	// Create a shared logger
	logger := logrus.New()

	// Configure logger based on environment
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Configure log format
	logFormat := strings.ToLower(os.Getenv("LOG_FORMAT"))
	if logFormat == "text" {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	} else {
		// Default to JSON format
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	}

	// Create Nomad client with the shared logger
	nomadClient := nomad.NewClient(cfg.NomadURL, cfg.SkipTLSVerify)
	nomadClient.SetLogLevel(logger.Level)
	nomadClient.SetLogFormatter(logger.Formatter)

	handler := handlers.NewHandler(db, cfg, nomadClient)

	s := &Server{
		config:  cfg,
		db:      db,
		handler: handler,
		router:  mux.NewRouter(),
		logger:  logger,
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
		s.logger.WithFields(logrus.Fields{
			"secret_key": secretKey,
			"path":       r.URL.Path,
			"method":     r.Method,
		}).Debug("Authenticating request")

		// Validate secret key
		if secretKey != s.config.ValidSecret {
			s.logger.WithFields(logrus.Fields{
				"path":   r.URL.Path,
				"method": r.Method,
				"ip":     r.RemoteAddr,
			}).Warn("Invalid secret key provided")
			http.Error(w, "Invalid secret key", http.StatusUnauthorized)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	s.logger.WithField("port", s.config.Port).Info("Server starting")
	return http.ListenAndServe(":"+s.config.Port, s.router)
}
