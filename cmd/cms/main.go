package main

import (
	"cms/internal/config"
	"cms/internal/core"
	"cms/internal/handlers"
	"cms/internal/models"
	"cms/internal/storage"

	"embed"
	"encoding/gob"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"

	// Session management
	session "github.com/fasthttp/session/v2"
	"github.com/fasthttp/session/v2/providers/memory"

	"github.com/valyala/fasthttp"
)

// Embed assets relative to this file
//
//go:embed all:assets
var assets embed.FS

func main() {
	// Set a stricter GC percent (less memory usage, more frequent GC)
	debug.SetGCPercent(20) // Change from 100 to 20

	// Set memory limit more aggressively
	if os.Getenv("LOW_MEMORY") == "true" {
		// For low memory environments (512MB total server RAM)
		debug.SetMemoryLimit(256 * 1024 * 1024) // 256MB
	} else {
		// For standard environments
		debug.SetMemoryLimit(512 * 1024 * 1024) // 512MB
	}

	// Initialize configuration
	cfg := config.Load()

	// Register custom types for session encoding (gob)
	gob.Register(models.Content{})
	gob.Register(map[string]models.Content{})

	// Initialize session management
	// 1. Create provider
	provider, err := memory.New(memory.Config{})
	if err != nil {
		log.Fatalf("Failed to create session provider: %v", err)
	}
	// 2. Create session config
	sessionConfig := session.NewDefaultConfig()
	sessionConfig.CookieName = "cms_sessionid" // Customize cookie name
	sessionConfig.Expiration = 24 * time.Hour  // Set session expiration (e.g., 24 hours)
	sessionConfig.Secure = false               // Set to true if using HTTPS
	// sessionConfig.Encoder = fasthttpgob.Encoder // Explicitly use gob
	// sessionConfig.Decoder = fasthttpgob.Decoder // Explicitly use gob
	// 3. Create session manager
	sess := session.New(sessionConfig)
	// 4. Set provider for the session manager
	if err = sess.SetProvider(provider); err != nil {
		log.Fatalf("Failed to set session provider: %v", err)
	}

	// Initialize Initial Data Reader
	initialDataReader, errDb := storage.NewInitialDataReader(assets, "assets/db/initial.db")
	if errDb != nil {
		log.Fatalf("Failed to initialize initial data reader: %v", errDb)
	}
	// Load initial data once at the start
	initialContent, errLoad := initialDataReader.LoadInitialContent()
	if errLoad != nil {
		log.Fatalf("Failed to load initial content: %v", errLoad)
	}
	// Close the reader immediately after loading, we don't need the BoltDB file open anymore.
	if errClose := initialDataReader.Close(); errClose != nil {
		log.Printf("Warning: Failed to close initial data reader cleanly: %v", errClose)
	}

	// Initialize router
	router := core.NewRouter()

	// Static files handler (path is relative to embed FS root)
	// staticHandler := handlers.NewStaticHandler(assets, "assets/static")
	// router.GET("/static/*filepath", staticHandler.Handle)

	// API handlers for CRUD operations (Now need initialContent for cloning)
	crudHandler := handlers.NewCRUDHandler(sess, cfg, initialContent)
	router.GET("/api/content", crudHandler.List)
	router.GET("/api/content/{id}", crudHandler.Get)
	router.POST("/api/content", crudHandler.Create)
	router.PUT("/api/content/{id}", crudHandler.Update)
	router.DELETE("/api/content/{id}", crudHandler.Delete)

	// HTML page handlers using templates (Now need initialContent for cloning)
	pageHandler := handlers.NewPageHandler(sess, cfg, initialContent)
	router.GET("/", pageHandler.Index) // Public route
	// Login/Logout routes are now implemented
	router.GET("/login", pageHandler.Login)      // Login form route
	router.POST("/login", pageHandler.PostLogin) // Login action route
	router.GET("/logout", pageHandler.Logout)    // Logout route

	// Protected routes (Middleware will handle protection)
	router.GET("/content", pageHandler.List)
	router.GET("/content/new", pageHandler.New)
	router.GET("/content/{id}", pageHandler.View)
	router.GET("/content/{id}/edit", pageHandler.Edit)
	router.GET("/admin", pageHandler.Admin)
	router.GET("/settings", pageHandler.Settings)
	router.GET("/404", pageHandler.NotFound)
	router.NotFound = pageHandler.NotFound // Keep NotFound accessible

	// Import/Export handlers (pass session manager and config)
	router.POST("/api/export", crudHandler.ExportJSON)
	router.POST("/api/import", crudHandler.ImportJSON)

	// Authentication Middleware
	// Note: static file handling might need adjustment depending on how they are served.
	// If served via a separate handler before the router, AuthMiddleware might not see /static/ paths.
	// Ensure public paths in AuthMiddleware match your routing setup.
	authMiddleware := handlers.AuthMiddleware(router.Handler, sess, cfg)

	// Start time tracking (relevant if using timing middleware)
	startTime := time.Now()
	requestCount := 0
	var requestLock sync.Mutex

	// Placeholder for timing logic if re-introduced
	_ = startTime // Use variables to avoid unused errors if timing is removed
	_ = requestCount
	_ = requestLock
	// trackTiming definition removed for now, can be added back if needed

	// Start the server
	server := &fasthttp.Server{
		// Use the authentication middleware as the main handler
		Handler: authMiddleware,
		// Handler: timedAuthHandler, // Uncomment if using timing middleware
		Name: "cms",
		// Fasthttp optimizations
		Concurrency:        cfg.Concurrency,
		ReadBufferSize:     8192, // Increased from 4096
		WriteBufferSize:    8192, // Increased from 4096
		ReadTimeout:        cfg.ReadTimeout,
		WriteTimeout:       cfg.WriteTimeout,
		MaxRequestBodySize: 10 * 1024 * 1024, // 10MB
		DisableKeepalive:   false,            // Enable keep-alive for connection reuse
		MaxConnsPerIP:      100,              // Limit connections per IP to prevent abuse
		TCPKeepalive:       true,             // Enable TCP keepalive
		TCPKeepalivePeriod: 60 * time.Second, // Keep connections alive for 60 seconds
		ReduceMemoryUsage:  true,             // Enable memory usage optimization
	}

	log.Printf("Server starting on %s", cfg.Address)

	// Log performance stats every minute
	// Commented out as requestCount tracking was tied to trackTiming middleware
	/*
		go func() {
			ticker := time.NewTicker(1 * time.Minute)
			defer ticker.Stop()

			for range ticker.C {
				requestLock.Lock()
				count := requestCount
				requestCount = 0
				requestLock.Unlock()

				uptime := time.Since(startTime)
				log.Printf("Performance: %d requests in the last minute. Uptime: %v",
					count, uptime.Round(time.Second))
			}
		}()
	*/

	if err := server.ListenAndServe(cfg.Address); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
