package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/handlers"
	authMiddleware "github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

func main() {
	// Load configuration using hybrid loading strategy
	cfg, err := config.Load()
	if err != nil {
		// Use fallback logging before structured logger is available
		fmt.Printf("ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	log := logger.New("backend-api")

	log.Info("Backend API starting", 
		"environment", cfg.Environment, 
		"port", cfg.Port,
		"log_level", cfg.LogLevel)
	log.Info("Configuration status", 
		"database_configured", cfg.DatabaseURL != "",
		"google_oauth_configured", cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "")

	// Initialize database connection
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Critical("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Critical("Failed to ping database", "error", err)
		os.Exit(1)
	}
	log.Info("Database connection established successfully")

	// Initialize services
	jwtService := auth.NewJWTService(cfg.JWTSecret)
	encryptionService := auth.NewEncryptionService(cfg.EncryptionSecret) // Use separate encryption secret
	
	// Construct OAuth redirect URL using configurable base URL
	redirectURL := fmt.Sprintf("%s/api/auth/google/callback", cfg.BaseURL)
	oauthService := auth.NewOAuthService(cfg.GoogleClientID, cfg.GoogleClientSecret, redirectURL)

	// Initialize repositories
	userRepository := database.NewUserRepository(db, encryptionService)
	sessionRepository := database.NewSessionRepository(db)

	// Initialize middleware
	authMW := authMiddleware.NewAuthMiddleware(jwtService, sessionRepository, oauthService, userRepository, log.WithContext("component", "auth_middleware"))

	// Initialize handlers
	// Determine if running in development mode
	isDevelopment := cfg.Environment == "local" || cfg.Environment == "development" || cfg.Environment == "dev"
	
	authHandler := handlers.NewAuthHandler(
		oauthService,
		jwtService,
		userRepository,
		sessionRepository,
		cfg.FrontendURL,
		isDevelopment,
		log.WithContext("component", "auth_handler"),
	)

	// Create router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authMiddleware.CORS(cfg.FrontendURL)) // Enable CORS for frontend communication

	// Public routes (no authentication required)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Academy Sync Backend API is running in %s environment!", cfg.Environment)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "healthy", "environment": "%s", "service": "backend-api"}`, cfg.Environment)
	})

	// Authentication routes (public)
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/google", authHandler.GoogleAuthURL)           // Get Google OAuth URL
		r.Get("/google/callback", authHandler.GoogleCallback) // Handle OAuth callback
		r.Post("/refresh", authHandler.RefreshToken)          // Refresh JWT token
		
		// Protected auth routes
		r.Group(func(r chi.Router) {
			r.Use(authMW.RequireAuth)
			r.Get("/me", authHandler.GetCurrentUser) // Get current user info
			r.Post("/logout", authHandler.Logout)    // Logout user
		})
	})

	// Protected API routes (authentication required)
	r.Route("/api", func(r chi.Router) {
		r.Use(authMW.RequireAuth)
		
		// User routes
		r.Route("/users", func(r chi.Router) {
			r.Get("/me", authHandler.GetCurrentUser) // Duplicate for convenience
		})

		// Future protected endpoints will go here
		// r.Route("/automation", func(r chi.Router) { ... })
		// r.Route("/notifications", func(r chi.Router) { ... })
	})

	log.Info("Backend API server starting", 
		"port", cfg.Port,
		"base_url", cfg.BaseURL,
		"google_oauth_redirect_url", redirectURL)
	
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Critical("Server failed to start", "error", err)
		os.Exit(1)
	}
}