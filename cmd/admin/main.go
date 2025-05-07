package main

import (
	"fmt"
	"html/template" // Added for template cache
	"log/slog"
	"net/http"
	"os"
	"path/filepath" // Added for joining paths

	// Added for module type
	"go-module-builder/internal/modulemanager"
	"go-module-builder/internal/storage" // Added for storage interface

	"github.com/justinas/nosurf" // Added for CSRF token in template data
)

// adminApplication holds the application-wide dependencies for the admin server.
type adminApplication struct {
	logger        *slog.Logger
	moduleStore   storage.DataStore             // Corrected interface name
	projectRoot   string                        // Added project root
	moduleManager *modulemanager.ModuleManager  // Added module manager field
	templateCache map[string]*template.Template // Added for template caching
}

// newTemplateData creates a map of data to pass to templates, including CSRF token and active nav item.
func (app *adminApplication) newTemplateData(r *http.Request, activeNav string) map[string]any {
	// Create a base map.
	data := map[string]any{
		"CSRFToken": nosurf.Token(r),
		"ActiveNav": activeNav, // Identifier for the current active navigation tab
		// "CurrentYear": time.Now().Year(), // Could be added here if not page-specific
		// Add other common data here, e.g., flash messages, authentication status
	}
	return data
}

func newTemplateCache(projectRoot string) (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	// Define pages that use the layout.html as a base
	// and define their own "content" block.
	pages := []string{
		"dashboard.html",
		"module_form.html",
		"module_editor.html",
	}

	// Path to the admin templates directory
	adminTemplatesDir := filepath.Join(projectRoot, "web", "admin", "templates")

	for _, page := range pages {
		name := page // Use the filename as the key in the cache

		// Create a new template set.
		// Add any functions if you have them (e.g., .Funcs(functions))
		// Parse the base layout template first.
		ts, err := template.ParseFiles(filepath.Join(adminTemplatesDir, "layout.html"))
		if err != nil {
			return nil, fmt.Errorf("error parsing layout template: %w", err)
		}

		// Parse the specific page template, adding it to the set.
		// The page template should define blocks that layout.html expects (e.g., "content", "extra_css").
		ts, err = ts.ParseFiles(filepath.Join(adminTemplatesDir, page))
		if err != nil {
			return nil, fmt.Errorf("error parsing page template %s: %w", page, err)
		}
		cache[name] = ts
	}
	return cache, nil
}

func main() {
	// --- Initialize Logger ---
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// --- Initialize Application Struct ---
	// Get working directory (assuming run from project root)
	wd, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get working directory", "error", err)
		os.Exit(1)
	}
	projRoot := wd
	metadataDir := filepath.Join(projRoot, ".module_metadata")
	logger.Info("Using metadata directory", "path", metadataDir)

	// --- Initialize Storage ---
	store, err := storage.NewJSONStore(metadataDir)
	if err != nil {
		// Log non-fatal error if dir doesn't exist, fatal otherwise
		if os.IsNotExist(err) {
			logger.Warn("Metadata directory not found, no modules will be loaded initially.", "path", metadataDir)
			// Create an empty store or handle appropriately if needed
			// For now, we might proceed with an uninitialized store or a dummy one
			// Let's proceed, routes.go handler will need to check if store is nil or handle error
		} else {
			logger.Error("Failed to initialize module store", "error", err)
			os.Exit(1)
		}
	}

	// Initialize Module Manager
	// Note: Using the same modules directory as the CLI/Server for now.
	modulesDir := filepath.Join(projRoot, "modules")                         // Define modules dir path
	manager := modulemanager.NewManager(store, logger, projRoot, modulesDir) // Use same logger for now

	// --- Initialize Application Struct ---
	// Initialize Template Cache
	templateCache, err := newTemplateCache(projRoot)
	if err != nil {
		logger.Error("Failed to create template cache", "error", err)
		os.Exit(1)
	}
	logger.Info("Admin UI templates cached successfully")

	app := &adminApplication{
		logger:        logger,
		moduleStore:   store, // Assign the initialized store
		projectRoot:   projRoot,
		moduleManager: manager,       // Assign the initialized manager
		templateCache: templateCache, // Assign the initialized cache
	}

	// --- Configuration (Placeholder) ---
	// TODO: Integrate with Viper or flags later
	adminPort := "8081"

	// --- Start Server ---
	addr := ":" + adminPort
	logger.Info("Starting admin server", "address", fmt.Sprintf("http://localhost%s", addr))

	// Get the router from the routes method
	router := app.routes() // This now uses the app variable

	err = http.ListenAndServe(addr, router) // Corrected assignment from := to =
	if err != nil {
		logger.Error("Admin server failed to start", "error", err)
		os.Exit(1) // Keep os.Exit(1)
	}
}
