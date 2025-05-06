package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go-module-builder/internal/model" // Import model package

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware" // Import middleware
)

// TemplateData (General purpose, maybe remove or use later for common layout data if needed)
// type TemplateData struct {
// 	CurrentYear int
// 	Data        any
// }

// DashboardPageData holds all data needed for the dashboard template (layout + content)
type DashboardPageData struct {
	CurrentYear int // Moved here for layout footer
	Modules     []*model.Module
	Error       string // To display errors if module loading fails
}

// routes sets up the HTTP router for the admin application.
func (app *adminApplication) routes() http.Handler {
	r := chi.NewRouter()

	// --- Middleware ---
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger) // Chi's built-in logger
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second)) // Keep timeout

	// --- Static file server ---
	// TODO: Add static file server for web/admin/static later

	// --- Handlers ---
	r.Get("/", app.dashboardHandler)

	// Module Creation Routes
	r.Get("/admin/modules/new", app.moduleCreateFormHandler) // Display the form
	r.Post("/admin/modules/new", app.moduleCreateHandler)    // Handle form submission

	// Module Deletion Route
	r.Post("/admin/modules/delete/{moduleID}", app.moduleDeleteHandler) // Handle delete submission

	// Module Editing Route (Stub)
	r.Get("/admin/modules/edit/{moduleID}", app.moduleEditFormHandler) // Display edit form/placeholder

	return r
}

// dashboardHandler serves the main admin dashboard page.
func (app *adminApplication) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	// Initialize pageData with CurrentYear
	pageData := DashboardPageData{
		CurrentYear: time.Now().Year(),
		Modules:     make([]*model.Module, 0),
	}

	// Check if store was initialized correctly
	if app.moduleStore == nil {
		app.logger.Warn("Module store is not initialized in dashboard handler")
		pageData.Error = "Module storage not available."
	} else {
		// Fetch modules from the store
		modules, err := app.moduleStore.ReadAll()
		if err != nil {
			app.logger.Error("Failed to read modules from store", "error", err)
			// Don't send 500, just show error on dashboard
			pageData.Error = "Failed to load module list."
		} else {
			pageData.Modules = modules
			app.logger.Debug("Loaded modules for dashboard", "count", len(modules))
		}
	}

	// Basic template parsing (will be refined later)
	templateFiles := []string{
		filepath.Join(app.projectRoot, "web", "admin", "templates", "layout.html"),    // Use projectRoot
		filepath.Join(app.projectRoot, "web", "admin", "templates", "dashboard.html"), // Use projectRoot
	}

	// TODO: Cache parsed templates instead of parsing on every request
	ts, err := template.ParseFiles(templateFiles...)
	if err != nil {
		app.logger.Error("Error parsing admin templates", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Execute the layout template, passing pageData directly
	err = ts.ExecuteTemplate(w, "layout.html", pageData)
	if err != nil {
		app.logger.Error("Error executing admin layout template", "error", err)
		// Avoid writing header again if already sent
		// http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// moduleCreateFormHandler displays the form for creating a new module.
func (app *adminApplication) moduleCreateFormHandler(w http.ResponseWriter, r *http.Request) {
	// Basic template parsing (will be refined later)
	templateFiles := []string{
		filepath.Join(app.projectRoot, "web", "admin", "templates", "layout.html"),
		filepath.Join(app.projectRoot, "web", "admin", "templates", "module_form.html"), // Use the new form template
	}

	// TODO: Cache parsed templates
	ts, err := template.ParseFiles(templateFiles...)
	if err != nil {
		app.logger.Error("Error parsing admin create form templates", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Prepare data (just the year for the layout for now)
	templateData := DashboardPageData{ // Reusing DashboardPageData for simplicity, might need dedicated struct later
		CurrentYear: time.Now().Year(),
	}

	// Execute the layout template
	err = ts.ExecuteTemplate(w, "layout.html", templateData)
	if err != nil {
		app.logger.Error("Error executing admin create form layout template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// moduleCreateHandler handles the submission of the new module form.
func (app *adminApplication) moduleCreateHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Parse form data
	err := r.ParseForm()
	if err != nil {
		app.logger.Error("Error parsing create module form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	moduleName := r.PostForm.Get("moduleName")
	customSlug := r.PostForm.Get("customSlug") // Optional

	// Basic validation
	if moduleName == "" {
		// TODO: Improve error handling - show error on form instead of plain text
		http.Error(w, "Module Name is required", http.StatusBadRequest)
		return
	}
	// TODO: Add validation for customSlug format if provided

	// 2. Initialize ModuleManager if not already done (should be done in main)
	// For now, assume app.moduleManager is available and initialized
	// We need to add moduleManager to the adminApplication struct first!
	// Let's assume it exists for now and fix it later.
	if app.moduleManager == nil { // Placeholder check
		app.logger.Error("ModuleManager not initialized in admin application")
		http.Error(w, "Internal Server Error - Configuration Error", http.StatusInternalServerError)
		return
	}

	// 3. Call the manager's CreateModule method
	_, err = app.moduleManager.CreateModule(moduleName, customSlug) // Use app.moduleManager
	if err != nil {
		app.logger.Error("Error creating module via manager", "error", err, "moduleName", moduleName, "customSlug", customSlug)
		// TODO: Improve error handling - show error on form
		http.Error(w, fmt.Sprintf("Failed to create module: %v", err), http.StatusInternalServerError)
		return
	}
	// 4. Redirect back to the dashboard (root path) on success
	// TODO: Add flash message for success
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// moduleDeleteHandler handles the submission for deleting a module.
func (app *adminApplication) moduleDeleteHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Get module ID from URL parameter
	moduleID := chi.URLParam(r, "moduleID")
	if moduleID == "" {
		app.logger.Error("Module ID missing from URL in delete request")
		http.Error(w, "Bad Request - Missing Module ID", http.StatusBadRequest)
		return
	}

	// 2. Parse form data to check for 'force' flag (optional)
	// Even though it's a POST, ParseForm handles query params too if needed,
	// but we expect it in the POST body from the hidden input.
	err := r.ParseForm()
	if err != nil {
		app.logger.Error("Error parsing delete module form", "error", err, "moduleID", moduleID)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	forceDelete := r.PostForm.Get("force") == "true" // Check if force=true was submitted

	// 3. Check module manager initialization
	if app.moduleManager == nil {
		app.logger.Error("ModuleManager not initialized in admin application")
		http.Error(w, "Internal Server Error - Configuration Error", http.StatusInternalServerError)
		return
	}

	// 4. Call the manager's DeleteModule method
	err = app.moduleManager.DeleteModule(moduleID, forceDelete)
	if err != nil {
		app.logger.Error("Error deleting module via manager", "error", err, "moduleID", moduleID, "force", forceDelete)
		// TODO: Improve error handling - show error message on dashboard using flash messages
		// For now, just redirect back with a potential error logged server-side.
		// We could potentially pass an error query param, but flash messages are better.
		http.Redirect(w, r, "/", http.StatusSeeOther) // Redirect even on error for now
		return
	}

	// 5. Redirect back to the dashboard on success
	// TODO: Add flash message for success
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// moduleEditFormHandler displays the placeholder page for editing a module.
func (app *adminApplication) moduleEditFormHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	if moduleID == "" {
		app.logger.Error("Module ID missing from URL in edit request")
		http.Error(w, "Bad Request - Missing Module ID", http.StatusBadRequest)
		return
	}

	// TODO: Implement actual edit form rendering in Phase 3
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotImplemented)
	fmt.Fprintf(w, "Module editing for ID %s is not yet implemented.", moduleID)
}

// Helper function to get working directory (can be moved later)
func getProjectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Assuming the executable is run from the project root
	return wd, nil
}
