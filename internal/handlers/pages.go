package handlers

import (
	"encoding/json"
	"log"

	"cms/internal/models"
	"cms/internal/storage"

	// Import the specific generated template packages
	"cms/internal/templates/pages"
	// "cms/internal/templates/layouts" - Layouts are called by pages
	// "cms/internal/templates/components" - Components are called by layouts/pages

	"github.com/valyala/fasthttp"
)

// PageHandler handles requests for HTML pages.
type PageHandler struct {
	db *storage.EphemeralBoltDB
}

// NewPageHandler creates a new page handler.
func NewPageHandler(db *storage.EphemeralBoltDB) *PageHandler {
	return &PageHandler{
		db: db,
	}
}

// Index handles GET / - renders the home page.
func (h *PageHandler) Index(ctx *fasthttp.RequestCtx) {
	data := &models.IndexData{
		BasePageData: models.BasePageData{
			PageTitle:       "CMS Home",
			PageDescription: "Modern, lightweight content management system",
		},
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
		BasePageData: models.BasePageData{
			PageTitle:       "Content List",
			PageDescription: "All content items",
		},
		Items: contents,
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
		BasePageData: models.BasePageData{
			PageTitle:       content.Title,
			PageDescription: "View content item", // TODO: Use excerpt or meta description
		},
		Item: content,
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteViewPage(ctx, data)
}

// New handles GET /content/new - renders the form to create new content.
func (h *PageHandler) New(ctx *fasthttp.RequestCtx) {
	data := &models.NewData{
		BasePageData: models.BasePageData{
			PageTitle:       "Create New Content",
			PageDescription: "Create a new content item",
		},
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
		BasePageData: models.BasePageData{
			PageTitle:       "Edit: " + content.Title,
			PageDescription: "Edit content item",
		},
		Item:  content,
		IsNew: false,
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteEditPage(ctx, data)
}

// NotFound handles rendering the custom 404 page.
func (h *PageHandler) NotFound(ctx *fasthttp.RequestCtx) {
	data := &models.BasePageData{
		PageTitle:       "404 Not Found",
		PageDescription: "The requested page could not be found.",
	}
	ctx.SetStatusCode(fasthttp.StatusNotFound) // 404
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteNotFoundPage(ctx, data) // notfound
}

// Admin handles GET /admin - renders the placeholder admin page.
func (h *PageHandler) Admin(ctx *fasthttp.RequestCtx) {
	data := &models.BasePageData{
		PageTitle:       "Admin Panel",
		PageDescription: "Admin section (under construction)",
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteAdminPage(ctx, data)
}

// Settings handles GET /settings - renders the placeholder settings page.
func (h *PageHandler) Settings(ctx *fasthttp.RequestCtx) {
	data := &models.BasePageData{
		PageTitle:       "Settings",
		PageDescription: "Settings page (under construction)",
	}
	ctx.SetContentType("text/html; charset=utf-8")
	pages.WriteSettingsPage(ctx, data)
}
