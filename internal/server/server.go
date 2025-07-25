package server

import (
	"database/sql"
	"net/http"

	"shipper-deployment/internal/config"
	"shipper-deployment/internal/handlers"
	"shipper-deployment/internal/logger"
	"shipper-deployment/internal/nomad"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	config  *config.Config
	db      *sql.DB
	handler *handlers.Handler
	router  *mux.Router
	logger  *logrus.Entry
}

func NewServer(cfg *config.Config, db *sql.DB) *Server {
	// Initialize the global logger
	logger.Initialize()

	// Get a logger instance with the server module context
	serverLogger := logger.WithModule("server")

	// Create Nomad client with the shared logger
	nomadClient := nomad.NewClient(cfg.NomadURL, cfg.SkipTLSVerify, cfg.NomadToken)

	handler := handlers.NewHandler(db, cfg, nomadClient)

	s := &Server{
		config:  cfg,
		db:      db,
		handler: handler,
		router:  mux.NewRouter(),
		logger:  serverLogger,
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

	protectedRouter.HandleFunc("/deploy/job", s.handler.Deploy).Methods("POST")

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
