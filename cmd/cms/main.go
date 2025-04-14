package main

import (
	"cms/internal/config"
	"cms/internal/core"
	"cms/internal/handlers"
	"cms/internal/storage"

	"embed"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"

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

	// Initialize ephemeral BoltDB (path is relative to embed FS root)
	db, err := storage.NewEphemeralBoltDB(assets, "assets/db/initial.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize router
	router := core.NewRouter()

	// Static files handler (path is relative to embed FS root)
	// staticHandler := handlers.NewStaticHandler(assets, "assets/static")
	// router.GET("/static/*filepath", staticHandler.Handle)

	// API handlers for CRUD operations
	crudHandler := handlers.NewCRUDHandler(db)
	router.GET("/api/content", crudHandler.List)
	router.GET("/api/content/{id}", crudHandler.Get)
	router.POST("/api/content", crudHandler.Create)
	router.PUT("/api/content/{id}", crudHandler.Update)
	router.DELETE("/api/content/{id}", crudHandler.Delete)

	// HTML page handlers using templates
	pageHandler := handlers.NewPageHandler(db)
	router.GET("/", pageHandler.Index)
	router.GET("/content", pageHandler.List)
	router.GET("/content/new", pageHandler.New)
	router.GET("/content/{id}", pageHandler.View)
	router.GET("/content/{id}/edit", pageHandler.Edit)
	router.GET("/admin", pageHandler.Admin)
	router.GET("/settings", pageHandler.Settings)
	router.GET("/404", pageHandler.NotFound)
	router.NotFound = pageHandler.NotFound

	// Import/Export handlers
	router.POST("/api/export", crudHandler.ExportJSON)
	router.POST("/api/import", crudHandler.ImportJSON)

	// Start time tracking
	startTime := time.Now()
	requestCount := 0
	var requestLock sync.Mutex

	// Create a simple middleware to track request timing
	trackTiming := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			next(ctx)
			duration := time.Since(start)

			// Log if request takes longer than threshold
			if duration > 20*time.Millisecond {
				log.Printf("Slow request: %s %s - %v",
					string(ctx.Method()), string(ctx.Path()), duration)
			}

			requestLock.Lock()
			requestCount++
			requestLock.Unlock()

			// Add response time header (helps with debugging)
			ctx.Response.Header.Set("X-Response-Time",
				fmt.Sprintf("%d ms", duration.Milliseconds()))
		}
	}

	// Wrap the main router handler with the timing middleware
	serverHandler := trackTiming(router.Handler)

	// Start the server
	server := &fasthttp.Server{
		Handler: serverHandler,
		Name:    "cms",
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

	if err := server.ListenAndServe(cfg.Address); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
