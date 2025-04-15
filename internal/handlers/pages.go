//go:generate qtc -dir=../templates
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cms/internal/config"
	"cms/internal/models"

	// Import the specific generated template packages
	"cms/internal/templates/pages"
	// "cms/internal/templates/layouts" - Layouts are called by pages
	// "cms/internal/templates/components" - Components are called by layouts/pages

	session "github.com/fasthttp/session/v2"
	"github.com/valyala/fasthttp"
)

// PageHandler handles requests for HTML pages.
type PageHandler struct {
	sess           *session.Session
	cfg            *config.Config
	initialContent map[string]models.Content // Added initial content map
}

// NewPageHandler creates a new page handler.
func NewPageHandler(sess *session.Session, cfg *config.Config, initialContent map[string]models.Content) *PageHandler {
	return &PageHandler{
		sess:           sess,
		cfg:            cfg,
		initialContent: initialContent,
	}
}

// Helper function to get user's content map from session or initialize it
func (h *PageHandler) getUserContent(ctx *fasthttp.RequestCtx) (map[string]models.Content, error) {
	store, err := h.sess.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	contentData := store.Get("user_content")
	if contentData == nil {
		// Not initialized, clone initial data and serialize to JSON
		log.Println("getUserContent (Page): Initializing user content in session")
		userContent := make(map[string]models.Content, len(h.initialContent))
		for k, v := range h.initialContent {
			userContent[k] = v // Shallow copy
		}
		// Serialize content to JSON bytes for storage
		jsonBytes, err := json.Marshal(userContent)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal content for session: %w", err)
		}
		// Store the JSON rather than direct object
		store.Set("user_content", jsonBytes)
		if err := h.sess.Save(ctx, store); err != nil {
			return nil, fmt.Errorf("failed to save session after initializing content: %w", err)
		}
		return userContent, nil
	}

	// Now handle different types that might be in session
	var userContent map[string]models.Content

	switch v := contentData.(type) {
	case []byte:
		// This is our expected format - JSON bytes
		if err := json.Unmarshal(v, &userContent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal content bytes from session: %w", err)
		}
	case string:
		// Handle if somehow stored as string
		if err := json.Unmarshal([]byte(v), &userContent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal content string from session: %w", err)
		}
	case map[string]interface{}:
		// Handle case where session serialization might change types
		log.Println("getUserContent (Page): Attempting conversion from map[string]interface{}")
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("page: failed to marshal map[string]interface{}: %w", err)
		}
		if err := json.Unmarshal(b, &userContent); err != nil {
			return nil, fmt.Errorf("page: failed to unmarshal into map[string]models.Content: %w", err)
		}
		// Update session with bytes to prevent future type conversions
		jsonBytes, err := json.Marshal(userContent)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal content for session update: %w", err)
		}
		store.Set("user_content", jsonBytes)
		if err := h.sess.Save(ctx, store); err != nil {
			log.Printf("Warning: failed to update session after type conversion: %v", err)
			// Non-fatal
		}
	default:
		return nil, fmt.Errorf("page: unexpected type for user_content: %T", contentData)
	}

	return userContent, nil
}

// Helper function to populate BasePageData, including auth status
// Now uses getUserContent to potentially initialize session data if needed
func (h *PageHandler) newBasePageData(ctx *fasthttp.RequestCtx, title, description string) models.BasePageData {
	authStatus := false
	// We still need the session store for the auth flag
	store, err := h.sess.Get(ctx)
	if err == nil {
		if authVal := store.Get("authenticated"); authVal != nil {
			if authenticated, ok := authVal.(bool); ok {
				authStatus = authenticated
			}
		}
		// Trigger user content initialization if not already done (harmless if already done)
		_, _ = h.getUserContent(ctx) // Ignore errors here, focus is on BasePageData
	} else {
		log.Printf("newBasePageData: Error getting session for %s: %v", string(ctx.Path()), err)
	}

	return models.BasePageData{
		PageTitle:       title,
		PageDescription: description,
		AuthStatus:      authStatus,
	}
}

// Index handles GET / - renders the home page.
// Requires a corresponding template function: pages.WriteIndexPage
func (h *PageHandler) Index(ctx *fasthttp.RequestCtx) {
	// For the index page, we typically just need the base data for layout (e.g., auth status)
	// Content itself isn't usually displayed directly on a generic index page.
	// If specific content *is* needed for the index, logic to fetch/prepare it would go here.
	baseData := h.newBasePageData(ctx, "Home", "Welcome to the CMS")
	ctx.SetContentType("text/html; charset=utf-8")
	// Assuming you have an IndexPage template similar to others
	// pages.WriteIndexPage(ctx, &data) // Use WriteIndexPage
	// Create the correct data type
	data := &models.IndexData{
		BasePageData: baseData,
		// Initialize any other IndexData specific fields if they were added
	}
	pages.WriteIndexPage(ctx, data) // Pass the pointer to models.IndexData
}

// List handles GET /content - renders the list of content items from session.
func (h *PageHandler) List(ctx *fasthttp.RequestCtx) {
	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("Page List: Error getting user content: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// Convert map to slice for template
	contents := make([]models.Content, 0, len(userContent))
	for _, item := range userContent {
		contents = append(contents, item)
	}
	// TODO: Add sorting

	data := &models.ListData{
		BasePageData: h.newBasePageData(ctx, "Content List", "Your current content items"),
		Items:        contents,
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteListPage(ctx, data)
}

// View handles GET /content/{id} - renders a single content item from session.
func (h *PageHandler) View(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("Page View: Error getting user content for id %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	item, found := userContent[id]
	if !found {
		// Redirect to list or show 404? Redirecting to list for now.
		log.Printf("Page View: Item %s not found in user session", id)
		ctx.Redirect("/content?notfound="+id, fasthttp.StatusSeeOther)
		return
		// Or: Use NotFound handler
		// h.NotFound(ctx)
		// return
	}

	data := &models.ViewData{
		BasePageData: h.newBasePageData(ctx, item.Title, "View content item"),
		Item:         item,
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteViewPage(ctx, data)
}

// New handles GET /content/new - renders the form to create new content.
func (h *PageHandler) New(ctx *fasthttp.RequestCtx) {
	// Create an empty content item for the form
	newItem := models.Content{
		// Initialize any default fields if necessary
		// ID will be generated on save (POST)
	}

	// Use the Edit page template, but mark it as 'new'
	data := &models.EditData{
		BasePageData: h.newBasePageData(ctx, "Create New Content", "Fill in the details for the new content item"),
		Item:         newItem, // Pass the empty item
		IsNew:        true,    // Indicate this is for creating a new item
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteEditPage(ctx, data) // Reuse the Edit page template
}

// Edit handles GET /content/{id}/edit - renders the form to edit content from session.
func (h *PageHandler) Edit(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("Page Edit: Error getting user content for id %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	item, found := userContent[id]
	if !found {
		log.Printf("Page Edit: Item %s not found in user session", id)
		ctx.Redirect("/content?notfound="+id, fasthttp.StatusSeeOther)
		return
	}

	data := &models.EditData{
		BasePageData: h.newBasePageData(ctx, "Edit: "+item.Title, "Edit content item"),
		Item:         item,
		IsNew:        false,
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteEditPage(ctx, data)
}

// Login handles GET /login - renders the login form.
// Logic for initializing user_content happens in getUserContent, called by newBasePageData.
// Existing Login handler is mostly fine, might tweak message clearing slightly.
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

// PostLogin handles POST /login - processes login attempt.
// Need to ensure user_content is cleared/reset upon successful login.
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

		// Get session store BEFORE regenerating
		store, err := h.sess.Get(ctx)
		if err != nil {
			log.Printf("PostLogin Success: Error getting session before clearing: %v", err)
			// Proceed, but might not clear old data if session was invalid
		} else {
			// Clear login attempt tracking & old user content
			store.Delete("login_attempts")
			store.Delete("last_login_attempt_time")
			store.Delete("login_error")
			store.Delete("login_lockout_message")
			store.Delete("user_content") // <<--- IMPORTANT: Clear previous user content
			// Save immediately after clearing and before regenerating
			if errSave := h.sess.Save(ctx, store); errSave != nil {
				log.Printf("PostLogin Success: Error saving session after clearing: %v", errSave)
				// Non-fatal, try regenerating anyway
			}
		}

		// Regenerate session ID for security
		if errRegen := h.sess.Regenerate(ctx); errRegen != nil {
			log.Printf("PostLogin Success: Error regenerating session: %v", errRegen)
			// Need to fetch the new store after failed regenerate?
			// Let's assume we continue with the old store if regenerate fails,
			// but log it. If Get fails below, it will be handled.
		}

		// Get the store again (might be new one after successful regenerate)
		store, err = h.sess.Get(ctx)
		if err != nil {
			log.Printf("PostLogin Success: Error getting session after regenerate/clear: %v", err)
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
			return
		}

		// Set authentication flag in the potentially new store
		store.Set("authenticated", true)
		store.Set("username", username) // Optional: store username

		// Check for redirect URL (already deleted from old store if possible)
		redirectURLVal := store.Get("redirect_url") // Check in current store
		store.Delete("redirect_url")

		// Save the session (contains auth flags, maybe cleared redirect_url)
		if err = h.sess.Save(ctx, store); err != nil {
			log.Printf("PostLogin Success: Error saving final session: %v", err)
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
// Session destruction handles clearing user_content automatically.
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
