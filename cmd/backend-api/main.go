package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/handlers"
	authMiddleware "github.com/Perseverance/the-academy-sync-claude/internal/pkg/api/middleware"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/config"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/health"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/retry"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/services"
)

// performStartupHealthChecks validates critical dependencies and fails fast if any are unavailable
// This function implements the US046 fail-fast mechanism for critical startup dependencies
func performStartupHealthChecks(cfg *config.Config, log *logger.Logger) error {
	log.Info("Starting dependency health checks")
	
	// Create health checker
	healthChecker := health.NewHealthChecker(log)
	
	// Create context with timeout for all health checks
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Validate critical dependencies
	if cfg.DatabaseURL == "" {
		log.Critical("Critical dependency validation failed: DATABASE_URL not configured")
		return fmt.Errorf("DATABASE_URL is required but not configured")
	}
	
	// Critical dependency: Database connection with retry logic
	err := retry.WithExponentialBackoff(ctx, retry.CriticalConfig(), log, "database_health_check", func() error {
		result := healthChecker.CheckDatabaseConnection(ctx, cfg.DatabaseURL)
		if !result.IsHealthy() {
			return fmt.Errorf("database health check failed: %w", result.Error)
		}
		return nil
	})
	
	if err != nil {
		log.Critical("Critical dependency failed: Database connection unavailable after retries", 
			"error", err.Error())
		return fmt.Errorf("database dependency check failed: %w", err)
	}
	
	log.Info("All critical dependency health checks passed successfully")
	return nil
}

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

	// Dependency Health Check - US046 Fail Fast Mechanism
	// Validate critical dependencies before proceeding with initialization
	if err := performStartupHealthChecks(cfg, log); err != nil {
		log.Critical("Startup dependency health checks failed - application cannot continue", 
			"error", err.Error())
		os.Exit(2) // Exit code 2 indicates dependency failure
	}

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
	
	// Construct OAuth redirect URLs using configurable base URL
	googleRedirectURL := fmt.Sprintf("%s/api/auth/google/callback", cfg.BaseURL)
	stravaRedirectURL := fmt.Sprintf("%s/api/connections/strava/callback", cfg.BaseURL)
	oauthService := auth.NewOAuthService(
		cfg.GoogleClientID, 
		cfg.GoogleClientSecret, 
		googleRedirectURL, 
		cfg.StravaClientID, 
		cfg.StravaClientSecret, 
		stravaRedirectURL,
	)

	// Initialize repositories
	userRepository := database.NewUserRepository(db, encryptionService)
	sessionRepository := database.NewSessionRepository(db)

	// Initialize services
	sheetsService := services.NewSheetsService(userRepository, log.WithContext("component", "sheets_service"))
	configService := services.NewConfigService(userRepository, sheetsService, log.WithContext("component", "config_service"))

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

	stravaHandler := handlers.NewStravaHandler(
		oauthService,
		userRepository,
		cfg.FrontendURL,
		isDevelopment,
		log.WithContext("component", "strava_handler"),
	)

	configHandler := handlers.NewConfigHandler(
		configService,
		log.WithContext("component", "config_handler"),
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

	// Connection routes - mixed public and protected
	r.Route("/api/connections", func(r chi.Router) {
		// Public OAuth callback (Strava redirects here directly)
		r.Get("/strava/callback", stravaHandler.StravaCallback) // Handle Strava OAuth callback (public)
		
		// Protected Strava endpoints (require authentication)
		r.Group(func(r chi.Router) {
			r.Use(authMW.RequireAuth)
			r.Get("/strava", stravaHandler.StravaAuthURL)      // Get Strava OAuth URL
			r.Delete("/strava", stravaHandler.DisconnectStrava) // Disconnect Strava account
		})
	})

	// Protected API routes (authentication required)
	r.Route("/api", func(r chi.Router) {
		r.Use(authMW.RequireAuth)
		
		// User routes
		r.Route("/users", func(r chi.Router) {
			r.Get("/me", authHandler.GetCurrentUser) // Duplicate for convenience
		})

		// Configuration routes
		r.Route("/config", func(r chi.Router) {
			r.Post("/spreadsheet", configHandler.SetSpreadsheet)      // Set spreadsheet URL
			r.Delete("/spreadsheet", configHandler.ClearSpreadsheet)  // Clear spreadsheet configuration
		})

		// Future protected endpoints will go here
		// r.Route("/automation", func(r chi.Router) { ... })
		// r.Route("/notifications", func(r chi.Router) { ... })
	})

	log.Info("Backend API server starting", 
		"port", cfg.Port,
		"base_url", cfg.BaseURL,
		"google_oauth_redirect_url", googleRedirectURL,
		"strava_oauth_redirect_url", stravaRedirectURL)
	
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Critical("Server failed to start", "error", err)
		os.Exit(1)
	}
}