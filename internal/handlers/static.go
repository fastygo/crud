package handlers

import (
	"embed"
	"io/fs"
	"log"
	"time"

	"github.com/valyala/fasthttp"
)

// StaticHandler serves static files from an embedded filesystem.
type StaticHandler struct {
	requestHandler fasthttp.RequestHandler // Store the generated handler
}

// NewStaticHandler creates a handler for serving static files.
func NewStaticHandler(assets embed.FS, staticRoot string) *StaticHandler {
	// Get a sub-filesystem rooted at the actual static files directory
	subFS, err := fs.Sub(assets, staticRoot) // staticRoot is "assets/static"
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem for static assets at %s: %v", staticRoot, err)
	}

	// Create a fasthttp.FS using the SUB-filesystem
	fsImpl := &fasthttp.FS{
		FS:            subFS,
		Compress:      true,
		CacheDuration: time.Hour,
		PathRewrite:   fasthttp.NewPathSlashesStripper(1),
	}

	// Create the actual request handler from the FS configuration
	handler := fsImpl.NewRequestHandler()

	return &StaticHandler{
		requestHandler: handler,
	}
}

// Handle serves the static file request by calling the pre-generated FS request handler.
// The handler uses the original ctx.Path() and applies PathRewrite internally,
// then looks for the resulting path within the subFS.
func (h *StaticHandler) Handle(ctx *fasthttp.RequestCtx) {
	h.requestHandler(ctx)
}
