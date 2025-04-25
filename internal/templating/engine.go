package templating

import (
	"bytes"
	"fmt"
	"go-module-builder/internal/storage"
	"html/template"
	"path/filepath"
	"strings" // Added for error message check
)

// Engine handles template parsing and execution.
type Engine struct {
	store storage.DataStore
}

// NewEngine creates a new template engine.
func NewEngine(store storage.DataStore) *Engine {
	return &Engine{store: store}
}

// CombineTemplates loads, parses, and executes the templates for a given module
// to generate a single HTML string output, executing the "page" template.
func (e *Engine) CombineTemplates(moduleID string) (string, error) {
	// 1. Load module metadata
	module, err := e.store.LoadModule(moduleID)
	if err != nil {
		return "", fmt.Errorf("failed to load module %s: %w", moduleID, err)
	}

	if module.Status == "removed" {
		return "", fmt.Errorf("cannot preview module %s because it is marked as removed", moduleID)
	}

	// 2. Gather all template file paths for this module
	var templateFiles []string
	for _, tmpl := range module.Templates {
		// Construct absolute path based on module directory
		absPath := filepath.Join(module.Directory, tmpl.Path)
		templateFiles = append(templateFiles, absPath)
	}

	if len(templateFiles) == 0 {
		return "", fmt.Errorf("no template files found for module %s", moduleID)
	}

	// 3. Parse all template files together
	// We use the module ID as a base name for the template set for clarity
	// Funcs(nil) is added just in case we need functions later
	tmplSet, err := template.New(moduleID).Funcs(nil).ParseFiles(templateFiles...)
	if err != nil {
		return "", fmt.Errorf("failed to parse templates for module %s: %w", moduleID, err)
	}

	// 4. Execute the main "page" template into a buffer
	var buf bytes.Buffer
	// We execute the template named "page" by convention (from base.html)
	// Pass a simple map containing the ModuleName for now, as the template uses it.
	executionData := map[string]string{
		"ModuleName": module.Name,
	}
	err = tmplSet.ExecuteTemplate(&buf, "page", executionData)
	if err != nil {
		// Check if the error message indicates the template wasn't defined
		if strings.Contains(err.Error(), "template \"page\" is undefined") || strings.Contains(err.Error(), "template \"page\" not defined") {
			return "", fmt.Errorf("failed to execute template for module %s: the main template file (usually base.html) must contain '{{ define \"page\" }} ... {{ end }}'", moduleID)
		}
		return "", fmt.Errorf("failed to execute template 'page' for module %s: %w", moduleID, err)
	}

	// 5. Return the resulting HTML string
	return buf.String(), nil
}
