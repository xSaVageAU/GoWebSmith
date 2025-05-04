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

// createServerMux sets up the HTTP router using application methods.
func (app *application) createServerMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Static file server for general assets
	staticDir := filepath.Join(app.projectRoot, "web", "static")
	log.Printf("Mux Setup: Serving static files from: %s", staticDir)
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Static file server for module-specific assets
	modulesDir := filepath.Join(app.projectRoot, "modules")
	log.Printf("Mux Setup: Serving module static files relative to: %s", modulesDir)
	mux.HandleFunc("/modules/", app.handleModuleStaticRequest) // Use method

	// Page handlers
	mux.HandleFunc("/", app.handleRootRequest)                   // Use method
	mux.HandleFunc("/view/module/", app.handleModulePageRequest) // Use method

	// Conditionally register the module list handler
	if app.isModuleListEnabled {
		mux.HandleFunc("/modules/list", app.handleModuleListRequest) // Use method
	}

	return mux
}

// --- HTTP Handlers (Methods on *application) ---

// handleRootRequest serves the main layout for the root path
func (app *application) handleRootRequest(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if app.baseTemplates == nil {
		http.Error(w, "Internal Server Error - Base templates not loaded", http.StatusInternalServerError)
		return
	}

	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	layoutData := LayoutData{
		IsModuleListEnabled: app.isModuleListEnabled, // Use app field
		PageContent:         nil,                     // No specific content for root page
	}

	if isHTMX {
		log.Println("HTMX request detected for root. Rendering fragment and clearing module header.")
		headerSwapHTML := `<span id="module-header-info" hx-swap-oob="innerHTML"></span>`
		_, err := w.Write([]byte(headerSwapHTML))
		if err != nil {
			log.Printf("Error writing OOB header clear for root: %v", err)
		}
		err = app.baseTemplates.ExecuteTemplate(w, "page", layoutData) // Use app field
		if err != nil {
			log.Printf("Error executing page template for root (HTMX): %v", err)
			if !strings.Contains(err.Error(), "multiple response.WriteHeader calls") {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	} else {
		log.Println("Standard request for root. Rendering full layout.html")
		err := app.baseTemplates.ExecuteTemplate(w, "layout.html", layoutData) // Use app field
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

	// Filter only active modules for listing
	activeModules := make([]*model.Module, 0)
	for _, mod := range app.loadedModules { // Use app field
		if mod.IsActive {
			activeModules = append(activeModules, mod)
		}
	}

	layoutData := LayoutData{
		IsModuleListEnabled: app.isModuleListEnabled, // Use app field
		PageContent:         activeModules,           // Pass active modules as PageContent
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	log.Println("Rendering module list page (/modules/list)")
	err := app.baseTemplates.ExecuteTemplate(w, "layout.html", layoutData) // Use app field
	if err != nil {
		log.Printf("Error executing layout template for module list: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleModulePageRequest serves a specific module's main page
func (app *application) handleModulePageRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Extract Module ID
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) != 3 || pathParts[0] != "view" || pathParts[1] != "module" {
		http.NotFound(w, r)
		return
	}
	moduleID := pathParts[2]

	// 2. Find the module by ID
	var targetModule *model.Module
	for _, mod := range app.loadedModules { // Use app field
		if mod.ID == moduleID {
			targetModule = mod
			break
		}
	}

	// 3. Handle not found or inactive module
	if targetModule == nil {
		log.Printf("Module with ID %s not found", moduleID)
		http.NotFound(w, r)
		return
	}
	if !targetModule.IsActive {
		log.Printf("Module %s (%s) is not active (IsActive: %v)", targetModule.Name, moduleID, targetModule.IsActive)
		http.Error(w, "Module not available", http.StatusForbidden)
		return
	}

	// 4. Get the specific template set for this module
	app.moduleTemplatesMutex.RLock()                             // Use app field
	moduleSpecificTemplates, ok := app.moduleTemplates[moduleID] // Use app field
	app.moduleTemplatesMutex.RUnlock()                           // Use app field

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
		IsModuleListEnabled: app.isModuleListEnabled, // Use app field
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

// handleModuleStaticRequest serves static files from a module's directory
func (app *application) handleModuleStaticRequest(w http.ResponseWriter, r *http.Request) {
	// Expected path: /modules/{module_id}/static/{file_path}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")

	if len(pathParts) < 4 || pathParts[0] != "modules" || pathParts[2] != "static" {
		http.NotFound(w, r)
		return
	}

	moduleID := pathParts[1]
	relativeFilePath := filepath.Join(pathParts[3:]...)

	// Construct the actual file path on disk
	filePath := filepath.Join(app.projectRoot, "modules", moduleID, "templates", relativeFilePath) // Use app field

	// Check if file exists and serve it
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Static file not found for module %s: %s (requested path: %s)", moduleID, filePath, r.URL.Path)
		http.NotFound(w, r)
		return
	}

	log.Printf("Serving static file for module %s: %s", moduleID, filePath)
	http.ServeFile(w, r, filePath)
}
