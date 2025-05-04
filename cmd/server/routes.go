package main

import (
	"bytes"
	"fmt"
	"go-module-builder/internal/model"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5" // Import chi
)

// --- Struct Definitions ---

// application holds the application-wide dependencies.
type application struct {
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
	log.Printf("Router Setup: Serving static files from: %s", staticDir)
	fileServer := http.FileServer(http.Dir(staticDir))
	r.Mount("/static", http.StripPrefix("/static/", fileServer)) // Use Mount

	// Module-specific static assets
	log.Printf("Router Setup: Serving module static files via /modules/{moduleID}/static/*")
	r.Get("/modules/{moduleID}/static/*", app.handleModuleStaticRequest) // Use chi pattern

	// --- Page Handlers ---
	r.Get("/", app.handleRootRequest) // Use r.Get

	// Conditionally register the module list handler
	if app.isModuleListEnabled {
		log.Printf("Router Setup: Enabling /modules/list route")
		r.Get("/modules/list", app.handleModuleListRequest) // Use r.Get
	}

	// Root-level module page handler (MUST be last to avoid overriding other routes)
	log.Printf("Router Setup: Enabling /{moduleID} route for module pages")
	r.Get("/{moduleID}", app.handleModulePageRequest) // Use chi pattern at root

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
		log.Println("HTMX request detected for root. Rendering fragment and clearing module header.")
		headerSwapHTML := `<span id="module-header-info" hx-swap-oob="innerHTML"></span>`
		_, err := w.Write([]byte(headerSwapHTML))
		if err != nil {
			log.Printf("Error writing OOB header clear for root: %v", err)
		}
		err = app.baseTemplates.ExecuteTemplate(w, "page", layoutData)
		if err != nil {
			log.Printf("Error executing page template for root (HTMX): %v", err)
			if !strings.Contains(err.Error(), "multiple response.WriteHeader calls") {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	} else {
		log.Println("Standard request for root. Rendering full layout.html")
		err := app.baseTemplates.ExecuteTemplate(w, "layout.html", layoutData)
		if err != nil {
			log.Printf("Error executing layout template for root: %v", err)
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
	log.Println("Rendering module list page (/modules/list)")
	err := app.baseTemplates.ExecuteTemplate(w, "layout.html", layoutData)
	if err != nil {
		log.Printf("Error executing layout template for module list: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleModulePageRequest serves a specific module's main page using chi URL params
func (app *application) handleModulePageRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Extract Module ID using chi
	moduleID := chi.URLParam(r, "moduleID")
	if moduleID == "" {
		// This case might occur if the route was somehow matched without the param
		log.Println("Error: Module ID missing in URL parameter for handleModulePageRequest")
		http.NotFound(w, r)
		return
	}

	// 2. Find the module by ID
	var targetModule *model.Module
	for _, mod := range app.loadedModules {
		if mod.ID == moduleID {
			targetModule = mod
			break
		}
	}

	// 3. Handle not found or inactive module
	if targetModule == nil {
		log.Printf("Module with ID %s not found via URL param", moduleID)
		http.NotFound(w, r) // Let this fall through - maybe another route matches? Or handle 404 here.
		// For now, let's assume if the param is present but module not found, it's a 404 for this handler.
		return
	}
	if !targetModule.IsActive {
		log.Printf("Module %s (%s) is not active (IsActive: %v)", targetModule.Name, moduleID, targetModule.IsActive)
		http.Error(w, "Module not available", http.StatusForbidden)
		return
	}

	// 4. Get the specific template set for this module
	app.moduleTemplatesMutex.RLock()
	moduleSpecificTemplates, ok := app.moduleTemplates[moduleID]
	app.moduleTemplatesMutex.RUnlock()

	if !ok {
		log.Printf("Template set not found for module %s. Was it parsed correctly?", moduleID)
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
		log.Printf("Rendering sub-template with defined name: %s (from file: %s) for module %s", definedName, tmplToRender.Name, moduleID)
		err := moduleSpecificTemplates.ExecuteTemplate(&renderedContentBuf, definedName, targetModule)
		if err != nil {
			log.Printf("ERROR rendering sub-template '%s' for module %s: %v", definedName, moduleID, err)
		}
	}

	pageData := PageData{
		Module:          targetModule,
		RenderedContent: template.HTML(renderedContentBuf.String()),
	}
	log.Printf("Prepared %d sorted templates, rendered into combined content for module %s page", len(renderableTemplates), moduleID)

	layoutData := LayoutData{
		IsModuleListEnabled: app.isModuleListEnabled,
		PageContent:         pageData,
	}

	// 6. Determine if it's an HTMX request and render
	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX {
		log.Printf("HTMX request detected for module %s. Rendering OOB header and page fragment.", moduleID)
		headerSwapHTML := fmt.Sprintf(`<span id="module-header-info" hx-swap-oob="innerHTML">Module: %s</span>`, template.HTMLEscapeString(targetModule.Name))
		_, err := w.Write([]byte(headerSwapHTML))
		if err != nil {
			log.Printf("Error writing OOB header swap for module %s: %v", moduleID, err)
			return
		}
		err = moduleSpecificTemplates.ExecuteTemplate(w, "page", layoutData.PageContent)
		if err != nil {
			log.Printf("Error executing 'page' template for module %s (HTMX): %v", moduleID, err)
			return
		}
		log.Printf("Successfully rendered OOB header and page fragment for module %s.", moduleID)
	} else {
		log.Printf("Standard request for module %s. Rendering full layout: layout.html", moduleID)
		err := moduleSpecificTemplates.ExecuteTemplate(w, "layout.html", layoutData)
		if err != nil {
			log.Printf("Error executing 'layout.html' template for module %s: %v", moduleID, err)
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
		log.Printf("Error: Missing moduleID (%q) or filePath (%q) in module static request", moduleID, filePathParam)
		http.NotFound(w, r)
		return
	}

	// Clean the path to prevent directory traversal issues.
	// Join turns cleaned path segments into a valid path for the OS.
	relativeFilePath := filepath.Join(strings.Split(filePathParam, "/")...)
	if relativeFilePath == "" || strings.Contains(relativeFilePath, "..") {
		log.Printf("Error: Invalid file path requested: %q (cleaned: %q)", filePathParam, relativeFilePath)
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Construct the actual file path on disk
	// NOTE: Assumes static assets are within the module's 'templates' subdirectory
	filePath := filepath.Join(app.projectRoot, "modules", moduleID, "templates", relativeFilePath)

	// Check if file exists and serve it
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Static file not found for module %s: %s (requested path: %s)", moduleID, filePath, r.URL.Path)
		http.NotFound(w, r)
		return
	} else if err != nil {
		log.Printf("Error stating static file for module %s: %s. Error: %v", moduleID, filePath, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("Serving static file for module %s: %s", moduleID, filePath)
	http.ServeFile(w, r, filePath)
}
