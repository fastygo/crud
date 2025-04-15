package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"cms/internal/config"
	"cms/internal/models"

	session "github.com/fasthttp/session/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

// CRUDHandler handles API requests for content management.
type CRUDHandler struct {
	sess           *session.Session
	cfg            *config.Config
	initialContent map[string]models.Content // Added initial content map
	parserPool     fastjson.ParserPool
}

// NewCRUDHandler creates a new CRUD handler.
func NewCRUDHandler(sess *session.Session, cfg *config.Config, initialContent map[string]models.Content) *CRUDHandler {
	return &CRUDHandler{
		sess:           sess,
		cfg:            cfg,
		initialContent: initialContent,
		// parserPool is implicitly initialized
	}
}

// Helper function to get user's content map from session or initialize it
func (h *CRUDHandler) getUserContent(ctx *fasthttp.RequestCtx) (map[string]models.Content, error) {
	store, err := h.sess.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	contentData := store.Get("user_content")
	if contentData == nil {
		// Not initialized, clone initial data and serialize to JSON
		log.Println("getUserContent (CRUD): Initializing user content in session")
		userContent := make(map[string]models.Content, len(h.initialContent))
		for k, v := range h.initialContent {
			userContent[k] = v // Shallow copy is okay if Content struct fields are simple types or immutable
		}
		// Serialize content to JSON bytes for storage
		jsonBytes, err := json.Marshal(userContent)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal content for session: %w", err)
		}
		// Store the JSON rather than direct object
		store.Set("user_content", jsonBytes)
		// We need to save the session *now* so subsequent gets in the same request see it
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
		log.Println("getUserContent (CRUD): Attempting conversion from map[string]interface{}")
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal map[string]interface{} for conversion: %w", err)
		}
		if err := json.Unmarshal(b, &userContent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into map[string]models.Content: %w", err)
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
		return nil, fmt.Errorf("unexpected type for user_content in session: %T", contentData)
	}

	return userContent, nil
}

// Helper function to save user's content map back to session
func (h *CRUDHandler) saveUserContent(ctx *fasthttp.RequestCtx, userContent map[string]models.Content) error {
	store, err := h.sess.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get session for saving: %w", err)
	}

	// Serialize to JSON bytes instead of storing the object directly
	jsonBytes, err := json.Marshal(userContent)
	if err != nil {
		return fmt.Errorf("failed to marshal content map for session: %w", err)
	}

	store.Set("user_content", jsonBytes)
	if err := h.sess.Save(ctx, store); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

// List handles GET /api/content - lists all content items for the user.
func (h *CRUDHandler) List(ctx *fasthttp.RequestCtx) {
	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("CRUD List: Error getting user content: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// Convert map to slice for response
	contents := make([]models.Content, 0, len(userContent))
	for _, item := range userContent {
		contents = append(contents, item)
	}
	// TODO: Add sorting if needed

	ctx.SetContentType("application/json; charset=utf-8")
	if err := json.NewEncoder(ctx).Encode(contents); err != nil {
		log.Printf("CRUD List: Error encoding content list: %v", err)
		if !ctx.Response.Header.IsHTTP11() {
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
	}
}

// Get handles GET /api/content/{id} - retrieves a specific content item for the user.
func (h *CRUDHandler) Get(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("CRUD Get: Error getting user content for id %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	item, found := userContent[id]
	if !found {
		ctx.Error("Content not found", fasthttp.StatusNotFound)
		return
	}

	ctx.SetContentType("application/json; charset=utf-8")
	if err := json.NewEncoder(ctx).Encode(item); err != nil {
		log.Printf("CRUD Get: Error encoding item %s: %v", id, err)
		if !ctx.Response.Header.IsHTTP11() {
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
	}
}

// Create handles POST /api/content - creates a new content item in the user's session.
func (h *CRUDHandler) Create(ctx *fasthttp.RequestCtx) {
	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("CRUD Create: Error getting user content: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// Check limit
	if len(userContent) >= 50 {
		log.Printf("CRUD Create: User content limit (50) reached.")
		ctx.Error("Content limit reached. Please delete items before adding more.", fasthttp.StatusConflict) // 409 Conflict
		return
	}

	id, err := generateID()
	if err != nil {
		log.Printf("CRUD Create: Error generating ID: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	body := ctx.PostBody()
	if len(body) == 0 {
		ctx.Error("Request body is empty", fasthttp.StatusBadRequest)
		return
	}

	var newItem models.Content
	if err := json.Unmarshal(body, &newItem); err != nil {
		ctx.Error("Invalid JSON data: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	// Set mandatory fields
	now := time.Now().UTC()
	newItem.ID = id
	newItem.CreatedAt = now
	newItem.UpdatedAt = now
	if newItem.Status == "" {
		newItem.Status = "draft"
	}
	// TODO: Add more validation

	userContent[id] = newItem

	if err := h.saveUserContent(ctx, userContent); err != nil {
		log.Printf("CRUD Create: Error saving user content for id %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json; charset=utf-8")
	ctx.SetStatusCode(fasthttp.StatusCreated)
	fmt.Fprintf(ctx, `{"id":"%s"}`, id)
}

// Update handles PUT /api/content/{id} - updates an item in the user's session.
func (h *CRUDHandler) Update(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("CRUD Update: Error getting user content for id %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	_, found := userContent[id]
	if !found {
		ctx.Error("Content not found", fasthttp.StatusNotFound)
		return
	}

	body := ctx.PostBody()
	if len(body) == 0 {
		ctx.Error("Request body is empty", fasthttp.StatusBadRequest)
		return
	}

	var updatedItem models.Content
	if err := json.Unmarshal(body, &updatedItem); err != nil {
		ctx.Error("Invalid JSON data: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	// Preserve original CreatedAt, ensure ID matches, set UpdatedAt
	originalItem := userContent[id]
	updatedItem.ID = id                            // Ensure ID is correct
	updatedItem.CreatedAt = originalItem.CreatedAt // Keep original creation time
	updatedItem.UpdatedAt = time.Now().UTC()
	// TODO: More validation

	userContent[id] = updatedItem

	if err := h.saveUserContent(ctx, userContent); err != nil {
		log.Printf("CRUD Update: Error saving user content for id %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// Delete handles DELETE /api/content/{id} - deletes an item from the user's session.
func (h *CRUDHandler) Delete(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("CRUD Delete: Error getting user content for id %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	_, found := userContent[id]
	if !found {
		ctx.Error("Content not found", fasthttp.StatusNotFound)
		return
	}

	delete(userContent, id)

	if err := h.saveUserContent(ctx, userContent); err != nil {
		log.Printf("CRUD Delete: Error saving user content after deleting id %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// ExportJSON handles POST /api/export - exports the user's session content.
func (h *CRUDHandler) ExportJSON(ctx *fasthttp.RequestCtx) {
	userContent, err := h.getUserContent(ctx)
	if err != nil {
		log.Printf("CRUD Export: Error getting user content: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// Prepare data in the original export format (map bucket -> map id -> data)
	exportData := map[string]map[string]json.RawMessage{
		"content": make(map[string]json.RawMessage),
	}
	for id, item := range userContent {
		itemJSON, err := json.Marshal(item)
		if err != nil {
			log.Printf("CRUD Export: Error marshaling item %s: %v", id, err)
			// Skip this item or return error?
			continue // Skipping for now
		}
		exportData["content"][id] = itemJSON
	}

	ctx.SetContentType("application/json; charset=utf-8")
	ctx.Response.Header.Set("Content-Disposition", `attachment; filename="cms_export_`+time.Now().UTC().Format("20060102_150405")+`.json"`)

	if err := json.NewEncoder(ctx).Encode(exportData); err != nil {
		log.Printf("CRUD Export: Error encoding database export: %v", err)
		if !ctx.Response.Header.IsHTTP11() {
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
	}
}

// ImportJSON handles POST /api/import - imports data into the user's session.
// WARNING: This replaces existing session data.
func (h *CRUDHandler) ImportJSON(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() || !bytes.Contains(ctx.Request.Header.ContentType(), []byte("multipart/form-data")) {
		ctx.Error("Invalid request method or content type. Use POST with multipart/form-data.", fasthttp.StatusBadRequest)
		return
	}

	form, err := ctx.MultipartForm()
	if err != nil {
		log.Printf("ImportJSON: Error parsing multipart form: %v", err)
		ctx.Error("Failed to parse multipart form: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	fileHeaders := form.File["importFile"]
	if len(fileHeaders) == 0 {
		ctx.Error("No file uploaded with name 'importFile'", fasthttp.StatusBadRequest)
		return
	}
	fileHeader := fileHeaders[0]

	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("ImportJSON: Error opening uploaded file: %v", err)
		ctx.Error("Failed to open uploaded file", fasthttp.StatusInternalServerError)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("ImportJSON: Error reading uploaded file: %v", err)
		ctx.Error("Failed to read uploaded file", fasthttp.StatusInternalServerError)
		return
	}

	var importFormat map[string]map[string]json.RawMessage
	if err := json.Unmarshal(fileBytes, &importFormat); err != nil {
		log.Printf("ImportJSON: Error unmarshaling import JSON from file: %v", err)
		ctx.Error("Invalid JSON format in uploaded file: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	// Extract content bucket data
	contentBucketData, ok := importFormat["content"]
	if !ok {
		log.Printf("ImportJSON: '%s' bucket not found in imported file.", "content")
		ctx.Error(fmt.Sprintf("Invalid import file: '%s' bucket missing.", "content"), fasthttp.StatusBadRequest)
		return
	}

	// Check limit before processing
	if len(contentBucketData) > 50 {
		log.Printf("ImportJSON: Import exceeds content limit (50). Found %d items.", len(contentBucketData))
		ctx.Error(fmt.Sprintf("Import failed: File contains %d items, exceeding the limit of 50.", len(contentBucketData)), fasthttp.StatusConflict)
		return
	}

	// Convert RawMessage map to models.Content map
	importedContent := make(map[string]models.Content, len(contentBucketData))
	for id, rawData := range contentBucketData {
		var item models.Content
		if err := json.Unmarshal(rawData, &item); err != nil {
			log.Printf("ImportJSON: Error unmarshaling item %s: %v", id, err)
			ctx.Error(fmt.Sprintf("Error processing item '%s' in import file: %v", id, err), fasthttp.StatusBadRequest)
			return
		}
		// Optional: Validate imported item further?
		importedContent[id] = item
	}

	// Save the imported content map to the session, replacing existing
	if err := h.saveUserContent(ctx, importedContent); err != nil {
		log.Printf("ImportJSON: Error saving imported content to session: %v", err)
		ctx.Error("Internal Server Error during import save", fasthttp.StatusInternalServerError)
		return
	}

	log.Printf("ImportJSON: Successfully imported %d items into session.", len(importedContent))
	ctx.Redirect("/content?imported=true", fasthttp.StatusSeeOther)
}

// generateID creates a cryptographically secure random hex ID.
func generateID() (string, error) {
	idBytes := make([]byte, 8) // 16 hex characters
	if _, err := rand.Read(idBytes); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return hex.EncodeToString(idBytes), nil
}

// Helper to get the buffer pool from context if needed, although not used in this version.
// func bufferPoolFromCtx(ctx *fasthttp.RequestCtx) *bytebufferpool.Pool {
// 	iface := ctx.UserValue("bufferPool")
// 	if pool, ok := iface.(*bytebufferpool.Pool); ok {
// 		return pool
// 	}
// 	return nil // Or handle error
// }
