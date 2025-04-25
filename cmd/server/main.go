package main

import (
	"bytes" // Added for rendering sub-templates
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort" // Added for sorting
	"strings"
	"sync" // Added for map concurrency

	"go-module-builder/internal/model"
	"go-module-builder/internal/storage"
)

// Global variable to hold loaded modules
var loadedModules []*model.Module

// Global variable to hold the base layout templates
var baseTemplates *template.Template

// Global map to hold template sets for each module (ModuleID -> *template.Template)
var moduleTemplates map[string]*template.Template
var moduleTemplatesMutex sync.RWMutex // Mutex for safe concurrent access

var projectRoot string

// PageData holds the data passed to the main layout and page templates
type PageData struct {
	Module          *model.Module
	RenderedContent template.HTML // Pre-rendered HTML of sorted sub-templates
}

func main() {
	// 1. Define and parse command-line flags
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	// Get working directory and store globally
	var err error
	projectRoot, err = os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	staticDir := filepath.Join(projectRoot, "web", "static")
	metadataDir := filepath.Join(projectRoot, ".module_metadata")  // Path to metadata
	templatesDir := filepath.Join(projectRoot, "web", "templates") // Path to main templates
	modulesDir := filepath.Join(projectRoot, "modules")            // Path to modules directory
	log.Printf("Serving static files from: %s", staticDir)
	log.Printf("Loading module metadata from: %s", metadataDir)
	log.Printf("Loading layout templates from: %s", templatesDir)

	// Ensure static directory exists (optional, but good practice)
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		log.Printf("Warning: Could not create static directory %s: %v", staticDir, err)
	}

	// --- Module Discovery ---
	store, err := storage.NewJSONStore(metadataDir)
	if err != nil {
		// Log non-fatal error if metadata dir doesn't exist yet
		if os.IsNotExist(err) {
			log.Printf("Metadata directory not found at %s. No modules loaded.", metadataDir)
			loadedModules = make([]*model.Module, 0) // Initialize as empty slice
		} else {
			log.Fatalf("Error initializing storage: %v", err)
		}
	} else {
		loadedModules, err = store.ReadAll()
		if err != nil {
			log.Printf("Warning: Error reading module metadata: %v", err)
			loadedModules = make([]*model.Module, 0) // Initialize as empty on error
		}
	}

	log.Printf("Discovered %d modules:", len(loadedModules))
	for _, mod := range loadedModules {
		log.Printf("  - ID: %s, Name: %s, Status: %s", mod.ID, mod.Name, mod.Status)
	}
	// --- End Module Discovery ---

	// Initialize map
	moduleTemplates = make(map[string]*template.Template)

	// --- Template Parsing (Revised with Cloning) ---
	log.Println("Parsing templates...")

	// 1. Parse base/layout templates first
	layoutPattern := filepath.Join(templatesDir, "*.html")
	layoutFiles, err := filepath.Glob(layoutPattern)
	if err != nil || len(layoutFiles) == 0 {
		log.Fatalf("Error finding or no layout templates found matching %s: %v", layoutPattern, err)
		// Cannot proceed without layout
	} else {
		log.Printf("Parsing base layout templates: %v", layoutFiles)
		baseTemplates, err = template.ParseFiles(layoutFiles...)
		if err != nil {
			log.Fatalf("Error parsing base layout templates: %v", err)
		}
	}

	// 2. For each active module, clone base templates and parse module templates into the clone
	for _, mod := range loadedModules {
		if mod.Status == "active" {
			moduleTemplatesDir := filepath.Join(modulesDir, mod.ID, "templates")

			// Find all relevant template files (.html, .tmpl, .css)
			htmlPattern := filepath.Join(moduleTemplatesDir, "*.[th][mt][lm]l") // *.html, *.tmpl
			cssPattern := filepath.Join(moduleTemplatesDir, "*.css")

			htmlFiles, errHtml := filepath.Glob(htmlPattern)
			cssFiles, errCss := filepath.Glob(cssPattern)

			if errHtml != nil {
				log.Printf("Warning: Error finding html/tmpl templates for module %s (%s): %v", mod.Name, mod.ID, errHtml)
			}
			if errCss != nil {
				log.Printf("Warning: Error finding css templates for module %s (%s): %v", mod.Name, mod.ID, errCss)
			}

			moduleFiles := append(htmlFiles, cssFiles...)

			if len(moduleFiles) > 0 {
				log.Printf("Preparing templates for module %s from files: %v", mod.ID, moduleFiles)

				// --- Modification Start ---
				// 1. Parse module files into a temporary, separate set first
				moduleSet := template.New(mod.ID) // Create a new set for the module
				moduleSet, err := moduleSet.ParseFiles(moduleFiles...)
				if err != nil {
					log.Printf("ERROR: Failed to parse module templates for %s: %v", mod.ID, err)
					continue // Skip this module if parsing fails
				}

				// 2. Clone the base template set
				clonedTemplates, err := baseTemplates.Clone()
				if err != nil {
					log.Printf("ERROR: Failed to clone base templates for module %s: %v", mod.ID, err)
					continue // Skip this module if cloning fails
				}

				// 3. Add the successfully parsed module templates to the cloned set
				for _, tmpl := range moduleSet.Templates() {
					if tmpl.Name() == mod.ID { // Skip the top-level template container itself
						continue
					}
					addTmpl, err := clonedTemplates.AddParseTree(tmpl.Name(), tmpl.Tree)
					if err != nil {
						log.Printf("ERROR: Failed to add template '%s' from module %s to cloned set: %v", tmpl.Name(), mod.ID, err)
						// Decide if we should continue or fail the whole module preparation
						// For now, let's log and continue, but this might lead to runtime errors
						continue
					}
					clonedTemplates = addTmpl // Update clonedTemplates with the result of AddParseTree
				}

				// Parse module files into the *cloned* set
				clonedTemplates, err = clonedTemplates.ParseFiles(moduleFiles...)
				if err != nil {
					// --- Log the specific error more clearly ---
					log.Printf("CRITICAL: Failed to parse templates for module %s (%s). The specific error was: %v", mod.Name, mod.ID, err)
					log.Printf("CRITICAL: Module %s will NOT be available.", mod.ID)
					// --- End logging change ---
					continue // Skip this module if parsing fails
				}
				// --- Modification End ---

				// Store the completed template set for this module
				moduleTemplatesMutex.Lock()
				moduleTemplates[mod.ID] = clonedTemplates // Store the combined set
				moduleTemplatesMutex.Unlock()
				log.Printf("Successfully prepared templates for module %s", mod.ID)

			} else {
				log.Printf("No template files (.html, .tmpl, .css) found for active module %s in %s", mod.ID, moduleTemplatesDir)
			}
		}
	}
	log.Println("Finished template preparation.")
	// --- End Template Parsing ---

	// 2. Create a new ServeMux (router)
	mux := http.NewServeMux()

	// 3. Setup static file servers
	// Main static files
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))
	// Module static files
	mux.HandleFunc("/modules/", handleModuleStaticRequest) // Add handler for module static files

	// 4. Define page handlers
	mux.HandleFunc("/", handleRootRequest)
	// IMPORTANT: Change /module/ handler registration to be more specific
	// to avoid conflict with /modules/ static handler.
	// We'll use a different prefix, e.g., /view/module/
	mux.HandleFunc("/view/module/", handleModulePageRequest) // Renamed handler and changed path

	// 5. Start the HTTP server
	addr := ":" + *port
	fmt.Printf("Starting server on http://localhost%s\n", addr)
	log.Printf("Listening on port %s...", *port)

	err = http.ListenAndServe(addr, mux)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// handleRootRequest serves the main layout for the root path
func handleRootRequest(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if baseTemplates == nil {
		http.Error(w, "Internal Server Error - Base templates not loaded", http.StatusInternalServerError)
		return
	}

	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX {
		log.Println("HTMX request detected for root. Rendering fragment and clearing module header.")
		// Clear the module header using OOB swap
		_, err := w.Write([]byte(`<span id="module-header-info" hx-swap-oob="innerHTML"></span>`))
		if err != nil {
			log.Printf("Error writing OOB header clear for root: %v", err)
			// Don't necessarily stop, try rendering main content anyway
		}
		// Render only the default page content block (or a specific home fragment if defined)
		// For now, let's render an empty block or a default message
		err = baseTemplates.ExecuteTemplate(w, "page", nil) // Assuming 'page' block exists in layout.html
		if err != nil {
			log.Printf("Error executing page template for root (HTMX): %v", err)
			// Avoid writing generic error if header already sent
			if !strings.Contains(err.Error(), "multiple response.WriteHeader calls") {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	} else {
		// Execute the full layout template directly from the base set for standard requests
		log.Println("Standard request for root. Rendering full layout.html")
		err := baseTemplates.ExecuteTemplate(w, "layout.html", nil) // Use baseTemplates
		if err != nil {
			log.Printf("Error executing layout template for root: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// handleModulePageRequest serves a specific module's main page
func handleModulePageRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Extract Module ID from URL path /view/module/{id}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) != 3 || pathParts[0] != "view" || pathParts[1] != "module" {
		http.NotFound(w, r)
		return
	}
	moduleID := pathParts[2]

	// 2. Find the module by ID
	var targetModule *model.Module
	for _, mod := range loadedModules {
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
	if targetModule.Status != "active" {
		log.Printf("Module %s (%s) is not active (status: %s)", targetModule.Name, moduleID, targetModule.Status)
		http.Error(w, "Module not available", http.StatusForbidden)
		return
	}

	// 4. Get the specific template set for this module
	moduleTemplatesMutex.RLock()
	moduleSpecificTemplates, ok := moduleTemplates[moduleID]
	moduleTemplatesMutex.RUnlock()

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

	// Render sorted templates into a buffer
	var renderedContentBuf bytes.Buffer
	for _, tmplToRender := range renderableTemplates {
		// Derive the defined template name from the filename (e.g., "card.html" -> "card")
		definedName := strings.TrimSuffix(tmplToRender.Name, filepath.Ext(tmplToRender.Name))

		log.Printf("Rendering sub-template with defined name: %s (from file: %s) for module %s", definedName, tmplToRender.Name, moduleID)
		// Execute using the derived definedName
		err := moduleSpecificTemplates.ExecuteTemplate(&renderedContentBuf, definedName, targetModule) // Pass Module data
		if err != nil {
			log.Printf("ERROR rendering sub-template '%s' for module %s: %v", definedName, moduleID, err)
			// Decide how to handle partial failures. For now, log and continue.
			// You might want to return an error to the user instead.
			// Append an error message to the buffer?
			// renderedContentBuf.WriteString(fmt.Sprintf("<p>Error rendering %s</p>", definedName))
		} else {
			// Add a newline or separator if desired between rendered templates
			// renderedContentBuf.WriteString("\n")
		}
	}

	pageData := PageData{
		Module:          targetModule,
		RenderedContent: template.HTML(renderedContentBuf.String()), // Mark as safe HTML
	}
	log.Printf("Prepared %d sorted templates, rendered into combined content for module %s page", len(renderableTemplates), moduleID)

	// 6. Determine if it's an HTMX request
	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX {
		log.Printf("HTMX request detected for module %s. Rendering OOB header and page fragment.", moduleID)

		// Render OOB header swap first
		headerSwapHTML := fmt.Sprintf(`<span id="module-header-info" hx-swap-oob="innerHTML">Module: %s</span>`, template.HTMLEscapeString(targetModule.Name))
		_, err := w.Write([]byte(headerSwapHTML))
		if err != nil {
			log.Printf("Error writing OOB header swap for module %s: %v", moduleID, err)
			return // Stop processing if header write fails
		}

		// Render the main content ('page' block) using the prepared pageData
		err = moduleSpecificTemplates.ExecuteTemplate(w, "page", pageData) // Pass pageData
		if err != nil {
			log.Printf("Error executing 'page' template for module %s (HTMX): %v", moduleID, err)
			return
		}
		log.Printf("Successfully rendered OOB header and page fragment for module %s.", moduleID)

	} else {
		// Standard request: Render the full layout
		log.Printf("Standard request for module %s. Rendering full layout: layout.html", moduleID)
		err := moduleSpecificTemplates.ExecuteTemplate(w, "layout.html", pageData) // Pass pageData
		if err != nil {
			log.Printf("Error executing 'layout.html' template for module %s: %v", err)
			if strings.Contains(err.Error(), "template\" is undefined") {
				http.Error(w, "Internal Server Error - Module template missing", http.StatusInternalServerError)
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	}
}

// handleModuleStaticRequest serves static files from a module's directory
func handleModuleStaticRequest(w http.ResponseWriter, r *http.Request) {
	// Expected path: /modules/{module_id}/static/{file_path}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")

	// Basic validation: must have at least 4 parts (modules, id, static, filename)
	if len(pathParts) < 4 || pathParts[0] != "modules" || pathParts[2] != "static" {
		http.NotFound(w, r)
		return
	}

	moduleID := pathParts[1]
	// Join the remaining parts to get the relative file path
	relativeFilePath := filepath.Join(pathParts[3:]...)

	// Construct the actual file path on disk
	// NOTE: Files are currently generated into the 'templates' subdir by the generator
	// Adjust this path if the generator changes where it puts static assets
	filePath := filepath.Join(projectRoot, "modules", moduleID, "templates", relativeFilePath)

	// Check if file exists and serve it
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Static file not found for module %s: %s (requested path: %s)", moduleID, filePath, r.URL.Path)
		http.NotFound(w, r)
		return
	}

	log.Printf("Serving static file for module %s: %s", moduleID, filePath)
	http.ServeFile(w, r, filePath)
}
