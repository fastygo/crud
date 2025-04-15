package handlers

import (
	"log"
	"strings"

	"cms/internal/config"

	session "github.com/fasthttp/session/v2"
	"github.com/valyala/fasthttp"
)

// AuthMiddleware checks if the user is authenticated via session.
// If not authenticated, redirects to the login page.
// Allows access to public paths.
func AuthMiddleware(next fasthttp.RequestHandler, sess *session.Session, cfg *config.Config) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		// Define public paths that don't require authentication
		publicPaths := []string{
			"/login",
			"/", // Assuming the index page is public
			// Add other public paths like /static if needed (though static files might be handled differently)
		}

		// Check if the current path is public
		isPublic := false
		for _, publicPath := range publicPaths {
			if path == publicPath {
				isPublic = true
				break
			}
		}
		// Allow static files (if they have a common prefix)
		if strings.HasPrefix(path, "/static/") {
			isPublic = true
		}

		// If the path is public, allow access without checking session
		if isPublic {
			next(ctx)
			return
		}

		// Get session store for the current request
		store, err := sess.Get(ctx)
		if err != nil {
			log.Printf("AuthMiddleware: Error getting session: %v", err)
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
			return
		}

		// Check if the user is authenticated
		authValue := store.Get("authenticated")
		authenticated, ok := authValue.(bool)

		// Redirect to login if not authenticated
		if !ok || !authenticated {
			log.Printf("AuthMiddleware: Unauthenticated access attempt to %s", path)
			// Save the originally requested URL to redirect back after login
			store.Set("redirect_url", path)
			if err := sess.Save(ctx, store); err != nil {
				log.Printf("AuthMiddleware: Error saving redirect URL to session: %v", err)
				// Continue to redirect anyway, but log the error
			}
			ctx.Redirect("/login", fasthttp.StatusSeeOther)
			return
		}

		// If authenticated, proceed to the requested handler
		next(ctx)
	}
}
