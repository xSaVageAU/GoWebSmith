package main

import (
	"net/http"
	"path/filepath"
	"time" // Keep time for middleware.Timeout

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware" // Import middleware
)

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
	staticPath := filepath.Join(app.projectRoot, "web", "admin", "static")
	app.logger.Info("Serving static files", "path", staticPath, "url_prefix", "/static")

	// Serve static files
	r.Group(func(r chi.Router) {
		r.Use(middleware.StripSlashes) // Optional: helps with trailing slashes
		fs := http.FileServer(http.Dir(staticPath))
		r.Handle("/static/*", http.StripPrefix("/static/", fs))
	})

	// --- Handlers ---
	// These handlers are now defined in handlers.go
	r.Get("/", app.dashboardHandler)

	// Module Creation Routes
	r.Get("/admin/modules/new", app.moduleCreateFormHandler) // Display the form
	r.Post("/admin/modules/new", app.moduleCreateHandler)    // Handle form submission

	// Module Deletion Route
	r.Post("/admin/modules/delete/{moduleID}", app.moduleDeleteHandler) // Handle delete submission

	// Module Editing Route
	r.Get("/admin/modules/edit/{moduleID}", app.moduleEditFormHandler) // Display edit form/placeholder

	// API Route to get template content
	r.Get("/api/admin/modules/{moduleID}/templates/{filename}", app.getModuleTemplateContentHandler)

	// API Route for Live Preview
	r.Post("/api/admin/preview/{moduleID}", app.modulePreviewHandler)

	// API Route to save template content
	r.Put("/api/admin/modules/{moduleID}/templates/{filename}", app.saveModuleTemplateContentHandler)

	return r
}
