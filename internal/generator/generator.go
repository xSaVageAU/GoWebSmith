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

// --- Slug Generation ---
var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`) // For slugs, allow only lowercase alphanum and hyphen
var multiHyphen = regexp.MustCompile(`-+`)             // To collapse multiple hyphens

// generateSlug creates a URL-friendly slug from a name.
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = nonAlphanumeric.ReplaceAllString(slug, "-") // Replace non-alphanum with hyphens
	slug = multiHyphen.ReplaceAllString(slug, "-")     // Collapse multiple hyphens
	slug = strings.Trim(slug, "-")                     // Trim leading/trailing hyphens
	if slug == "" {
		// Handle empty slug case, maybe use a default or part of UUID? For now, simple default.
		return "module"
	}
	return slug
}

// --- Package Name Sanitization ---
// Sanitize package name: replace non-alphanumeric with underscore, ensure starts with letter
var nonAlphanumericPkg = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sanitizePackageName(name string) string {
	sanitized := nonAlphanumericPkg.ReplaceAllString(name, "_") // Use the correct regex for package names
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
	// Updated default content for base.html - Wraps style call, uses .Module/.RenderedContent
	const defaultBaseHTMLContent = `{{ define "page" }}
<div class="module-content"> <!-- Added a wrapper div -->
    <style>
        {{/* Pass the .Module field from PageData to the style template */}}
        {{ template "module-style" .Module }}
    </style>
    <h2>Module: {{ .Module.Name }} (ID: {{ .Module.ID }})</h2>
    <p>This is the base template for the module.</p>
    {{/* <p>Status: {{ .Module.Status }}</p>  Removed, using IsActive now */}}
    <!-- Inject pre-rendered content from Go handler -->
    {{ .RenderedContent }}
</div>
{{ end }}`

	// Updated default content for style.css - Uses HTML comments
	const defaultStyleCSSContent = `{{ define "module-style" }}
<!-- Base styles for module {{ .Name }} -->

<!-- Add specific styles for your module below -->

<!--
 * ==== SUB-TEMPLATES STYLES ====
 * CSS for sub-templates will be added below this line
 -->
{{ end }}`

	// --- NEW: Default content for content.html ---
	const defaultContentHTMLContent = `{{ define "content" }}
<!-- Default content for module {{ .Name }} -->
<div class="default-content">
    <p>This is the default content template (content.html).</p>
    <p>Module ID: {{ .ID }}</p>
</div>
{{ end }}`
	// --- END NEW ---

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
			// --- NEW: Add content.html to default files ---
			"content.html": {
				Content: defaultContentHTMLContent,
				SubDir:  "templates",
			},
			// --- END NEW ---
		},
	}
}

// GenerateModuleBoilerplate creates the directory structure and default files for a new module.
// It now accepts an optional customSlug. If empty, the moduleID (UUID) is used as the slug.
// It returns the newly created Module object (with paths populated) or an error.
func GenerateModuleBoilerplate(cfg Config, moduleName, moduleID, customSlug string) (*model.Module, error) { // Add customSlug param
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
		CreatedAt:   now,
		LastUpdated: now,
		IsActive:    true, // Default new modules to active
		Group:       "",
		Layout:      "",
		Assets:      nil,
		Description: "",
		// Slug logic: Use custom slug if provided (and sanitized), otherwise default to moduleID
		Slug: func() string {
			if customSlug != "" {
				// Optionally sanitize the custom slug here if needed
				// For now, assume the user provides a valid slug via the flag
				// return generateSlug(customSlug) // Or just use it directly if flag handles validation
				return customSlug // Using custom slug directly
			}
			return moduleID // Default to UUID if no custom slug
		}(),
		Templates: make([]model.Template, 0, len(cfg.DefaultFiles)),
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

		// --- UPDATED: Metadata creation logic ---
		if fileInfo.SubDir == "templates" {
			isBase := false
			order := 99 // Default high order

			switch filename {
			case "base.html":
				isBase = true
				order = 0
			case "style.css":
				isBase = false // Style is not a base HTML template
				order = 1
			case "content.html":
				isBase = false // Content is not a base HTML template
				order = 2      // Set order for content
			}

			template := model.Template{
				Name:     filename,
				Path:     filepath.Join(fileInfo.SubDir, filename),
				IsBase:   isBase,
				Order:    order,
				IsActive: true, // Default template to active
			}
			newModule.Templates = append(newModule.Templates, template)
		}
		// --- END UPDATED ---
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
