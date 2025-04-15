package models

// BasePageData contains common data for all page templates.
// Defined in content.go or another file.

// LoginData contains data specific to the login page template.
type LoginData struct {
	BasePageData          // Assumes BasePageData is defined in the same package
	ErrorMessage   string // To display general login errors
	LockoutMessage string // To display lockout-specific messages
}

// IndexData defined elsewhere
// ListData defined elsewhere
// ViewData defined elsewhere
// NewData defined elsewhere
// EditData defined elsewhere

// IndexData contains data specific to the index/home page template.
// type IndexData struct {
// 	BasePageData
// 	// Add any index-page specific fields here if needed in the future
// }
