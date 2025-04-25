package model

import "time"

// Template represents a single template file (HTML, CSS, etc.) within a module.
type Template struct {
	ID       string `json:"id"`       // Unique identifier for the template
	Name     string `json:"name"`     // Filename (e.g., "card.html", "styles.css")
	Path     string `json:"path"`     // Relative path within the module's directory
	Content  string `json:"-"`        // File content (often loaded on demand, ignored in JSON)
	IsBase   bool   `json:"isBase"`   // Is this the base template for the module?
	InsertID string `json:"insertId"` // ID or marker in the base template where this should be inserted
	Order    int    `json:"order"`    // Order of insertion for sub-templates
}

// Module represents a self-contained component or website section.
type Module struct {
	ID          string     `json:"id"`        // Unique identifier for the module
	Name        string     `json:"name"`      // User-friendly name (e.g., "Product Card", "Header")
	Directory   string     `json:"directory"` // Path to the module's root directory
	Status      string     `json:"status"`    // Status of the module (e.g., "active", "removed")
	Order       int        `json:"order"`     // Order for display/processing
	CreatedAt   time.Time  `json:"createdAt"`
	LastUpdated time.Time  `json:"lastUpdated"`
	Templates   []Template `json:"templates"` // List of templates belonging to this module
	// Add other metadata as needed, e.g., description, tags
}
