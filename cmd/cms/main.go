package main

import (
	"cms/internal/config"
	"cms/internal/core"
	"cms/internal/handlers"
	"cms/internal/storage"

	"embed"
	"log"
	"runtime/debug"

	"github.com/valyala/fasthttp"
)

// Embed assets relative to this file
//
//go:embed all:assets
var assets embed.FS

func main() {
	// Optimize GC
	debug.SetGCPercent(100)
	// Consider making this configurable or based on available memory
	// debug.SetMemoryLimit(512 * 1024 * 1024) // 512MB

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

	// Start the server
	server := &fasthttp.Server{
		Handler: router.Handler,
		Name:    "cms",
		// Fasthttp optimizations
		Concurrency:        cfg.Concurrency,
		ReadBufferSize:     4096, // Consider making configurable
		WriteBufferSize:    4096, // Consider making configurable
		ReadTimeout:        cfg.ReadTimeout,
		WriteTimeout:       cfg.WriteTimeout,
		MaxRequestBodySize: 10 * 1024 * 1024, // 10MB - Consider making configurable
	}

	log.Printf("Server starting on %s", cfg.Address)
	if err := server.ListenAndServe(cfg.Address); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
