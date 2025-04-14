package core

import (
	"strings"

	"github.com/fasthttp/router"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// Router wraps fasthttp/router and manages buffer pools.
type Router struct {
	router   *router.Router
	pool     *bytebufferpool.Pool
	NotFound fasthttp.RequestHandler
}

// NewRouter creates a new router instance.
func NewRouter() *Router {
	r := router.New()
	defaultNotFound := r.NotFound

	// Configure the router for better performance
	r.RedirectTrailingSlash = false  // Avoid redirects for trailing slashes
	r.RedirectFixedPath = false      // Avoid path auto-fixing redirects
	r.HandleMethodNotAllowed = false // Skip method not allowed checks for speed
	r.HandleOPTIONS = false          // Skip automatic OPTIONS handling

	return &Router{
		router:   r,
		pool:     &bytebufferpool.Pool{},
		NotFound: defaultNotFound,
	}
}

// wrapHandler enhances a fasthttp.RequestHandler to use a pooled buffer.
func wrapHandler(h fasthttp.RequestHandler, pool *bytebufferpool.Pool) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// Only get a buffer if the handler is likely to need it
		// (content handlers, not static files)
		if needsBuffer(ctx) {
			buf := pool.Get()
			defer pool.Put(buf)
			ctx.SetUserValue("buffer", buf)
		}
		h(ctx)
	}
}

// Helper to determine if a request needs a buffer
func needsBuffer(ctx *fasthttp.RequestCtx) bool {
	// Only allocate buffers for API and content creation/editing paths
	path := string(ctx.Path())
	return strings.HasPrefix(path, "/api/") ||
		strings.Contains(path, "/edit") ||
		strings.Contains(path, "/new")
}

// GET registers a GET handler.
func (r *Router) GET(path string, handler fasthttp.RequestHandler) {
	r.router.GET(path, wrapHandler(handler, r.pool))
}

// POST registers a POST handler.
func (r *Router) POST(path string, handler fasthttp.RequestHandler) {
	r.router.POST(path, wrapHandler(handler, r.pool))
}

// PUT registers a PUT handler.
func (r *Router) PUT(path string, handler fasthttp.RequestHandler) {
	r.router.PUT(path, wrapHandler(handler, r.pool))
}

// DELETE registers a DELETE handler.
func (r *Router) DELETE(path string, handler fasthttp.RequestHandler) {
	r.router.DELETE(path, wrapHandler(handler, r.pool))
}

// Handler returns the underlying fasthttp handler.
func (r *Router) Handler(ctx *fasthttp.RequestCtx) {
	if r.NotFound != nil {
		r.router.NotFound = r.NotFound
	}
	r.router.Handler(ctx)
}
