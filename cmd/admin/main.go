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
	"github.com/spf13/viper"     // Added for configuration management
)

// adminApplication holds the application-wide dependencies for the admin server.
type adminApplication struct {
	logger        *slog.Logger
	moduleStore   storage.DataStore             // Corrected interface name
	projectRoot   string                        // Added project root
	moduleManager *modulemanager.ModuleManager  // Added module manager field
	templateCache map[string]*template.Template // Added for template caching
	// Fields for simulated flash messages
	FlashSuccessMessage string
	FlashErrorMessage   string
}

// newTemplateData creates a map of data to pass to templates, including CSRF token and active nav item.
func (app *adminApplication) newTemplateData(r *http.Request, activeNav string) map[string]any {
	// Create a base map.
	data := map[string]any{
		"CSRFToken": nosurf.Token(r),
		"ActiveNav": activeNav, // Identifier for the current active navigation tab
		// "CurrentYear": time.Now().Year(), // Could be added here if not page-specific
	}

	// Add flash messages to template data if they exist
	if app.FlashSuccessMessage != "" {
		data["FlashSuccess"] = app.FlashSuccessMessage
		app.FlashSuccessMessage = "" // Clear after adding to data
	}
	if app.FlashErrorMessage != "" {
		data["FlashError"] = app.FlashErrorMessage
		app.FlashErrorMessage = "" // Clear after adding to data
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
	partialsDir := filepath.Join(adminTemplatesDir, "partials")

	// First, glob all partial files. These will be included with each page
	// and also cached individually if they need to be rendered standalone.
	partialFiles, err := filepath.Glob(filepath.Join(partialsDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("error globbing partials: %w", err)
	}

	// Process main pages
	for _, page := range pages {
		name := page // Use the page filename as the key in the cache

		// Create a slice of files to parse for this page: layout, the page itself, and all partials.
		filesToParse := []string{
			filepath.Join(adminTemplatesDir, "layout.html"),
			filepath.Join(adminTemplatesDir, page),
		}
		filesToParse = append(filesToParse, partialFiles...) // Add all found partials

		// Parse all files into a single template set for this page.
		// The first file in the slice becomes the 'master' template for the set.
		ts, err := template.ParseFiles(filesToParse...)
		if err != nil {
			return nil, fmt.Errorf("error parsing page template set for %s: %w", page, err)
		}
		cache[name] = ts
	}

	// Additionally, cache partials individually so they can be executed directly by handlers.
	// This is important for HTMX partial responses that are not part of a full page render.
	for _, partialFile := range partialFiles {
		name := filepath.Base(partialFile) // e.g., "template_list_items.html"

		// Parse the partial file individually.
		ts, err := template.ParseFiles(partialFile)
		if err != nil {
			return nil, fmt.Errorf("error parsing standalone partial template %s: %w", name, err)
		}
		// If a partial with this name was already added via a page's template set,
		// this individual parsing might overwrite it or be redundant if the content is identical.
		// However, for direct execution, we need it parsed as the primary template in its set.
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

	// --- Configuration ---
	viper.SetConfigName("config")    // name of config file (without extension)
	viper.SetConfigType("yaml")      // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")         // look for config in the working directory
	viper.SetEnvPrefix("GOWS_ADMIN") // Prefix for environment variables for admin server
	viper.AutomaticEnv()             // Read in environment variables that match

	// Set default values
	viper.SetDefault("admin_server.port", "8081")

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("config.yaml not found, using default admin server port.")
		} else {
			logger.Error("Fatal error reading config file for admin server", "error", err)
			os.Exit(1)
		}
	}

	adminPort := viper.GetString("admin_server.port")

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
