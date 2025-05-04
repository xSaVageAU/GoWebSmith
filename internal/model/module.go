package model

import "time"

// Template represents a single template file (HTML, CSS, etc.) within a module.
type Template struct {
	// ID       string `json:"id"`        // Unique identifier for the template - REMOVED (Unused)
	Name    string `json:"name"`   // Filename (e.g., "card.html", "styles.css")
	Path    string `json:"path"`   // Relative path within the module's directory
	Content string `json:"-"`      // File content (often loaded on demand, ignored in JSON)
	IsBase  bool   `json:"isBase"` // Is this the base template for the module?
	// InsertID string `json:"insertId"`  // ID or marker in the base template where this should be inserted - REMOVED (Unused)
	Order    int  `json:"order"`     // Order of insertion for sub-templates
	IsActive bool `json:"is_active"` // Whether this specific template file is enabled
}

// Module represents a self-contained component or website section,
// including its metadata stored in the corresponding JSON file.
type Module struct {
	ID        string `json:"id"`        // Unique identifier for the module
	Name      string `json:"name"`      // User-friendly name (e.g., "Product Card", "Header")
	Directory string `json:"directory"` // Path to the module's root directory
	// Status      string     `json:"status"`    // Status of the module (e.g., "active", "removed") - Consider if IsActive replaces this need - REMOVED, use IsActive
	Order       int        `json:"order"` // Order for display/processing
	CreatedAt   time.Time  `json:"createdAt"`
	LastUpdated time.Time  `json:"lastUpdated"`
	IsActive    bool       `json:"is_active"`             // Whether the module is enabled (true) or disabled (false)
	Group       string     `json:"group,omitempty"`       // Group this module belongs to
	Layout      string     `json:"layout,omitempty"`      // Specific layout file override (relative path from web/templates?)
	Assets      []string   `json:"assets,omitempty"`      // List of global asset identifiers associated
	Templates   []Template `json:"templates"`             // List of templates belonging to this module (Loaded dynamically, might not be saved in meta JSON)
	Description string     `json:"description,omitempty"` // Optional description (Moved from ModuleMeta)
	// Add other metadata as needed, e.g., version, author, tags
}
