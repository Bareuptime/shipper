package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"shipper-deployment/internal/config"
	"shipper-deployment/internal/handlers"
	"shipper-deployment/internal/logger"
	"shipper-deployment/internal/nomad"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/sirupsen/logrus"
)

type Server struct {
	config  *config.Config
	db      *sql.DB
	handler *handlers.Handler
	router  *mux.Router
	logger  *logrus.Entry
	nrApp   *newrelic.Application
}

func NewServer(cfg *config.Config, db *sql.DB, nrApp *newrelic.Application) *Server {
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
		nrApp:   nrApp,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Add New Relic middleware if available
	if s.nrApp != nil {
		s.router.Use(s.newRelicMiddleware)
	}

	// Health endpoint (unprotected)
	s.router.HandleFunc("/health", s.handler.Health).Methods("GET")

	// Protected routes with secret key validation
	protectedRouter := s.router.PathPrefix("").Subrouter()
	protectedRouter.Use(s.authMiddleware)

	protectedRouter.HandleFunc("/deploy/job", s.handler.DeployJob).Methods("POST")

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
			http.Error(w, fmt.Sprintf("Invalid secret key - %s", s.config.ValidSecret), http.StatusUnauthorized)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// newRelicMiddleware wraps HTTP handlers with New Relic monitoring
func (s *Server) newRelicMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.nrApp == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Create New Relic transaction
		txn := s.nrApp.StartTransaction(r.Method + " " + r.URL.Path)
		defer txn.End()

		// Add request attributes
		txn.AddAttribute("http.method", r.Method)
		txn.AddAttribute("http.url", r.URL.String())
		txn.AddAttribute("user.agent", r.Header.Get("User-Agent"))

		// Wrap response writer to capture response code
		wrappedWriter := txn.SetWebResponse(w)
		r = newrelic.RequestWithTransactionContext(r, txn)

		// Continue to next handler
		next.ServeHTTP(wrappedWriter, r)
	})
}

func (s *Server) Start() error {
	s.logger.WithField("port", s.config.Port).Info("Server starting1")

	// Create server with timeouts for security
	srv := &http.Server{
		Addr:         ":" + s.config.Port,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return srv.ListenAndServe()
}
