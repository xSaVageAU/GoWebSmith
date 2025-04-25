package generator

import (
	"fmt"
	"go-module-builder/internal/model"
	"go-module-builder/pkg/fsutils"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Config holds the configuration for module generation.
type Config struct {
	BaseDir      string                 // Base directory where module folders are created (e.g., "modules")
	SubDirs      []string               // Subdirectories to create within each module folder
	DefaultFiles map[string]FileContent // Map of filename to its content and target subdir
}

// FileContent defines the content and target subdirectory for a default file.
type FileContent struct {
	Content string
	SubDir  string // Relative path from the module root (e.g., "templates", "")
}

// Sanitize package name: replace non-alphanumeric with underscore, ensure starts with letter
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sanitizePackageName(name string) string {
	sanitized := nonAlphanumeric.ReplaceAllString(name, "_")
	sanitized = strings.ToLower(sanitized)
	if len(sanitized) == 0 || (sanitized[0] >= '0' && sanitized[0] <= '9') {
		sanitized = "module_" + sanitized
	}
	// Ensure it's a valid Go identifier (basic check)
	if len(sanitized) > 0 && (sanitized[0] == '_' || (sanitized[0] >= 'a' && sanitized[0] <= 'z')) {
		// Potentially add more checks if needed
	} else {
		sanitized = "module_" + sanitized // Fallback prefix
	}
	return sanitized
}

// DefaultGeneratorConfig provides a standard configuration for module generation.
func DefaultGeneratorConfig(baseDir string) Config {
	// Define boilerplate content
	// Note: Using {{ .ModuleName }} as a simple placeholder for generator logic.
	// Actual Go template execution happens at runtime by the server.

	// This content is intended to be inserted into a main layout's {{ block "page" . }}
	baseHTMLContent := `{{ define "page" }} {{/* Or define "{{ .ModuleName }}" ? */}}
    {{/* Module-specific CSS inclusion */}}
    <style>
        {{ template "style.css" . }}
    </style>

    <h1>Module: {{ .ModuleName }}</h1>
    <p>This is the default content for the {{ .ModuleName }} module.</p>

    <!-- Example of including another template fragment -->
    <!-- Example: {{/* template "card" . */}} -->

    {{/* Module-specific JS (if needed and not handled globally) */}}
    <!-- <script>
        // Example: Load script.js if it exists
    </script> -->
{{ end }}`

	// Note: This handler is BOILERPLATE. The actual rendering logic
	// (e.g., using a shared renderer package) needs to be implemented
	// in the web server phase and potentially referenced here.
	handlerGoContent := `package {{ .PackageName }} // Dynamic package name

import (
	"fmt"
	"net/http"
	// "path/to/your/renderer" // Import your actual renderer package
	// "path/to/your/models"   // Import models if needed
)

// ModuleData might hold data passed from the handler to the template
type ModuleData struct {
	ModuleName string
	// Add other fields needed by the template(s)
}

// Handle is the main entry point for this module's HTTP requests.
// The main server needs a mechanism to discover and route to this handler.
func Handle(w http.ResponseWriter, r *http.Request) {
	// Example data - replace with actual data fetching/logic
	data := ModuleData{
		ModuleName: "{{ .ModuleName }}", // Use the actual module name
	}

	// Check for HTMX request targeting the main content area
	isHTMX := r.Header.Get("HX-Request") == "true"
	isTargetMain := r.Header.Get("HX-Target") == "main" // Check if the target is 'main'

	if isHTMX && isTargetMain {
		// Render only the "page" block for HTMX requests to #main
		// Replace with your actual template rendering logic
		// err := renderer.ExecuteTemplate(w, "page", data) // Assuming renderer handles finding the right template
		// if err != nil {
		//     http.Error(w, fmt.Sprintf("Error rendering page fragment: %v", err), http.StatusInternalServerError)
		//     return
		// }
		// Placeholder response:
		fmt.Fprintf(w, "<div>HTMX Fragment Response for %s (Render page block here)</div>", data.ModuleName)

	} else {
		// Render the full page (layout + page block) for regular requests
		// Replace with your actual template rendering logic
		// err := renderer.ExecuteTemplate(w, "layout", data) // Assuming layout includes {{ block "page" . }}
		// if err != nil {
		//     http.Error(w, fmt.Sprintf("Error rendering full page: %v", err), http.StatusInternalServerError)
		//     return
		// }
		// Placeholder response:
		fmt.Fprintf(w, "<html><body>Full Page Response for %s (Render layout + page block here)</body></html>", data.ModuleName)
	}
}

// Add other handler functions specific to this module if needed...
// e.g., func HandleFormSubmission(w http.ResponseWriter, r *http.Request) { ... }
`
	styleCSSContent := `/* Basic styles for module {{ .ModuleName }} */
body {
    font-family: sans-serif;
    padding: 1em;
    border: 1px dashed #ccc; /* Example style */
}`

	// Removed scriptJSContent as we are not creating js/script.js by default

	return Config{
		BaseDir: baseDir,
		// Only create the templates subdirectory by default
		SubDirs: []string{"templates"},
		DefaultFiles: map[string]FileContent{
			// Place handler.go in the module root directory
			"handler.go": {
				Content: handlerGoContent,
				SubDir:  "", // Empty string means module root
			},
			// Place templates in the 'templates' subdirectory
			"base.html": {
				Content: baseHTMLContent,
				SubDir:  "templates",
			},
			"style.css": {
				Content: styleCSSContent,
				SubDir:  "templates",
			},
			// Removed script.js from default files
		},
	}
}

// GenerateModuleBoilerplate creates the directory structure and default files for a new module.
// It returns the newly created Module object (with paths populated) or an error.
func GenerateModuleBoilerplate(cfg Config, moduleName, moduleID string) (*model.Module, error) {
	if moduleName == "" || moduleID == "" {
		return nil, fmt.Errorf("module name and ID cannot be empty")
	}

	moduleDir := filepath.Join(cfg.BaseDir, moduleID)

	// 1. Create the main module directory
	// Use CreateDir instead of EnsureDir
	if err := fsutils.CreateDir(moduleDir); err != nil {
		return nil, fmt.Errorf("failed to create module directory %s: %w", moduleDir, err)
	}
	fmt.Printf("Created directory: %s\n", moduleDir)

	// 2. Create subdirectories
	for _, subDir := range cfg.SubDirs {
		fullSubDirPath := filepath.Join(moduleDir, subDir)
		// Use CreateDir instead of EnsureDir
		if err := fsutils.CreateDir(fullSubDirPath); err != nil {
			// Attempt cleanup? Maybe remove moduleDir?
			return nil, fmt.Errorf("failed to create subdirectory %s: %w", fullSubDirPath, err)
		}
		fmt.Printf("Created directory: %s\n", fullSubDirPath)
	}

	now := time.Now()
	newModule := &model.Module{
		ID:          moduleID,
		Name:        moduleName,
		Directory:   moduleDir,
		Status:      "active", // Set initial status to active
		CreatedAt:   now,
		LastUpdated: now,
		// Initialize Templates slice
		Templates: make([]model.Template, 0, len(cfg.DefaultFiles)), // Estimate capacity
	}

	// 3. Create default files and populate Module.Templates metadata
	packageName := sanitizePackageName(moduleName) // Generate package name once

	for filename, fileInfo := range cfg.DefaultFiles {
		filePath := filepath.Join(moduleDir, fileInfo.SubDir, filename)

		// Simple placeholder replacement for generator-time variables
		content := fileInfo.Content
		content = strings.ReplaceAll(content, "{{ .ModuleName }}", moduleName)
		// Replace package name placeholder specifically in handler.go content
		if filename == "handler.go" {
			content = strings.ReplaceAll(content, "{{ .PackageName }}", packageName)
		}

		if err := fsutils.WriteToFile(filePath, []byte(content)); err != nil {
			// Attempt cleanup?
			return nil, fmt.Errorf("failed to create default file %s: %w", filePath, err)
		}
		fmt.Printf("Created file: %s\n", filePath)

		// Add template metadata *only for files intended to be Go templates* (e.g., html, css)
		// We place them in the 'templates' subdir convention
		if fileInfo.SubDir == "templates" {
			isBase := (filename == "base.html") // Convention: base.html is the base
			order := 0                          // Default order
			if filename == "base.html" {
				order = 0
			} else if filename == "style.css" {
				order = 1 // CSS comes after base conceptually
			} // Add more default orders if needed

			template := model.Template{
				// Use Name instead of Filename
				Name:   filename,
				Path:   filepath.Join(fileInfo.SubDir, filename), // Relative path within module dir
				IsBase: isBase,
				Order:  order,
			}
			newModule.Templates = append(newModule.Templates, template)
		}
	}

	return newModule, nil
}
