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
	if !module.IsActive { // Check IsActive instead of Status
		return "", fmt.Errorf("cannot preview module %s because it is inactive", moduleID) // Update error message
	}

	// 2. Gather all template file paths for this module using Glob
	templatesDir := filepath.Join(module.Directory, "templates")
	pattern := filepath.Join(templatesDir, "*.[th][mt][lm]l") // Glob for *.html and *.tmpl
	htmlFiles, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("error finding html/tmpl template files for module %s: %w", moduleID, err)
	}

	// Glob specifically for *.css files containing template definitions
	cssPattern := filepath.Join(templatesDir, "*.css")
	cssFiles, err := filepath.Glob(cssPattern)
	if err != nil {
		return "", fmt.Errorf("error finding css template files for module %s: %w", moduleID, err)
	}

	// Combine the file lists
	allTemplateFiles := append(htmlFiles, cssFiles...)

	if len(allTemplateFiles) == 0 {
		return "", fmt.Errorf("no template files (.html, .tmpl, .css) found in %s", templatesDir)
	}

	// 3. Parse all discovered template files together
	// Use the module ID as a base name for the template set for clarity
	tmplSet, err := template.New(moduleID).Funcs(nil).ParseFiles(allTemplateFiles...)
	if err != nil {
		return "", fmt.Errorf("failed to parse templates from %s: %w", templatesDir, err)
	}

	// 4. Execute the main "page" template into a buffer
	var buf bytes.Buffer
	// Pass module data to the template execution
	err = tmplSet.ExecuteTemplate(&buf, "page", module)
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
