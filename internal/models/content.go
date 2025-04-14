package models

import "time"

// Content represents the main data structure for content items.
type Content struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Content     string    `json:"content"` // Consider using a more specific type if needed (e.g., HTML, Markdown)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	Status      string    `json:"status"` // e.g., "draft", "published", "archived"
}

// --- Template Data Structures ---
// Define interfaces and structs needed for rendering templates.

// PageData defines the common interface for data passed to base layouts.
type PageData interface {
	Title() string
	Description() string
	// Add other common fields if needed, e.g., CanonicalURL()
}

// BasePageData provides a basic implementation of PageData.
type BasePageData struct {
	PageTitle       string
	PageDescription string
}

func (d *BasePageData) Title() string {
	return d.PageTitle
}

func (d *BasePageData) Description() string {
	return d.PageDescription
}

// IndexData holds data specifically for the index page template.
type IndexData struct {
	BasePageData // Embed common page data
	// Add index-specific fields here if necessary
}

// ListData holds data for the content list page template.
type ListData struct {
	BasePageData           // Embed common page data
	Items        []Content // The list of content items to display
}

// ViewData holds data for the content view page template.
type ViewData struct {
	BasePageData         // Embed common page data
	Item         Content // The content item being viewed
}

// EditData holds data for the content edit page template.
type EditData struct {
	BasePageData         // Embed common page data
	Item         Content // The content item being edited
	IsNew        bool    // Flag to indicate if this is for creating a new item
}

// NewData holds data for the new content page template.
type NewData struct {
	BasePageData // Embed common page data
	// Add any fields needed for the 'new' form (e.g., default values)
}
