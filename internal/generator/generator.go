package generator

import (
	"fmt"
	"go-module-builder/internal/model"
	"go-module-builder/pkg/fsutils"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Config holds the configuration for module generation.
type Config struct {
	BaseDir      string                 // Base directory where module folders are created (e.g., "modules")
	SubDirs      []string               // Subdirectories to create within each module folder
	DefaultFiles map[string]FileContent // Map of filename to its content and target subdir
}

// FileContent defines the content and target subdirectory for a default file.
type FileContent struct {
	Content string
	SubDir  string // Relative path from the module root (e.g., "templates", "")
}

// Sanitize package name: replace non-alphanumeric with underscore, ensure starts with letter
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sanitizePackageName(name string) string {
	sanitized := nonAlphanumeric.ReplaceAllString(name, "_")
	sanitized = strings.ToLower(sanitized)
	if len(sanitized) == 0 || (sanitized[0] >= '0' && sanitized[0] <= '9') {
		sanitized = "module_" + sanitized
	}
	// Ensure it's a valid Go identifier (basic check)
	if len(sanitized) > 0 && (sanitized[0] == '_' || (sanitized[0] >= 'a' && sanitized[0] <= 'z')) {
		// Potentially add more checks if needed
	} else {
		sanitized = "module_" + sanitized // Fallback prefix
	}
	return sanitized
}

// DefaultGeneratorConfig provides a standard configuration for module generation.
func DefaultGeneratorConfig(baseDir string) Config {
	// Updated default content for base.html - Uses PageData fields
	const defaultBaseHTMLContent = `{{ define "page" }}
<style>
    {{/* Pass the .Module field from PageData to the style template */}}
    {{ template "module-style" .Module }}
</style>
<div class="module-content"> <!-- Added a wrapper div -->
    <h2>Module: {{ .Module.Name }} (ID: {{ .Module.ID }})</h2>
    <p>This is the base template for the module.</p>
    <p>Status: {{ .Module.Status }}</p>
    <!-- Inject pre-rendered content from Go handler -->
    {{ .RenderedContent }}
</div>
{{ end }}`

	// Updated default content for style.css - Removed .module-content
	const defaultStyleCSSContent = `{{ define "module-style" }}
/* Base styles for module {{ .Name }} */

/* Add specific styles for your module below */

/* 
 * ==== SUB-TEMPLATES STYLES ====
 * CSS for sub-templates will be added below this line
 */
{{ end }}`

	handlerGoContent := `package {{ .PackageName }} // Dynamic package name

import (
	"fmt"
	"net/http"
)

// ModuleData might hold data passed from the handler to the template
type ModuleData struct {
	ModuleName string
	// Add other fields needed by the template(s)
}

// Handle is the main entry point for this module's HTTP requests.
// The main server needs a mechanism to discover and route to this handler.
func Handle(w http.ResponseWriter, r *http.Request) {
	data := ModuleData{
		ModuleName: "{{ .ModuleName }}", // Use the actual module name
	}

	isHTMX := r.Header.Get("HX-Request") == "true"
	isTargetMain := r.Header.Get("HX-Target") == "main"

	if isHTMX && isTargetMain {
		fmt.Fprintf(w, "<div>HTMX Fragment Response for %s (Render page block here)</div>", data.ModuleName)
	} else {
		fmt.Fprintf(w, "<html><body>Full Page Response for %s (Render layout + page block here)</body></html>", data.ModuleName)
	}
}`

	return Config{
		BaseDir: baseDir,
		SubDirs: []string{"templates"},
		DefaultFiles: map[string]FileContent{
			"handler.go": {
				Content: handlerGoContent,
				SubDir:  "",
			},
			"base.html": {
				Content: defaultBaseHTMLContent,
				SubDir:  "templates",
			},
			"style.css": {
				Content: defaultStyleCSSContent,
				SubDir:  "templates",
			},
		},
	}
}

// GenerateModuleBoilerplate creates the directory structure and default files for a new module.
// It returns the newly created Module object (with paths populated) or an error.
func GenerateModuleBoilerplate(cfg Config, moduleName, moduleID string) (*model.Module, error) {
	if moduleName == "" || moduleID == "" {
		return nil, fmt.Errorf("module name and ID cannot be empty")
	}

	moduleDir := filepath.Join(cfg.BaseDir, moduleID)

	if err := fsutils.CreateDir(moduleDir); err != nil {
		return nil, fmt.Errorf("failed to create module directory %s: %w", moduleDir, err)
	}
	fmt.Printf("Created directory: %s\n", moduleDir)

	for _, subDir := range cfg.SubDirs {
		fullSubDirPath := filepath.Join(moduleDir, subDir)
		if err := fsutils.CreateDir(fullSubDirPath); err != nil {
			return nil, fmt.Errorf("failed to create subdirectory %s: %w", fullSubDirPath, err)
		}
		fmt.Printf("Created directory: %s\n", fullSubDirPath)
	}

	now := time.Now()
	newModule := &model.Module{
		ID:          moduleID,
		Name:        moduleName,
		Directory:   moduleDir,
		Status:      "active",
		CreatedAt:   now,
		LastUpdated: now,
		Templates:   make([]model.Template, 0, len(cfg.DefaultFiles)),
	}

	packageName := sanitizePackageName(moduleName)

	for filename, fileInfo := range cfg.DefaultFiles {
		filePath := filepath.Join(moduleDir, fileInfo.SubDir, filename)

		content := fileInfo.Content
		content = strings.ReplaceAll(content, "{{ .ModuleName }}", moduleName)
		if filename == "handler.go" {
			content = strings.ReplaceAll(content, "{{ .PackageName }}", packageName)
		}

		if err := fsutils.WriteToFile(filePath, []byte(content)); err != nil {
			return nil, fmt.Errorf("failed to create default file %s: %w", filePath, err)
		}
		fmt.Printf("Created file: %s\n", filePath)

		if fileInfo.SubDir == "templates" {
			isBase := (filename == "base.html")
			order := 0
			if filename == "base.html" {
				order = 0
			} else if filename == "style.css" {
				order = 1
			}

			template := model.Template{
				Name:   filename,
				Path:   filepath.Join(fileInfo.SubDir, filename),
				IsBase: isBase,
				Order:  order,
			}
			newModule.Templates = append(newModule.Templates, template)
		}
	}

	return newModule, nil
}

// AddTemplateToModule adds a new template file.
// It no longer modifies base.html or style.css.
// The CLI command is responsible for updating the module's JSON metadata.
func AddTemplateToModule(moduleID, templateName, modulesDir string) error {
	moduleTemplatesDir := filepath.Join(modulesDir, moduleID, "templates")

	templateFileName := strings.TrimSuffix(templateName, filepath.Ext(templateName))
	newTemplateFilePath := filepath.Join(moduleTemplatesDir, templateName)
	newTemplateContent := fmt.Sprintf(`{{ define "%s" }}
<!-- Content for %s -->
<div class="%s-template">
    <p>Placeholder content for %s</p>
</div>
{{ end }}`, templateFileName, templateName, templateFileName, templateName)

	if err := fsutils.WriteToFile(newTemplateFilePath, []byte(newTemplateContent)); err != nil {
		return fmt.Errorf("failed to create template file %s: %w", newTemplateFilePath, err)
	}
	fmt.Printf("Created template file: %s\n", newTemplateFilePath)

	log.Printf("Successfully created template file '%s' for module %s. Remember to update module metadata.", templateName, moduleID)
	return nil
}
