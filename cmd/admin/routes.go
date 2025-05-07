package main

import (
	"bytes"         // Added for preview buffer
	"encoding/json" // Added for preview API request body
	"fmt"
	"html/template" // Added for io.ReadAll
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort" // Added for sorting sub-templates
	"strings"
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
	staticPath := filepath.Join(app.projectRoot, "web", "admin", "static")
	app.logger.Info("Serving static files", "path", staticPath, "url_prefix", "/static")

	// Serve static files
	r.Group(func(r chi.Router) {
		r.Use(middleware.StripSlashes) // Optional: helps with trailing slashes
		fs := http.FileServer(http.Dir(staticPath))
		r.Handle("/static/*", http.StripPrefix("/static/", fs))
	})

	// --- Handlers ---
	r.Get("/", app.dashboardHandler)

	// Module Creation Routes
	r.Get("/admin/modules/new", app.moduleCreateFormHandler) // Display the form
	r.Post("/admin/modules/new", app.moduleCreateHandler)    // Handle form submission

	// Module Deletion Route
	r.Post("/admin/modules/delete/{moduleID}", app.moduleDeleteHandler) // Handle delete submission

	// Module Editing Route (Stub)
	r.Get("/admin/modules/edit/{moduleID}", app.moduleEditFormHandler) // Display edit form/placeholder

	// API Route to get template content
	r.Get("/api/admin/modules/{moduleID}/templates/{filename}", app.getModuleTemplateContentHandler)

	// API Route for Live Preview
	r.Post("/api/admin/preview/{moduleID}", app.modulePreviewHandler)

	// API Route to save template content
	r.Put("/api/admin/modules/{moduleID}/templates/{filename}", app.saveModuleTemplateContentHandler)

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

// moduleEditFormHandler loads module data and renders the editor page.
func (app *adminApplication) moduleEditFormHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	if moduleID == "" {
		app.logger.Error("Module ID missing from URL in edit request")
		http.Error(w, "Bad Request - Missing Module ID", http.StatusBadRequest)
		return
	}

	// Load module metadata
	if app.moduleManager == nil || app.moduleManager.GetStore() == nil {
		app.logger.Error("Module manager or store not initialized")
		http.Error(w, "Internal Server Error - Configuration Error", http.StatusInternalServerError)
		return
	}
	module, err := app.moduleManager.GetStore().LoadModule(moduleID)
	if err != nil {
		app.logger.Error("Failed to load module for editing", "moduleID", moduleID, "error", err)
		// Differentiate between not found and other errors
		// TODO: Use a more specific error type from storage if available
		if err.Error() == "module metadata file not found" || os.IsNotExist(err) { // Basic check
			http.NotFound(w, r)
		} else {
			http.Error(w, "Failed to load module data", http.StatusInternalServerError)
		}
		return
	}

	// Render the editor template
	templateFiles := []string{
		filepath.Join(app.projectRoot, "web", "admin", "templates", "layout.html"),
		filepath.Join(app.projectRoot, "web", "admin", "templates", "module_editor.html"),
	}

	// TODO: Implement template caching for admin templates
	ts, err := template.ParseFiles(templateFiles...)
	if err != nil {
		app.logger.Error("Error parsing admin editor templates", "error", err, "moduleID", moduleID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Prepare data for the template
	templateData := map[string]any{
		"CurrentYear": time.Now().Year(), // For layout
		"Data":        module,            // Pass the *model.Module object
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = ts.ExecuteTemplate(w, "layout.html", templateData)
	if err != nil {
		app.logger.Error("Error executing admin editor layout template", "error", err, "moduleID", moduleID)
		// Avoid writing header again if already sent
	}
}

// getModuleTemplateContentHandler returns the content of a specific module template file.
func (app *adminApplication) getModuleTemplateContentHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	filename := chi.URLParam(r, "filename")

	if moduleID == "" || filename == "" {
		app.logger.Error("Missing moduleID or filename in get template content request")
		http.Error(w, "Bad Request - Missing moduleID or filename", http.StatusBadRequest)
		return
	}

	// Load module metadata to get its directory
	if app.moduleManager == nil || app.moduleManager.GetStore() == nil {
		app.logger.Error("Module manager or store not initialized for get template content")
		http.Error(w, "Internal Server Error - Configuration Error", http.StatusInternalServerError)
		return
	}
	module, err := app.moduleManager.GetStore().LoadModule(moduleID)
	if err != nil {
		app.logger.Error("Failed to load module for get template content", "moduleID", moduleID, "filename", filename, "error", err)
		http.NotFound(w, r) // Module not found
		return
	}

	// Construct the path to the template file
	var moduleBasePath string
	if filepath.IsAbs(module.Directory) {
		moduleBasePath = module.Directory
	} else {
		moduleBasePath = filepath.Join(app.projectRoot, module.Directory)
	}
	templateFilePath := filepath.Join(moduleBasePath, "templates", filename)

	foundInMeta := false
	for _, tmplMeta := range module.Templates {
		if tmplMeta.Name == filename {
			foundInMeta = true
			break
		}
	}
	if !foundInMeta {
		app.logger.Warn("Requested filename not listed in module metadata", "moduleID", moduleID, "filename", filename, "path", templateFilePath)
		http.Error(w, "Bad Request - Invalid filename for module", http.StatusBadRequest)
		return
	}

	contentBytes, err := os.ReadFile(templateFilePath)
	if err != nil {
		app.logger.Error("Failed to read template file content", "moduleID", moduleID, "filename", filename, "path", templateFilePath, "error", err)
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Internal Server Error - Failed to read file", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err = w.Write(contentBytes)
	if err != nil {
		app.logger.Error("Error writing template content response", "error", err, "moduleID", moduleID, "filename", filename)
	}
}

// PreviewRequestData defines the structure for the preview API request body.
type PreviewRequestData struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// modulePreviewHandler renders a module preview using potentially modified template content.
// It performs its own template parsing on each request for the preview.
func (app *adminApplication) modulePreviewHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	if moduleID == "" {
		app.logger.Error("Module ID missing from URL in preview request")
		http.Error(w, "Bad Request - Missing Module ID", http.StatusBadRequest)
		return
	}

	var reqData PreviewRequestData
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		app.logger.Error("Error decoding preview request body", "error", err, "moduleID", moduleID)
		http.Error(w, "Bad Request - Invalid JSON", http.StatusBadRequest)
		return
	}
	if reqData.Filename == "" {
		app.logger.Error("Filename missing in preview request body", "moduleID", moduleID)
		http.Error(w, "Bad Request - Missing filename", http.StatusBadRequest)
		return
	}
	app.logger.Debug("Preview request received", "moduleID", moduleID, "filename", reqData.Filename)

	if app.moduleManager == nil || app.moduleManager.GetStore() == nil {
		app.logger.Error("Module manager or store not initialized for preview")
		http.Error(w, "Internal Server Error - Configuration Error", http.StatusInternalServerError)
		return
	}
	module, err := app.moduleManager.GetStore().LoadModule(moduleID)
	if err != nil {
		app.logger.Error("Failed to load module for preview", "moduleID", moduleID, "error", err)
		http.NotFound(w, r)
		return
	}

	var moduleBasePath string
	if filepath.IsAbs(module.Directory) {
		moduleBasePath = module.Directory
	} else {
		moduleBasePath = filepath.Join(app.projectRoot, module.Directory)
	}
	moduleTemplatesPath := filepath.Join(moduleBasePath, "templates")

	previewTmplSet := template.New(moduleID + "_preview_admin")
	if len(module.Templates) == 0 {
		app.logger.Warn("No templates defined in module metadata for preview", "moduleID", moduleID)
	}

	for _, tmplMeta := range module.Templates {
		filePath := filepath.Join(moduleTemplatesPath, tmplMeta.Name)
		var currentFileContent string
		if tmplMeta.Name == reqData.Filename {
			currentFileContent = reqData.Content
		} else {
			contentBytes, readErr := os.ReadFile(filePath)
			if readErr != nil {
				app.logger.Error("Failed to read module template for preview", "path", filePath, "error", readErr)
				http.Error(w, "Internal Server Error - Cannot read module template", http.StatusInternalServerError)
				return
			}
			currentFileContent = string(contentBytes)
		}
		templateName := strings.TrimSuffix(tmplMeta.Name, filepath.Ext(tmplMeta.Name))
		// If the file itself defines a template (e.g. {{define "foo"}}), that definition is added to the set.
		// The template created by .New(templateName) is also added to the set.
		// It's generally better to use unique names for New() if you're not relying on ParseFiles behavior.
		// However, for this ad-hoc parsing, ensuring each {{define}} is unique is key.
		if existing := previewTmplSet.Lookup(templateName); existing != nil && tmplMeta.Name != "base.html" { // Allow base.html to redefine "page"
			app.logger.Warn("Template name conflict during preview parsing, might be overwritten", "name", templateName)
		}
		_, err = previewTmplSet.New(templateName).Parse(currentFileContent)
		if err != nil {
			app.logger.Error("Failed to parse module template string for preview", "templateName", templateName, "error", err)
			http.Error(w, fmt.Sprintf("Internal Server Error - Template parse error for %s", templateName), http.StatusInternalServerError)
			return
		}
	}

	var buf bytes.Buffer
	var finalHtmlOutput string
	var renderErr error

	if strings.HasSuffix(reqData.Filename, ".css") {
		buf.WriteString("<style type=\"text/css\">\n")
		buf.WriteString(reqData.Content)
		buf.WriteString("\n</style>")
		finalHtmlOutput = buf.String()
	} else if strings.HasSuffix(reqData.Filename, ".html") || strings.HasSuffix(reqData.Filename, ".tmpl") {
		var entryPointTemplateName string
		var executionData any = module // Default data for direct template execution

		if reqData.Filename == "base.html" {
			entryPointTemplateName = "page" // base.html defines "page"
			app.logger.Debug("Attempting to execute 'page' template for base.html preview", "filename", reqData.Filename)

			renderedSubContent, subRenderErr := app.renderAdminPreviewSubTemplates(previewTmplSet, module, reqData.Filename, reqData.Content)
			if subRenderErr != nil {
				renderErr = fmt.Errorf("failed to render sub-templates for page preview: %w", subRenderErr)
			} else {
				executionData = map[string]any{
					"Module":          module,
					"RenderedContent": template.HTML(renderedSubContent),
				}
			}
		} else { // For other HTML/TMPL files (e.g., content.html), render them directly.
			entryPointTemplateName = strings.TrimSuffix(reqData.Filename, filepath.Ext(reqData.Filename))
			app.logger.Debug("Attempting to execute direct HTML/TMPL preview", "filename", reqData.Filename, "templateName", entryPointTemplateName)
		}

		if renderErr == nil { // Proceed only if sub-template rendering (if any) was successful
			entryPointTemplate := previewTmplSet.Lookup(entryPointTemplateName)
			if entryPointTemplate != nil {
				err = entryPointTemplate.Execute(&buf, executionData)
				if err != nil {
					renderErr = fmt.Errorf("failed executing template '%s': %w", entryPointTemplateName, err)
				}
			} else {
				renderErr = fmt.Errorf("template '%s' not found for preview", entryPointTemplateName)
			}
		}
		if renderErr == nil {
			finalHtmlOutput = buf.String()
		}
	} else {
		app.logger.Warn("Unsupported file type for preview", "filename", reqData.Filename)
		http.Error(w, "Unsupported file type for preview", http.StatusBadRequest)
		return
	}

	if renderErr != nil {
		app.logger.Error("Error during preview rendering", "moduleID", moduleID, "filename", reqData.Filename, "error", renderErr)
		errorMsg := fmt.Sprintf("<pre style='color:red; font-family:monospace;'>Preview Rendering Error:\n%s</pre>", renderErr.Error())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(errorMsg))
		return
	}

	app.logger.Debug("Preview buffer content after execution", "moduleID", moduleID, "filename", reqData.Filename, "bufferString", finalHtmlOutput)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, finalWriteErr := w.Write([]byte(finalHtmlOutput))
	if finalWriteErr != nil {
		app.logger.Error("Error writing preview response", "error", finalWriteErr, "moduleID", moduleID)
	}
}

// renderAdminPreviewSubTemplates renders the non-base HTML/TMPL templates for a module in their specified order.
// It uses an already parsed template set and considers the currently edited file's modified content.
func (app *adminApplication) renderAdminPreviewSubTemplates(previewTmplSet *template.Template, module *model.Module, editingFilename string, editingContent string) (string, error) {
	var subTemplatesToRender []model.Template
	for _, tmplMeta := range module.Templates {
		if !tmplMeta.IsBase && tmplMeta.Name != "base.html" && (strings.HasSuffix(tmplMeta.Name, ".html") || strings.HasSuffix(tmplMeta.Name, ".tmpl")) {
			subTemplatesToRender = append(subTemplatesToRender, tmplMeta)
		}
	}

	sort.SliceStable(subTemplatesToRender, func(i, j int) bool {
		return subTemplatesToRender[i].Order < subTemplatesToRender[j].Order
	})

	var renderedSubContentBuf bytes.Buffer
	for _, subTmplMeta := range subTemplatesToRender {
		subTemplateName := strings.TrimSuffix(subTmplMeta.Name, filepath.Ext(subTmplMeta.Name))

		// Note: The previewTmplSet already contains the potentially modified content
		// for the `editingFilename` because it was parsed in modulePreviewHandler.
		// So, we just need to ensure we execute the correct template from the set.
		subTmpl := previewTmplSet.Lookup(subTemplateName)
		if subTmpl != nil {
			// Sub-templates are executed with 'module' as their direct context
			if err := subTmpl.Execute(&renderedSubContentBuf, module); err != nil {
				app.logger.Error("Error executing sub-template for preview", "name", subTemplateName, "moduleID", module.ID, "error", err)
				return "", fmt.Errorf("failed executing sub-template %s: %w", subTemplateName, err)
			}
		} else {
			app.logger.Warn("Sub-template not found in preview set during ordered rendering", "name", subTemplateName, "moduleID", module.ID)
		}
	}
	return renderedSubContentBuf.String(), nil
}

// saveModuleTemplateContentHandler saves the provided content to a specific module template file.
func (app *adminApplication) saveModuleTemplateContentHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	filename := chi.URLParam(r, "filename")

	if moduleID == "" || filename == "" {
		app.logger.Error("Missing moduleID or filename in save template content request")
		http.Error(w, "Bad Request - Missing moduleID or filename", http.StatusBadRequest)
		return
	}

	// Read the request body (new file content)
	newContentBytes, err := io.ReadAll(r.Body)
	if err != nil {
		app.logger.Error("Failed to read request body for save template", "moduleID", moduleID, "filename", filename, "error", err)
		http.Error(w, "Internal Server Error - Failed to read content", http.StatusInternalServerError)
		return
	}
	// newContent := string(newContentBytes) // Keep as bytes for WriteFile

	// Load module metadata to verify and get its directory
	if app.moduleManager == nil || app.moduleManager.GetStore() == nil {
		app.logger.Error("Module manager or store not initialized for save template")
		http.Error(w, "Internal Server Error - Configuration Error", http.StatusInternalServerError)
		return
	}
	module, err := app.moduleManager.GetStore().LoadModule(moduleID)
	if err != nil {
		app.logger.Error("Failed to load module for save template", "moduleID", moduleID, "filename", filename, "error", err)
		http.NotFound(w, r) // Module not found
		return
	}

	// Construct the path to the template file
	var moduleBasePath string
	if filepath.IsAbs(module.Directory) {
		moduleBasePath = module.Directory
	} else {
		moduleBasePath = filepath.Join(app.projectRoot, module.Directory)
	}
	templateFilePath := filepath.Join(moduleBasePath, "templates", filename)

	// Ensure the filename is valid and part of the module's templates
	foundInMeta := false
	for _, tmplMeta := range module.Templates {
		if tmplMeta.Name == filename {
			foundInMeta = true
			break
		}
	}
	if !foundInMeta {
		app.logger.Warn("Attempt to save to a filename not listed in module metadata", "moduleID", moduleID, "filename", filename)
		http.Error(w, "Bad Request - Invalid filename for module", http.StatusBadRequest)
		return
	}

	// Write the new content to the file
	// Use 0666 for broader permissions, though 0644 is often standard.
	// Consider making this configurable or more restrictive based on needs.
	err = os.WriteFile(templateFilePath, newContentBytes, 0666)
	if err != nil {
		app.logger.Error("Failed to write template file", "moduleID", moduleID, "filename", filename, "path", templateFilePath, "error", err)
		http.Error(w, "Internal Server Error - Failed to save file", http.StatusInternalServerError)
		return
	}

	// Update in-memory edit session if it exists
	// editSessionMutex.Lock() // This was from the old session logic, not needed here for now
	// if session, exists := activeEditSessions[moduleID]; exists {
	// 	session.TemplateContents[filename] = string(newContentBytes) // Update with new content
	// 	session.LastAccessed = time.Now()
	// 	app.logger.Debug("Updated in-memory edit session after save", "moduleID", moduleID, "filename", filename)
	// }
	// editSessionMutex.Unlock()
	// For simplicity, let's not manage the activeEditSessions on save for now.
	// The next time the editor loads this file, it will fetch the updated content from disk.

	// Update the module's LastUpdated timestamp in metadata
	module.LastUpdated = time.Now()
	if err := app.moduleManager.GetStore().SaveModule(module); err != nil {
		app.logger.Error("Failed to update module metadata after saving template", "moduleID", moduleID, "filename", filename, "error", err)
		// This is not a fatal error for the save operation itself, but should be logged.
	}

	app.logger.Info("Successfully saved template file", "moduleID", moduleID, "filename", filename, "path", templateFilePath)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File %s saved successfully.", filename)
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
