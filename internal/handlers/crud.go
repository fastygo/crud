package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"cms/internal/storage"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

// CRUDHandler handles API requests for content management.
type CRUDHandler struct {
	db         *storage.EphemeralBoltDB
	parserPool fastjson.ParserPool // Use a pool for fastjson parsers
}

// NewCRUDHandler creates a new CRUD handler.
func NewCRUDHandler(db *storage.EphemeralBoltDB) *CRUDHandler {
	return &CRUDHandler{
		db: db,
		// parserPool is implicitly initialized
	}
}

// List handles GET /api/content - lists all content items.
func (h *CRUDHandler) List(ctx *fasthttp.RequestCtx) {
	contents, err := h.db.ListContent()
	if err != nil {
		log.Printf("Error listing content: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json; charset=utf-8")
	if err := json.NewEncoder(ctx).Encode(contents); err != nil {
		log.Printf("Error encoding content list: %v", err)
		// Error is already set by json.NewEncoder potentially
		if !ctx.Response.Header.IsHTTP11() { // Avoid setting error if already set
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
	}
}

// Get handles GET /api/content/{id} - retrieves a specific content item.
func (h *CRUDHandler) Get(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	contentData, err := h.db.GetContent(id)
	if err != nil {
		log.Printf("Error getting content %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	if contentData == nil {
		ctx.Error("Content not found", fasthttp.StatusNotFound)
		return
	}

	ctx.SetContentType("application/json; charset=utf-8")
	ctx.Write(contentData)
}

// generateID creates a cryptographically secure random hex ID.
func generateID() (string, error) {
	idBytes := make([]byte, 8) // 16 hex characters
	if _, err := rand.Read(idBytes); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return hex.EncodeToString(idBytes), nil
}

// Create handles POST /api/content - creates a new content item.
func (h *CRUDHandler) Create(ctx *fasthttp.RequestCtx) {
	id, err := generateID()
	if err != nil {
		log.Printf("Error generating ID for new content: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	body := ctx.PostBody()
	if len(body) == 0 {
		ctx.Error("Request body is empty", fasthttp.StatusBadRequest)
		return
	}

	// Validate JSON structure using fastjson from the pool
	p := h.parserPool.Get()
	_, err = p.ParseBytes(body) // Use ParseBytes for efficiency
	if err != nil {
		h.parserPool.Put(p)
		ctx.Error("Invalid JSON format: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	// --- Data Enrichment & Validation ---
	// Unmarshal into a map or struct to add/modify fields
	var contentMap map[string]interface{}
	if err := json.Unmarshal(body, &contentMap); err != nil {
		h.parserPool.Put(p)
		log.Printf("Error unmarshaling body for create: %v", err)
		ctx.Error("Invalid JSON data", fasthttp.StatusBadRequest)
		return
	}

	// Set mandatory fields
	now := time.Now().UTC()
	contentMap["id"] = id
	contentMap["created_at"] = now
	contentMap["updated_at"] = now
	if _, ok := contentMap["status"]; !ok {
		contentMap["status"] = "draft" // Default status
	}
	// TODO: Add more robust validation (required fields, types, formats)

	// Re-marshal the enriched/validated data
	enrichedBody, err := json.Marshal(contentMap)
	if err != nil {
		h.parserPool.Put(p)
		log.Printf("Error marshaling enriched data for create: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// --- Release fastjson parser ---
	h.parserPool.Put(p) // Release parser back to the pool

	// Save to database
	if err := h.db.CreateContent(id, enrichedBody); err != nil {
		log.Printf("Error creating content %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json; charset=utf-8")
	ctx.SetStatusCode(fasthttp.StatusCreated)
	fmt.Fprintf(ctx, `{"id":"%s"}`, id) // Return only the ID
}

// Update handles PUT /api/content/{id} - updates an existing content item.
func (h *CRUDHandler) Update(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	body := ctx.PostBody()
	if len(body) == 0 {
		ctx.Error("Request body is empty", fasthttp.StatusBadRequest)
		return
	}

	// Validate JSON structure
	p := h.parserPool.Get()
	_, err := p.ParseBytes(body)
	if err != nil {
		h.parserPool.Put(p)
		ctx.Error("Invalid JSON format: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	// --- Data Enrichment & Validation ---
	var contentMap map[string]interface{}
	if err := json.Unmarshal(body, &contentMap); err != nil {
		h.parserPool.Put(p)
		log.Printf("Error unmarshaling body for update %s: %v", id, err)
		ctx.Error("Invalid JSON data", fasthttp.StatusBadRequest)
		return
	}

	// Ensure ID matches and set updated_at
	contentMap["id"] = id // Overwrite ID in body if present
	contentMap["updated_at"] = time.Now().UTC()
	// TODO: Add more robust validation

	// Re-marshal the enriched/validated data
	enrichedBody, err := json.Marshal(contentMap)
	if err != nil {
		h.parserPool.Put(p)
		log.Printf("Error marshaling enriched data for update %s: %v", id, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}
	h.parserPool.Put(p) // Release parser

	// Save to database
	if err := h.db.UpdateContent(id, enrichedBody); err != nil {
		if strings.Contains(err.Error(), "not found") { // Check specific error from storage
			ctx.Error("Content not found", fasthttp.StatusNotFound)
		} else {
			log.Printf("Error updating content %s: %v", id, err)
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent) // Or StatusOK with updated content body
}

// Delete handles DELETE /api/content/{id} - deletes a content item.
func (h *CRUDHandler) Delete(ctx *fasthttp.RequestCtx) {
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		ctx.Error("Missing or invalid content ID", fasthttp.StatusBadRequest)
		return
	}

	if err := h.db.DeleteContent(id); err != nil {
		// Check if the error indicates "not found" - depends on storage implementation
		// If DeleteContent returns an error for not found, handle it.
		// If it's idempotent (no error for not found), this check might not be needed.
		if strings.Contains(err.Error(), "not found") { // Example check
			ctx.Error("Content not found", fasthttp.StatusNotFound)
		} else {
			log.Printf("Error deleting content %s: %v", id, err)
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// ExportJSON handles POST /api/export - exports the database content.
func (h *CRUDHandler) ExportJSON(ctx *fasthttp.RequestCtx) {
	exportData, err := h.db.ExportDatabase()
	if err != nil {
		log.Printf("Error exporting database: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json; charset=utf-8")
	// Set header for file download
	ctx.Response.Header.Set("Content-Disposition", `attachment; filename="cms_export_`+time.Now().UTC().Format("20060102_150405")+`.json"`)

	if err := json.NewEncoder(ctx).Encode(exportData); err != nil {
		log.Printf("Error encoding database export: %v", err)
		// Error might already be set
		if !ctx.Response.Header.IsHTTP11() {
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
	}
}

// ImportJSON handles POST /api/import - imports data into the database.
// WARNING: This replaces existing data.
func (h *CRUDHandler) ImportJSON(ctx *fasthttp.RequestCtx) {
	// Check if it's a multipart/form-data request
	if !ctx.IsPost() || !bytes.Contains(ctx.Request.Header.ContentType(), []byte("multipart/form-data")) {
		ctx.Error("Invalid request method or content type. Use POST with multipart/form-data.", fasthttp.StatusBadRequest)
		return
	}

	// Parse the multipart form
	form, err := ctx.MultipartForm()
	if err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		ctx.Error("Failed to parse multipart form: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}
	// No need to explicitly release the form in fasthttp

	// Get the file from the form (field name 'importFile' from header.qtpl)
	fileHeaders := form.File["importFile"]
	if len(fileHeaders) == 0 {
		ctx.Error("No file uploaded with name 'importFile'", fasthttp.StatusBadRequest)
		return
	}
	if len(fileHeaders) > 1 {
		ctx.Error("Multiple files uploaded with name 'importFile', expected one", fasthttp.StatusBadRequest)
		return
	}
	fileHeader := fileHeaders[0]

	// Open the uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("Error opening uploaded file: %v", err)
		ctx.Error("Failed to open uploaded file", fasthttp.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Read the file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Error reading uploaded file: %v", err)
		ctx.Error("Failed to read uploaded file", fasthttp.StatusInternalServerError)
		return
	}

	// Now parse the JSON from the file content
	var importData map[string]map[string]json.RawMessage
	if err := json.Unmarshal(fileBytes, &importData); err != nil {
		log.Printf("Error unmarshaling import JSON from file: %v", err)
		// Optionally log part of the fileBytes for debugging
		// log.Printf("Invalid JSON content: %s", string(fileBytes[:min(len(fileBytes), 500)]))
		ctx.Error("Invalid JSON format in uploaded file: "+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	// Import data into the database
	if err := h.db.ImportDatabase(importData); err != nil {
		log.Printf("Error importing database: %v", err)
		ctx.Error("Internal Server Error during import", fasthttp.StatusInternalServerError)
		return
	}

	// Send success response
	ctx.SetContentType("application/json; charset=utf-8")
	ctx.SetStatusCode(fasthttp.StatusOK)
	fmt.Fprint(ctx, `{"message":"Import successful"}`)
}

// Helper to get the buffer pool from context if needed, although not used in this version.
// func bufferPoolFromCtx(ctx *fasthttp.RequestCtx) *bytebufferpool.Pool {
// 	iface := ctx.UserValue("bufferPool")
// 	if pool, ok := iface.(*bytebufferpool.Pool); ok {
// 		return pool
// 	}
// 	return nil // Or handle error
// }
