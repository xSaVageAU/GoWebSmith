package main

import (
	"bytes"
	"fmt"
	"go-module-builder/internal/model"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug" // Add debug import
	"sort"
	"strings"
	"sync"

	"log/slog" // Import slog

	"github.com/go-chi/chi/v5" // Import chi
)

// --- Error Helper Functions ---

// serverError logs the detailed error and sends a generic 500 Internal Server Error response.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	trace := string(debug.Stack()) // Get stack trace
	app.logger.Error("Internal Server Error", "error", err.Error(), "trace", trace, "method", r.Method, "uri", r.RequestURI)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// clientError sends a specific status code and corresponding message to the client.
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// notFound is a convenience wrapper for sending a 404 Not Found response.
func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}

// --- Struct Definitions ---

// application holds the application-wide dependencies.
type application struct {
	logger *slog.Logger // Add logger field
	// Configuration
	projectRoot         string
	isModuleListEnabled bool
	// Data
	loadedModules []*model.Module
	// Templates
	baseTemplates        *template.Template
	moduleTemplates      map[string]*template.Template
	moduleTemplatesMutex sync.RWMutex
	// Potentially add logger, etc. here later
}

// PageData holds the data passed to the main layout and page templates
// for a specific module page
type PageData struct {
	Module          *model.Module
	RenderedContent template.HTML // Pre-rendered HTML of sorted sub-templates
}

// LayoutData holds the data passed to the main layout template
type LayoutData struct {
	IsModuleListEnabled bool
	PageContent         any // Can be nil, []*model.Module, or PageData
}

// --- Router Setup ---

// routes sets up the HTTP router using chi and application methods.
func (app *application) routes() http.Handler { // Changed return type
	r := chi.NewRouter() // Use chi router

	// --- Middleware (Optional but recommended) ---
	// r.Use(middleware.Logger) // Example: chi's built-in logger
	// r.Use(middleware.Recoverer) // Example: chi's built-in panic recoverer

	// --- Static file servers ---
	// General static assets
	staticDir := filepath.Join(app.projectRoot, "web", "static")
	app.logger.Info("Serving static files", "path", staticDir) // Use slog
	fileServer := http.FileServer(http.Dir(staticDir))
	r.Mount("/static", http.StripPrefix("/static/", fileServer)) // Use Mount

	// Module-specific static assets
	app.logger.Info("Serving module static files", "pattern", "/modules/{moduleID}/static/*") // Use slog
	r.Get("/modules/{moduleID}/static/*", app.handleModuleStaticRequest)                      // Use chi pattern

	// --- Page Handlers ---
	r.Get("/", app.handleRootRequest) // Use r.Get

	// Conditionally register the module list handler
	if app.isModuleListEnabled {
		app.logger.Info("Enabling module list route", "path", "/modules/list") // Use slog
		r.Get("/modules/list", app.handleModuleListRequest)                    // Use r.Get
	}

	// Root-level module page handler (MUST be last to avoid overriding other routes)
	app.logger.Info("Enabling module page route", "pattern", "/{moduleSlug}") // Use slog
	r.Get("/{moduleSlug}", app.handleModulePageRequest)                       // Use slug in pattern

	return r // Return the chi router (which implements http.Handler)
}

// --- HTTP Handlers (Methods on *application) ---

// handleRootRequest serves the main layout for the root path
func (app *application) handleRootRequest(w http.ResponseWriter, r *http.Request) {
	// No need for path check with chi, it handles exact match

	if app.baseTemplates == nil {
		http.Error(w, "Internal Server Error - Base templates not loaded", http.StatusInternalServerError)
		return
	}

	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	layoutData := LayoutData{
		IsModuleListEnabled: app.isModuleListEnabled,
		PageContent:         nil,
	}

	if isHTMX {
		app.logger.Debug("HTMX request detected for root") // Use Debug level
		headerSwapHTML := `<span id="module-header-info" hx-swap-oob="innerHTML"></span>`
		_, err := w.Write([]byte(headerSwapHTML))
		if err != nil {
			app.logger.Error("Error writing OOB header clear for root", "error", err) // Use slog Error
		}
		err = app.baseTemplates.ExecuteTemplate(w, "page", layoutData)
		if err != nil {
			app.logger.Error("Error executing page template for root (HTMX)", "error", err) // Use slog Error
			if !strings.Contains(err.Error(), "multiple response.WriteHeader calls") {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	} else {
		app.logger.Debug("Standard request for root") // Use Debug level
		err := app.baseTemplates.ExecuteTemplate(w, "layout.html", layoutData)
		if err != nil {
			app.logger.Error("Error executing layout template for root", "error", err) // Use slog Error
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// handleModuleListRequest serves the list of modules
func (app *application) handleModuleListRequest(w http.ResponseWriter, r *http.Request) {
	if app.baseTemplates == nil {
		http.Error(w, "Internal Server Error - Base templates not loaded", http.StatusInternalServerError)
		return
	}

	activeModules := make([]*model.Module, 0)
	for _, mod := range app.loadedModules {
		if mod.IsActive {
			activeModules = append(activeModules, mod)
		}
	}

	layoutData := LayoutData{
		IsModuleListEnabled: app.isModuleListEnabled,
		PageContent:         activeModules,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	app.logger.Info("Rendering module list page") // Use Info level
	err := app.baseTemplates.ExecuteTemplate(w, "layout.html", layoutData)
	if err != nil {
		app.logger.Error("Error executing layout template for module list", "error", err) // Use slog Error
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleModulePageRequest serves a specific module's main page using chi URL params (slug)
func (app *application) handleModulePageRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Extract Module Slug using chi
	moduleSlug := chi.URLParam(r, "moduleSlug") // Use moduleSlug param name
	if moduleSlug == "" {
		// This case might occur if the route was somehow matched without the param
		app.logger.Warn("Module slug missing in URL parameter") // Use Warn level
		http.NotFound(w, r)
		return
	}

	// 2. Find the module by Slug
	var targetModule *model.Module
	for _, mod := range app.loadedModules {
		if mod.Slug == moduleSlug { // Match by Slug field
			targetModule = mod
			break
		}
	}

	// 3. Handle not found or inactive module
	if targetModule == nil {
		app.logger.Warn("Module not found for slug", "slug", moduleSlug) // Use Warn level with context
		http.NotFound(w, r)                                              // Treat as 404 if slug doesn't match any loaded module
		return
	}
	if !targetModule.IsActive {
		app.logger.Warn("Attempted to access inactive module", "slug", moduleSlug, "name", targetModule.Name, "id", targetModule.ID) // Use Warn level with context
		http.Error(w, "Module not available", http.StatusForbidden)
		return
	}

	// 4. Get the specific template set for this module (using its ID, not slug)
	app.moduleTemplatesMutex.RLock()
	moduleSpecificTemplates, ok := app.moduleTemplates[targetModule.ID] // Still use ID to lookup templates
	app.moduleTemplatesMutex.RUnlock()

	if !ok {
		app.logger.Error("Template set not found for module", "name", targetModule.Name, "id", targetModule.ID) // Use Error level with context
		http.Error(w, "Internal Server Error - Module templates not loaded", http.StatusInternalServerError)
		return
	}

	// 5. Prepare data: Filter, sort, and pre-render sub-templates
	var renderableTemplates []model.Template
	for _, t := range targetModule.Templates {
		if !t.IsBase && (strings.HasSuffix(t.Name, ".html") || strings.HasSuffix(t.Name, ".tmpl")) {
			renderableTemplates = append(renderableTemplates, t)
		}
	}
	sort.SliceStable(renderableTemplates, func(i, j int) bool {
		return renderableTemplates[i].Order < renderableTemplates[j].Order
	})

	var renderedContentBuf bytes.Buffer
	for _, tmplToRender := range renderableTemplates {
		definedName := strings.TrimSuffix(tmplToRender.Name, filepath.Ext(tmplToRender.Name))
		app.logger.Debug("Rendering sub-template", "template_name", definedName, "module_name", targetModule.Name, "module_slug", moduleSlug) // Use Debug level
		err := moduleSpecificTemplates.ExecuteTemplate(&renderedContentBuf, definedName, targetModule)
		if err != nil {
			app.logger.Error("Error rendering sub-template", "template_name", definedName, "module_name", targetModule.Name, "module_slug", moduleSlug, "error", err) // Use Error level
		}
	}

	pageData := PageData{
		Module:          targetModule,
		RenderedContent: template.HTML(renderedContentBuf.String()),
	}
	app.logger.Debug("Prepared templates for module page", "count", len(renderableTemplates), "module_name", targetModule.Name, "module_slug", moduleSlug) // Use Debug level

	layoutData := LayoutData{
		IsModuleListEnabled: app.isModuleListEnabled,
		PageContent:         pageData,
	}

	// 6. Determine if it's an HTMX request and render
	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX {
		app.logger.Debug("HTMX request detected for module", "module_name", targetModule.Name, "module_slug", moduleSlug) // Use Debug level
		headerSwapHTML := fmt.Sprintf(`<span id="module-header-info" hx-swap-oob="innerHTML">Module: %s</span>`, template.HTMLEscapeString(targetModule.Name))
		_, err := w.Write([]byte(headerSwapHTML))
		if err != nil {
			app.logger.Error("Error writing OOB header swap", "module_name", targetModule.Name, "module_slug", moduleSlug, "error", err) // Use Error level
			return
		}
		err = moduleSpecificTemplates.ExecuteTemplate(w, "page", layoutData.PageContent)
		if err != nil {
			app.logger.Error("Error executing page template (HTMX)", "module_name", targetModule.Name, "module_slug", moduleSlug, "error", err) // Use Error level
			return
		}
		app.logger.Debug("Successfully rendered OOB header and page fragment", "module_name", targetModule.Name, "module_slug", moduleSlug) // Use Debug level
	} else {
		app.logger.Debug("Standard request for module", "module_name", targetModule.Name, "module_slug", moduleSlug) // Use Debug level
		err := moduleSpecificTemplates.ExecuteTemplate(w, "layout.html", layoutData)
		if err != nil {
			app.logger.Error("Error executing layout template", "module_name", targetModule.Name, "module_slug", moduleSlug, "error", err) // Use Error level
			if strings.Contains(err.Error(), "template\" is undefined") {
				http.Error(w, "Internal Server Error - Module template missing", http.StatusInternalServerError)
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	}
}

// handleModuleStaticRequest serves static files from a module's directory using chi URL params
func (app *application) handleModuleStaticRequest(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	// Get the requested file path from the wildcard parameter
	filePathParam := chi.URLParam(r, "*")

	if moduleID == "" || filePathParam == "" {
		app.logger.Warn("Missing parameters in module static request", "module_id", moduleID, "file_path", filePathParam) // Use Warn level
		http.NotFound(w, r)
		return
	}

	// Clean the path to prevent directory traversal issues.
	// Join turns cleaned path segments into a valid path for the OS.
	relativeFilePath := filepath.Join(strings.Split(filePathParam, "/")...)
	if relativeFilePath == "" || strings.Contains(relativeFilePath, "..") {
		app.logger.Warn("Invalid file path requested in module static request", "requested_path", filePathParam, "cleaned_path", relativeFilePath) // Use Warn level
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Construct the actual file path on disk
	// NOTE: Assumes static assets are within the module's 'templates' subdirectory
	filePath := filepath.Join(app.projectRoot, "modules", moduleID, "templates", relativeFilePath)

	// Check if file exists and serve it
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		app.logger.Warn("Module static file not found", "module_id", moduleID, "path", filePath, "requested_url", r.URL.Path)
		app.notFound(w) // Use notFound helper
		return
	} else if err != nil {
		app.logger.Error("Error stating module static file", "module_id", moduleID, "path", filePath, "error", err)
		app.serverError(w, r, err) // Use serverError helper
		return
	}

	app.logger.Debug("Serving module static file", "module_id", moduleID, "path", filePath) // Use Debug level
	http.ServeFile(w, r, filePath)
}
