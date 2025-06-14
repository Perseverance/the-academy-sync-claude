package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/handlers"
	authMiddleware "github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
)

func main() {
	// Load configuration using hybrid loading strategy
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Backend API starting in %s environment on port %s", cfg.Environment, cfg.Port)
	log.Printf("Database URL configured: %t", cfg.DatabaseURL != "")
	log.Printf("Google OAuth configured: %t", cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "")

	// Initialize database connection
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Printf("Database connection established successfully")

	// Initialize services
	jwtService := auth.NewJWTService(cfg.JWTSecret)
	encryptionService := auth.NewEncryptionService(cfg.JWTSecret) // Use JWT secret for encryption key
	
	// Construct OAuth redirect URL using configurable base URL
	redirectURL := fmt.Sprintf("%s/api/auth/google/callback", cfg.BaseURL)
	oauthService := auth.NewOAuthService(cfg.GoogleClientID, cfg.GoogleClientSecret, redirectURL)

	// Initialize repositories
	userRepository := database.NewUserRepository(db, encryptionService)
	sessionRepository := database.NewSessionRepository(db)

	// Initialize middleware
	authMW := authMiddleware.NewAuthMiddleware(jwtService, sessionRepository, oauthService, userRepository)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(
		oauthService,
		jwtService,
		userRepository,
		sessionRepository,
		"http://localhost:3000", // Frontend URL
	)

	// Create router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authMiddleware.CORS) // Enable CORS for frontend communication

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

	log.Printf("Backend API server starting on :%s", cfg.Port)
	log.Printf("Base URL configured: %s", cfg.BaseURL)
	log.Printf("Google OAuth redirect URL: %s", redirectURL)
	
	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}