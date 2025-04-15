//go:generate qtc -dir=../templates
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cms/internal/config"
	"cms/internal/models"
	"cms/internal/storage"

	// Import the specific generated template packages
	"cms/internal/templates/pages"
	// "cms/internal/templates/layouts" - Layouts are called by pages
	// "cms/internal/templates/components" - Components are called by layouts/pages

	session "github.com/fasthttp/session/v2"
	"github.com/valyala/fasthttp"
)

// PageHandler handles requests for HTML pages.
type PageHandler struct {
	db   *storage.EphemeralBoltDB
	sess *session.Session
	cfg  *config.Config
}

// NewPageHandler creates a new page handler.
func NewPageHandler(db *storage.EphemeralBoltDB, sess *session.Session, cfg *config.Config) *PageHandler {
	return &PageHandler{
		db:   db,
		sess: sess,
		cfg:  cfg,
	}
}

// Helper function to populate BasePageData, including auth status
func (h *PageHandler) newBasePageData(ctx *fasthttp.RequestCtx, title, description string) models.BasePageData {
	authStatus := false
	store, err := h.sess.Get(ctx)
	if err == nil {
		if authVal := store.Get("authenticated"); authVal != nil {
			if authenticated, ok := authVal.(bool); ok {
				authStatus = authenticated
			}
		}
	} else {
		// Log error getting session, but proceed assuming not authenticated
		log.Printf("newBasePageData: Error getting session for %s: %v", string(ctx.Path()), err)
	}

	return models.BasePageData{
		PageTitle:       title,
		PageDescription: description,
		AuthStatus:      authStatus,
	}
}

// Index handles GET / - renders the home page.
func (h *PageHandler) Index(ctx *fasthttp.RequestCtx) {
	data := &models.IndexData{
		BasePageData: h.newBasePageData(ctx, "CMS Home", "Modern, lightweight content management system"),
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteIndexPage(ctx, data)
}

// List handles GET /content - renders the list of content items.
func (h *PageHandler) List(ctx *fasthttp.RequestCtx) {
	contents, err := h.db.ListContent()
	if err != nil {
		log.Printf("Error listing content for page: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	data := &models.ListData{
		BasePageData: h.newBasePageData(ctx, "Content List", "All content items"),
		Items:        contents,
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteListPage(ctx, data)
}

// View handles GET /content/{id} - renders a single content item.
func (h *PageHandler) View(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	contentData, err := h.db.GetContent(id)
	if err != nil {
		log.Printf("Error getting content %s for view: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	if contentData == nil {
		ctx.Error("Content not found", fasthttp.StatusNotFound)
		return
	}

	var content models.Content
	if err := json.Unmarshal(contentData, &content); err != nil {
		log.Printf("Error unmarshaling content %s for view: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	data := &models.ViewData{
		BasePageData: h.newBasePageData(ctx, content.Title, "View content item"), // TODO: Use excerpt
		Item:         content,
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteViewPage(ctx, data)
}

// New handles GET /content/new - renders the form to create new content.
func (h *PageHandler) New(ctx *fasthttp.RequestCtx) {
	data := &models.NewData{
		BasePageData: h.newBasePageData(ctx, "Create New Content", "Create a new content item"),
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteCreatePage(ctx, data)
}

// Edit handles GET /content/{id}/edit - renders the form to edit content.
func (h *PageHandler) Edit(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	contentData, err := h.db.GetContent(id)
	if err != nil {
		log.Printf("Error getting content %s for edit: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	if contentData == nil {
		ctx.Error("Content not found", fasthttp.StatusNotFound)
		return
	}

	var content models.Content
	if err := json.Unmarshal(contentData, &content); err != nil {
		log.Printf("Error unmarshaling content %s for edit: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	data := &models.EditData{
		BasePageData: h.newBasePageData(ctx, "Edit: "+content.Title, "Edit content item"),
		Item:         content,
		IsNew:        false,
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteEditPage(ctx, data)
}

// Login handles GET /login - renders the login form.
func (h *PageHandler) Login(ctx *fasthttp.RequestCtx) {
	var store *session.Store
	var err error
	var errorMsgStr, lockoutMsgStr string

	store, err = h.sess.Get(ctx)
	if err != nil {
		log.Printf("Login GET: Error getting session: %v. Creating new store for page.", err)
		store = session.NewStore()
	} else {
		// Try to get messages only if session was retrieved successfully
		if errMsg := store.Get("login_error"); errMsg != nil {
			if s, ok := errMsg.(string); ok {
				errorMsgStr = s
				store.Delete("login_error") // Clear after reading
			}
		}
		if lockoutMsg := store.Get("login_lockout_message"); lockoutMsg != nil {
			if s, ok := lockoutMsg.(string); ok {
				lockoutMsgStr = s
				store.Delete("login_lockout_message") // Clear after reading
			}
		}

		// Save session only if we retrieved it and potentially modified it (cleared messages)
		if errorMsgStr != "" || lockoutMsgStr != "" {
			if saveErr := h.sess.Save(ctx, store); saveErr != nil {
				log.Printf("Login GET: Error saving session after clearing messages: %v", saveErr)
			}
		}
	}

	// Create LoginData, BasePageData now includes AuthStatus (which will be false here)
	data := &models.LoginData{
		BasePageData:   h.newBasePageData(ctx, "Login", "Login to access the CMS"),
		ErrorMessage:   errorMsgStr,
		LockoutMessage: lockoutMsgStr,
	}

	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteLoginPage(ctx, data)
}

// PostLogin handles POST /login - processes login attempt with rate limiting.
func (h *PageHandler) PostLogin(ctx *fasthttp.RequestCtx) {
	username := string(ctx.FormValue("username"))
	password := string(ctx.FormValue("password"))

	// Get session store first to handle attempts and lockout
	store, err := h.sess.Get(ctx)
	if err != nil {
		log.Printf("PostLogin: Error getting session: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// --- Check Lockout Status ---
	var attempts int
	var lastAttemptTime time.Time

	// Get attempt count flexibly
	if attemptsVal := store.Get("login_attempts"); attemptsVal != nil {
		switch v := attemptsVal.(type) {
		case int:
			attempts = v
		case float64: // Handle potential float64 if numbers go through JSON-like serialization
			attempts = int(v)
		case int64:
			attempts = int(v)
		case json.Number:
			if i, err := v.Int64(); err == nil {
				attempts = int(i)
			} else {
				log.Printf("PostLogin: Could not convert json.Number login_attempts '%s' to int64", v.String())
			}
		default:
			log.Printf("PostLogin: Unexpected type for login_attempts in session: %T", attemptsVal)
		}
		log.Printf("PostLogin: Read attempts count from session: %d", attempts) // Debug log
	}

	// Get last attempt time
	if lastAttemptVal := store.Get("last_login_attempt_time"); lastAttemptVal != nil {
		if lastTimeStr, ok := lastAttemptVal.(string); ok {
			lastAttemptTime, _ = time.Parse(time.RFC3339, lastTimeStr) // Ignore parse error for simplicity here
		}
	}

	// Check if currently locked out
	if attempts >= h.cfg.LoginLimitAttempt && !lastAttemptTime.IsZero() {
		lockoutExpiry := lastAttemptTime.Add(h.cfg.LoginLockDuration)
		if time.Now().Before(lockoutExpiry) {
			// LOCKOUT ACTIVE
			remaining := time.Until(lockoutExpiry).Round(time.Second)
			log.Printf("PostLogin: Account locked for user '%s'. Redirecting to home. Time remaining: %v", username, remaining)

			// Instead of showing message, redirect away immediately if already locked
			ctx.Redirect("/", fasthttp.StatusSeeOther) // Redirect to home page
			// Optionally, set a flash message for the home page if needed
			// store.Set("flash_message", "Login attempt blocked due to too many failed attempts.")
			// h.sess.Save(ctx, store) // Need to save if setting flash message
			return
		} else {
			// Lockout expired, reset attempts before checking credentials
			attempts = 0
			store.Delete("login_attempts")
			store.Delete("last_login_attempt_time")
			store.Delete("login_lockout_message") // Clear any old lockout message
			// No need to save yet, will be saved after credential check
		}
	}

	// --- Check Credentials ---
	if username == h.cfg.AuthUser && password == h.cfg.AuthPass {
		// --- Login Successful ---
		log.Printf("PostLogin: Successful login for user '%s'", username)

		// Clear login attempt tracking from session
		store.Delete("login_attempts")
		store.Delete("last_login_attempt_time")
		store.Delete("login_error")
		store.Delete("login_lockout_message")

		// Regenerate session ID for security
		if err := h.sess.Regenerate(ctx); err != nil {
			log.Printf("PostLogin Success: Error regenerating session: %v", err)
			// Re-fetch the store after regeneration, as the old one might be invalid
			newStore, getErr := h.sess.Get(ctx)
			if getErr != nil {
				log.Printf("PostLogin Success: Error getting new session after regenerate: %v", getErr)
				ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
				return
			}
			store = newStore // Use the new store
		} // else: Regeneration succeeded, store is still valid (points to the new session data)

		// Set authentication flag
		store.Set("authenticated", true)
		store.Set("username", username) // Optional: store username

		// Check for redirect URL
		redirectURLVal := store.Get("redirect_url")
		store.Delete("redirect_url") // Remove it after retrieving

		// Save the session
		if err := h.sess.Save(ctx, store); err != nil {
			log.Printf("PostLogin Success: Error saving session: %v", err)
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
			return
		}

		// Redirect after successful login
		if redirectURL, ok := redirectURLVal.(string); ok && redirectURL != "" && redirectURL != "/login" {
			log.Printf("PostLogin Success: Redirecting to saved URL: %s", redirectURL)
			ctx.Redirect(redirectURL, fasthttp.StatusSeeOther)
		} else {
			log.Printf("PostLogin Success: Redirecting to /content")
			ctx.Redirect("/content", fasthttp.StatusSeeOther) // Default redirect
		}
		return // Important: return after redirect

	} else {
		// --- Login Failed ---
		attempts++ // Increment attempts
		now := time.Now()
		store.Set("login_attempts", attempts)
		store.Set("last_login_attempt_time", now.Format(time.RFC3339))
		store.Delete("login_lockout_message") // Clear any previous lockout message

		log.Printf("PostLogin: Failed login attempt #%d for user '%s'", attempts, username)

		// Set error message
		errorMsg := "Invalid username or password."
		if attempts >= h.cfg.LoginLimitAttempt {
			lockoutExpiry := now.Add(h.cfg.LoginLockDuration)
			remaining := time.Until(lockoutExpiry).Round(time.Second)
			errorMsg = fmt.Sprintf("Too many failed login attempts. Please try again in %v.", remaining)
			store.Set("login_lockout_message", errorMsg) // Also set lockout message for next GET
			store.Delete("login_error")                  // Use lockout message instead of generic error
			log.Printf("PostLogin: Account locked for user '%s'. Lockout duration: %v", username, h.cfg.LoginLockDuration)
		} else {
			remainingAttempts := h.cfg.LoginLimitAttempt - attempts
			errorMsg = fmt.Sprintf("Invalid username or password. %d attempts remaining.", remainingAttempts)
			store.Set("login_error", errorMsg) // Set error message for GET /login
		}

		// Save session with updated attempts/time/error
		if err := h.sess.Save(ctx, store); err != nil {
			log.Printf("PostLogin Failed: Error saving session with attempts: %v", err)
		}

		// Redirect back to login form
		ctx.Redirect("/login", fasthttp.StatusSeeOther)
	}
}

// Logout handles GET /logout - logs the user out.
func (h *PageHandler) Logout(ctx *fasthttp.RequestCtx) {
	// We don't strictly need to get the store before destroying,
	// but it can be useful for logging the username if stored.
	store, errGet := h.sess.Get(ctx)
	if errGet == nil {
		if username := store.Get("username"); username != nil {
			log.Printf("Logout: Logging out user: %v", username)
		}
	} else {
		log.Printf("Logout: Error getting session before destroy: %v", errGet)
	}

	// Destroy the session
	if err := h.sess.Destroy(ctx); err != nil {
		log.Printf("Logout: Error destroying session: %v", err)
		ctx.Error("Internal Server Error during logout", fasthttp.StatusInternalServerError)
		return
	}

	// Redirect to login page after logout
	log.Println("Logout: User logged out, redirecting to /login")
	ctx.Redirect("/login", fasthttp.StatusSeeOther)
}

// NotFound handles rendering the custom 404 page.
func (h *PageHandler) NotFound(ctx *fasthttp.RequestCtx) {
	// data variable removed as it was unused
	baseData := h.newBasePageData(ctx, "404 Not Found", "The requested page could not be found.")
	ctx.SetStatusCode(fasthttp.StatusNotFound)
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteNotFoundPage(ctx, &baseData) // Pass the base data directly
}

// Admin handles GET /admin - renders the placeholder admin page.
func (h *PageHandler) Admin(ctx *fasthttp.RequestCtx) {
	data := &models.BasePageData{}
	*data = h.newBasePageData(ctx, "Admin Panel", "Admin section (under construction)")
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteAdminPage(ctx, data)
}

// Settings handles GET /settings - renders the placeholder settings page.
func (h *PageHandler) Settings(ctx *fasthttp.RequestCtx) {
	data := &models.BasePageData{}
	*data = h.newBasePageData(ctx, "Settings", "Settings page (under construction)")
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteSettingsPage(ctx, data)
}
