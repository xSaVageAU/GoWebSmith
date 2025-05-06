package main

import (
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
	r.Get("/", app.dashboardHandler) // Use the method from the app struct

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

// Helper function to get working directory (can be moved later)
func getProjectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Assuming the executable is run from the project root
	return wd, nil
}
