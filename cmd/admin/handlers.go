package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url" // Added for URL encoding error messages
	"os"
	"path/filepath"
	"regexp" // Added for slug validation
	"sort"
	"strings"
	"time"

	"go-module-builder/internal/model" // Import model package

	"github.com/go-chi/chi/v5"
	"github.com/justinas/nosurf"
)

// DashboardPageData holds all data needed for the dashboard template (layout + content)
type DashboardPageData struct {
	CurrentYear           int // Moved here for layout footer
	ActiveInactiveModules []*model.Module
	SoftDeletedModules    []*model.Module
	Error                 string // To display errors if module loading fails
}

// PreviewRequestData defines the structure for the preview API request body.
type PreviewRequestData struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// dashboardHandler serves the main admin dashboard page.
func (app *adminApplication) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r, "dashboard")

	pageData := DashboardPageData{
		CurrentYear:           time.Now().Year(),
		ActiveInactiveModules: make([]*model.Module, 0),
		SoftDeletedModules:    make([]*model.Module, 0),
	}

	if app.moduleStore == nil {
		app.logger.Warn("Module store is not initialized in dashboard handler")
		pageData.Error = "Module storage not available."
	} else {
		modules, err := app.moduleStore.ReadAll()
		if err != nil {
			app.logger.Error("Failed to read modules from store", "error", err)
			pageData.Error = "Failed to load module list."
		} else {
			for _, mod := range modules {
				if !mod.IsActive && strings.Contains(mod.Directory, "modules_removed") {
					pageData.SoftDeletedModules = append(pageData.SoftDeletedModules, mod)
				} else {
					pageData.ActiveInactiveModules = append(pageData.ActiveInactiveModules, mod)
				}
			}
			app.logger.Debug("Processed modules for dashboard", "active/inactive_count", len(pageData.ActiveInactiveModules), "soft_deleted_count", len(pageData.SoftDeletedModules))
		}
	}
	data["Page"] = pageData

	ts, ok := app.templateCache["dashboard.html"]
	if !ok {
		app.logger.Error("Template dashboard.html not found in cache")
		http.Error(w, "Internal Server Error - Template not found", http.StatusInternalServerError)
		return
	}

	err := ts.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		app.logger.Error("Error executing admin layout template", "error", err)
		// Avoid writing header again if already sent (http.Error might panic)
	}
}

// moduleCreateFormHandler displays the form for creating a new module.
func (app *adminApplication) moduleCreateFormHandler(w http.ResponseWriter, r *http.Request) {
	ts, ok := app.templateCache["module_form.html"]
	if !ok {
		app.logger.Error("Template module_form.html not found in cache")
		http.Error(w, "Internal Server Error - Template not found", http.StatusInternalServerError)
		return
	}

	data := app.newTemplateData(r, "create")
	// Ensure CurrentYear is available for layout, as newTemplateData might not set it.
	data["CurrentYear"] = time.Now().Year()

	data["FormError"] = r.URL.Query().Get("error")
	data["ModuleName"] = r.URL.Query().Get("moduleName")
	data["CustomSlug"] = r.URL.Query().Get("customSlug")

	err := ts.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		app.logger.Error("Error executing admin create form layout template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// moduleCreateHandler handles the submission of the new module form.
func (app *adminApplication) moduleCreateHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.logger.Error("Error parsing create module form", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	moduleName := r.PostForm.Get("moduleName")
	customSlug := r.PostForm.Get("customSlug")

	if moduleName == "" {
		errorMsg := "Module Name is required."
		app.FlashErrorMessage = errorMsg
		redirectURL := fmt.Sprintf("/admin/modules/new?moduleName=%s&customSlug=%s",
			url.QueryEscape(moduleName),
			url.QueryEscape(customSlug))
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	if customSlug != "" {
		// Validate customSlug: lowercase letters, numbers, hyphens. Start/end with letter/number.
		isValidSlug, _ := regexp.MatchString(`^[a-z0-9]+(?:-[a-z0-9]+)*$`, customSlug)
		if !isValidSlug {
			app.logger.Warn("Invalid custom slug format provided", "customSlug", customSlug)
			errorMsg := "Invalid Custom Slug format. Use lowercase letters, numbers, and hyphens. Must start and end with a letter or number."
			app.FlashErrorMessage = errorMsg
			redirectURL := fmt.Sprintf("/admin/modules/new?moduleName=%s&customSlug=%s",
				url.QueryEscape(moduleName),
				url.QueryEscape(customSlug))
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
			return
		}
	}

	if app.moduleManager == nil {
		app.logger.Error("ModuleManager not initialized in admin application")
		http.Error(w, "Internal Server Error - Configuration Error", http.StatusInternalServerError)
		return
	}

	createdModule, err := app.moduleManager.CreateModule(moduleName, customSlug)
	if err != nil {
		app.logger.Error("Error creating module via manager", "error", err, "moduleName", moduleName, "customSlug", customSlug)
		app.FlashErrorMessage = fmt.Sprintf("Failed to create module '%s': %v", moduleName, err)
		redirectURL := fmt.Sprintf("/admin/modules/new?moduleName=%s&customSlug=%s",
			url.QueryEscape(moduleName),
			url.QueryEscape(customSlug))
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}
	app.FlashSuccessMessage = fmt.Sprintf("Module '%s' created successfully.", createdModule.Name)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// moduleDeleteHandler handles the submission for deleting a module.
func (app *adminApplication) moduleDeleteHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	if moduleID == "" {
		app.logger.Error("moduleDeleteHandler: Module ID missing from URL")
		errorMessage := "Bad Request - Missing Module ID"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	err := r.ParseForm()
	if err != nil {
		app.logger.Error("moduleDeleteHandler: Error parsing delete module form", "error", err, "moduleID", moduleID)
		errorMessage := "Bad Request - Could not parse form"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}
	forceDelete := r.PostForm.Get("force") == "true"

	if app.moduleManager == nil {
		app.logger.Error("moduleDeleteHandler: ModuleManager not initialized")
		errorMessage := "Internal Server Error - Configuration Error"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	var moduleNameForMessage = moduleID
	// Try to get module name before deletion for a user-friendly message.
	moduleToDelete, loadErr := app.moduleManager.GetStore().LoadModule(moduleID)
	if loadErr == nil {
		moduleNameForMessage = moduleToDelete.Name
	} else {
		app.logger.Warn("moduleDeleteHandler: Could not load module before deletion for name", "moduleID", moduleID, "error", loadErr)
	}

	err = app.moduleManager.DeleteModule(moduleID, forceDelete)
	if err != nil {
		app.logger.Error("moduleDeleteHandler: Error deleting module via manager", "error", err, "moduleID", moduleID, "force", forceDelete)
		errorMessage := fmt.Sprintf("Failed to delete module '%s': %v", moduleNameForMessage, err)
		escapedErrorMessage, _ := json.Marshal(errorMessage)
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": %s, "type": "error"}}`, string(escapedErrorMessage))
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Fetch all modules again to reflect the deletion for the partial update.
	allModules, err := app.moduleStore.ReadAll()
	if err != nil {
		app.logger.Error("moduleDeleteHandler: Module deleted, but failed to read all modules for refresh", "error", err, "moduleID", moduleID)
		// Deletion succeeded, but list refresh for client will fail.
		// Send success for deletion, but warn about list refresh.
		successMessage := ""
		if forceDelete {
			successMessage = fmt.Sprintf("Module '%s' (ID: %s) force deleted. Dashboard list refresh failed.", moduleNameForMessage, moduleID)
		} else {
			successMessage = fmt.Sprintf("Module '%s' (ID: %s) soft-deleted. Dashboard list refresh failed.", moduleNameForMessage, moduleID)
		}
		escapedSuccessMessage, _ := json.Marshal(successMessage)
		// Use "success" type as primary action succeeded, message clarifies the secondary issue.
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": %s, "type": "success"}}`, string(escapedSuccessMessage))
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none") // Don't swap if we can't provide the new list.
		w.WriteHeader(http.StatusOK)
		return
	}

	dashboardPageData := DashboardPageData{
		ActiveInactiveModules: make([]*model.Module, 0),
		SoftDeletedModules:    make([]*model.Module, 0),
	}
	for _, mod := range allModules {
		if !mod.IsActive && strings.Contains(mod.Directory, "modules_removed") {
			dashboardPageData.SoftDeletedModules = append(dashboardPageData.SoftDeletedModules, mod)
		} else {
			dashboardPageData.ActiveInactiveModules = append(dashboardPageData.ActiveInactiveModules, mod)
		}
	}

	partialData := map[string]any{
		"Page":      dashboardPageData,
		"CSRFToken": nosurf.Token(r),
	}

	tmpl, ok := app.templateCache["module_dashboard_lists.html"]
	if !ok {
		app.logger.Error("moduleDeleteHandler: Partial template 'module_dashboard_lists.html' not found in cache")
		errorMessage := "Internal Server Error - UI component missing after delete"
		escapedErrorMessage, _ := json.Marshal(errorMessage)
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": %s, "type": "error"}}`, string(escapedErrorMessage))
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	flashMessage := ""
	if forceDelete {
		flashMessage = fmt.Sprintf("Module '%s' (ID: %s) force deleted successfully.", moduleNameForMessage, moduleID)
	} else {
		flashMessage = fmt.Sprintf("Module '%s' (ID: %s) soft-deleted successfully.", moduleNameForMessage, moduleID)
	}
	successTriggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "success"}}`, flashMessage)
	w.Header().Set("HX-Trigger", successTriggerEvent)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err = tmpl.Execute(w, partialData)
	if err != nil {
		app.logger.Error("moduleDeleteHandler: Error executing dashboard partial template", "error", err)
		// Avoid writing header again if already sent
	}
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
		// TODO: Use a more specific error type from storage if available for not found.
		if err.Error() == "module metadata file not found" || os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Failed to load module data", http.StatusInternalServerError)
		}
		return
	}

	// --- Sort Templates by Order, then Name ---
	if module != nil && module.Templates != nil {
		sort.SliceStable(module.Templates, func(i, j int) bool {
			if module.Templates[i].Order != module.Templates[j].Order {
				return module.Templates[i].Order < module.Templates[j].Order
			}
			// Secondary sort by name if orders are equal
			return module.Templates[i].Name < module.Templates[j].Name
		})
		app.logger.Debug("Sorted templates for editor view", "moduleID", moduleID)
	}

	ts, ok := app.templateCache["module_editor.html"]
	if !ok {
		app.logger.Error("Template module_editor.html not found in cache")
		http.Error(w, "Internal Server Error - Template not found", http.StatusInternalServerError)
		return
	}

	data := app.newTemplateData(r, "edit") // "edit" nav item is for context.
	data["CurrentYear"] = time.Now().Year()
	data["ModuleData"] = module
	// Flash messages (PageError, PageSuccess) are handled by newTemplateData.

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = ts.ExecuteTemplate(w, "layout.html", data) // layout.html is the entry point for cached templates
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

// modulePreviewHandler renders a module preview using potentially modified template content.
// It performs its own template parsing on each request for the preview.
func (app *adminApplication) modulePreviewHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	if moduleID == "" {
		app.logger.Error("modulePreviewHandler: Module ID missing from URL in preview request")
		http.Error(w, "Bad Request - Missing Module ID", http.StatusBadRequest)
		return
	}

	var reqData PreviewRequestData
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		app.logger.Error("modulePreviewHandler: Error decoding preview request body", "error", err, "moduleID", moduleID)
		http.Error(w, "Bad Request - Invalid JSON", http.StatusBadRequest)
		return
	}
	if reqData.Filename == "" {
		app.logger.Error("modulePreviewHandler: Filename missing in preview request body", "moduleID", moduleID)
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

			renderedSubContent, subRenderErr := app.renderAdminPreviewSubTemplates(previewTmplSet, module)
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
// It uses an already parsed template set.
func (app *adminApplication) renderAdminPreviewSubTemplates(previewTmplSet *template.Template, module *model.Module) (string, error) {
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

	newContentBytes, err := io.ReadAll(r.Body)
	if err != nil {
		app.logger.Error("Failed to read request body for save template", "moduleID", moduleID, "filename", filename, "error", err)
		http.Error(w, "Internal Server Error - Failed to read content", http.StatusInternalServerError)
		return
	}
	// newContent := string(newContentBytes) // Keeping as bytes for os.WriteFile

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
		app.logger.Warn("Attempt to save to a filename not listed in module metadata", "moduleID", moduleID, "filename", filename)
		http.Error(w, "Bad Request - Invalid filename for module", http.StatusBadRequest)
		return
	}

	// Using 0666 for file permissions; consider if this needs to be more restrictive.
	err = os.WriteFile(templateFilePath, newContentBytes, 0666)
	if err != nil {
		app.logger.Error("Failed to write template file", "moduleID", moduleID, "filename", filename, "path", templateFilePath, "error", err)
		http.Error(w, "Internal Server Error - Failed to save file", http.StatusInternalServerError)
		return
	}

	// The commented-out activeEditSessions logic was here and has been removed as it's unused.
	// For simplicity, we're not managing in-memory edit sessions on save for now.
	// The next time the editor loads this file, it will fetch the updated content from disk.

	// Update the module's LastUpdated timestamp in metadata
	module.LastUpdated = time.Now()
	if err := app.moduleManager.GetStore().SaveModule(module); err != nil {
		app.logger.Error("Failed to update module metadata after saving template", "moduleID", moduleID, "filename", filename, "error", err)
		// This non-fatal error for the save operation is logged.
	}

	app.logger.Info("Successfully saved template file", "moduleID", moduleID, "filename", filename, "path", templateFilePath)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File %s saved successfully.", filename)
}

// jsonResponse struct was here, removed as it's no longer used after HTMX refactoring.

// moduleAddTemplateHandler handles adding a new template to a module via HTMX.
func (app *adminApplication) moduleAddTemplateHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	if moduleID == "" {
		app.logger.Error("moduleAddTemplateHandler: Module ID missing from URL")
		errorMessage := "Bad Request - Missing Module ID"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK) // Respond 200 OK for HX-Trigger processing.
		return
	}

	if err := r.ParseForm(); err != nil {
		app.logger.Error("moduleAddTemplateHandler: Error parsing form data", "error", err, "moduleID", moduleID)
		errorMessage := "Bad Request - Could not parse form"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	newTemplateName := r.PostForm.Get("new_template_name")
	if newTemplateName == "" {
		app.logger.Warn("moduleAddTemplateHandler: New template name is required", "moduleID", moduleID)
		errorMessage := "New template name cannot be empty."
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.Contains(newTemplateName, "/") || strings.Contains(newTemplateName, "\\") {
		app.logger.Warn("moduleAddTemplateHandler: Invalid characters in template name", "templateName", newTemplateName, "moduleID", moduleID)
		errorMessage := "Template name cannot contain slashes."
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}
	if !strings.HasSuffix(newTemplateName, ".html") && !strings.HasSuffix(newTemplateName, ".css") && !strings.HasSuffix(newTemplateName, ".tmpl") && !strings.HasSuffix(newTemplateName, ".js") {
		app.logger.Warn("moduleAddTemplateHandler: Suspicious template extension", "templateName", newTemplateName, "moduleID", moduleID)
		errorMessage := "Template name should have a common extension (e.g., .html, .css, .tmpl, .js)."
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	if app.moduleManager == nil {
		app.logger.Error("moduleAddTemplateHandler: ModuleManager not initialized")
		errorMessage := "Internal Server Error - Configuration Error"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	addedModule, err := app.moduleManager.AddTemplate(moduleID, newTemplateName)
	if err != nil {
		app.logger.Error("moduleAddTemplateHandler: Error adding template via manager", "error", err, "moduleID", moduleID, "templateName", newTemplateName)
		errorMessage := fmt.Sprintf("Failed to add template: %v", err)
		escapedErrorMessage, _ := json.Marshal(errorMessage) // Ensure message is JSON-safe.
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": %s, "type": "error"}}`, string(escapedErrorMessage))
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	app.logger.Info("moduleAddTemplateHandler: Successfully added template, preparing HTML partial", "moduleID", moduleID, "templateName", newTemplateName)

	partialData := map[string]any{
		"Templates": addedModule.Templates,
		"ModuleID":  moduleID,
		"CSRFToken": nosurf.Token(r),
	}

	tmpl, ok := app.templateCache["template_list_items.html"]
	if !ok {
		app.logger.Error("moduleAddTemplateHandler: Partial template 'template_list_items.html' not found in cache")
		errorMessage := "Internal Server Error - UI component missing"
		escapedErrorMessage, _ := json.Marshal(errorMessage)
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": %s, "type": "error"}}`, string(escapedErrorMessage))
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	successMessage := fmt.Sprintf("Template '%s' added successfully.", newTemplateName)
	triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "success"}}`, successMessage)
	w.Header().Set("HX-Trigger", triggerEvent)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK) // Success
	err = tmpl.Execute(w, partialData)
	if err != nil {
		app.logger.Error("moduleAddTemplateHandler: Error executing template list partial", "error", err)
	}
}

// moduleRemoveTemplateHandler handles removing a template from a module via HTMX.
func (app *adminApplication) moduleRemoveTemplateHandler(w http.ResponseWriter, r *http.Request) {
	moduleID := chi.URLParam(r, "moduleID")
	templateFilename := chi.URLParam(r, "templateFilename")

	if moduleID == "" || templateFilename == "" {
		app.logger.Error("moduleRemoveTemplateHandler: Module ID or Template Filename missing from URL")
		errorMessage := "Bad Request - Missing Module ID or Template Filename"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Ensure it's a POST request (nosurf should handle CSRF token from form body)
	if r.Method != http.MethodPost {
		app.logger.Warn("moduleRemoveTemplateHandler: Invalid request method", "method", r.Method)
		errorMessage := "Method Not Allowed"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	// It's good practice to parse the form to ensure CSRF token is processed by nosurf,
	// even if we don't directly use other form values here.
	if err := r.ParseForm(); err != nil {
		app.logger.Error("moduleRemoveTemplateHandler: Error parsing form for CSRF", "error", err, "moduleID", moduleID)
		errorMessage := "Bad Request - Could not parse form"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	if app.moduleManager == nil {
		app.logger.Error("moduleRemoveTemplateHandler: ModuleManager not initialized")
		errorMessage := "Internal Server Error - Configuration Error"
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "error"}}`, errorMessage)
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	err := app.moduleManager.RemoveTemplateFromModule(moduleID, templateFilename)
	if err != nil {
		app.logger.Error("moduleRemoveTemplateHandler: Error removing template via manager", "error", err, "moduleID", moduleID, "templateFilename", templateFilename)
		errorMessage := fmt.Sprintf("Failed to remove template '%s': %v", templateFilename, err)
		escapedErrorMessage, _ := json.Marshal(errorMessage) // Ensure message is JSON-safe.
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": %s, "type": "error"}}`, string(escapedErrorMessage))
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	updatedModule, loadErr := app.moduleManager.GetStore().LoadModule(moduleID)
	if loadErr != nil {
		app.logger.Error("moduleRemoveTemplateHandler: Failed to reload module after template removal", "error", loadErr, "moduleID", moduleID)
		// Primary action (removal) succeeded, but list refresh for client will fail.
		// Send error message indicating the list couldn't be refreshed.
		errorMessage := fmt.Sprintf("Template '%s' removed, but failed to refresh list for display.", templateFilename)
		escapedErrorMessage, _ := json.Marshal(errorMessage)
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": %s, "type": "error"}}`, string(escapedErrorMessage))
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	app.logger.Info("moduleRemoveTemplateHandler: Successfully removed template, preparing HTML partial", "moduleID", moduleID, "templateFilename", templateFilename)

	partialData := map[string]any{
		"Templates": updatedModule.Templates,
		"ModuleID":  moduleID,
		"CSRFToken": nosurf.Token(r),
	}

	tmpl, ok := app.templateCache["template_list_items.html"]
	if !ok {
		app.logger.Error("moduleRemoveTemplateHandler: Partial template 'template_list_items.html' not found in cache")
		errorMessage := "Internal Server Error - UI component missing on remove"
		escapedErrorMessage, _ := json.Marshal(errorMessage)
		triggerEvent := fmt.Sprintf(`{"showMessage": {"message": %s, "type": "error"}}`, string(escapedErrorMessage))
		w.Header().Set("HX-Trigger", triggerEvent)
		w.Header().Set("HX-Reswap", "none")
		w.WriteHeader(http.StatusOK)
		return
	}

	successMessage := fmt.Sprintf("Template '%s' removed successfully.", templateFilename)
	triggerEvent := fmt.Sprintf(`{"showMessage": {"message": "%s", "type": "success"}}`, successMessage)
	w.Header().Set("HX-Trigger", triggerEvent)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err = tmpl.Execute(w, partialData)
	if err != nil {
		app.logger.Error("moduleRemoveTemplateHandler: Error executing template list partial", "error", err)
	}
}
