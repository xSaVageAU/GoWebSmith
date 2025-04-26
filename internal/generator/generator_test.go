package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateModuleBoilerplate(t *testing.T) {
	// --- Setup ---
	tempDir := t.TempDir() // Create a temporary directory for modules
	modulesDir := tempDir   // Use the temp dir as the base modules directory
	
	moduleName := "Test Module"
	moduleID := "test-uuid-123"
	
	// Create the default configuration
	cfg := DefaultGeneratorConfig(modulesDir)

	// --- Execute ---
	module, err := GenerateModuleBoilerplate(cfg, moduleName, moduleID)
	if err != nil {
		t.Fatalf("GenerateModuleBoilerplate failed: %v", err)
	}

	// --- Verification ---
	// 1. Check if the module was returned
	if module == nil {
		t.Fatal("GenerateModuleBoilerplate returned nil module")
	}

	// 2. Check basic module properties
	if module.ID != moduleID {
		t.Errorf("Module ID mismatch: got %q, want %q", module.ID, moduleID)
	}
	if module.Name != moduleName {
		t.Errorf("Module Name mismatch: got %q, want %q", module.Name, moduleName)
	}
	
	// 3. Check directory structure
	moduleBasePath := filepath.Join(modulesDir, moduleID)
	templatesPath := filepath.Join(moduleBasePath, "templates")
	
	dirsToCheck := []string{moduleBasePath, templatesPath}
	for _, dir := range dirsToCheck {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %q was not created", dir)
		} else if err != nil {
			t.Errorf("Error checking directory %q: %v", dir, err)
		}
	}

	// 4. Check file existence and content
	handlerPath := filepath.Join(moduleBasePath, "handler.go")
	baseHTMLPath := filepath.Join(templatesPath, "base.html") 
	styleCSSPath := filepath.Join(templatesPath, "style.css")
	
	filesToCheck := []struct {
		path        string
		contentCheck func(content string) bool
	}{
		{
			path: handlerPath,
			contentCheck: func(content string) bool {
				return strings.Contains(content, "package test_module") && // Sanitized package name
					   strings.Contains(content, `ModuleName: "Test Module"`) // Module name
			},
		},
		{
			path: baseHTMLPath,
			contentCheck: func(content string) bool {
				return strings.Contains(content, `{{ define "page" }}`) &&
					   strings.Contains(content, `{{ template "module-style" .Module }}`)
			},
		},
		{
			path: styleCSSPath,
			contentCheck: func(content string) bool {
				return strings.Contains(content, `{{ define "module-style" }}`)
			},
		},
	}

	for _, fileCheck := range filesToCheck {
		content, err := os.ReadFile(fileCheck.path)
		if os.IsNotExist(err) {
			t.Errorf("Expected file %q was not created", fileCheck.path)
			continue
		} else if err != nil {
			t.Errorf("Error reading file %q: %v", fileCheck.path, err)
			continue
		}
		
		if !fileCheck.contentCheck(string(content)) {
			t.Errorf("File %q content verification failed. Content: %q", fileCheck.path, string(content))
		}
	}

	// 5. Check template list in the returned module
	expectedTemplates := 2 // base.html and style.css
	if len(module.Templates) != expectedTemplates {
		t.Errorf("Expected %d templates in module, got %d", expectedTemplates, len(module.Templates))
	}
	
	// Check if template names are correct
	templateNames := make(map[string]bool)
	for _, tmpl := range module.Templates {
		templateNames[tmpl.Name] = true
	}
	
	if !templateNames["base.html"] {
		t.Errorf("base.html not found in module templates")
	}
	if !templateNames["style.css"] {
		t.Errorf("style.css not found in module templates")
	}
}

// TestAddTemplateToModule tests the AddTemplateToModule function
func TestAddTemplateToModule(t *testing.T) {
	// --- Setup ---
	tempDir := t.TempDir()
	
	moduleID := "test-module-123"
	moduleTemplatesDir := filepath.Join(tempDir, moduleID, "templates")
	
	// Create module directory structure to mimic an existing module
	if err := os.MkdirAll(moduleTemplatesDir, 0755); err != nil {
		t.Fatalf("Setup failed: Could not create test module directory structure: %v", err)
	}
	
	// Test template name
	templateName := "card.html"
	
	// --- Execute ---
	err := AddTemplateToModule(moduleID, templateName, tempDir)
	if err != nil {
		t.Fatalf("AddTemplateToModule failed: %v", err)
	}
	
	// --- Verification ---
	// 1. Check if template file was created
	templatePath := filepath.Join(moduleTemplatesDir, templateName)
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Errorf("Expected template file %q was not created", templatePath)
	} else if err != nil {
		t.Errorf("Error checking template file %q: %v", templatePath, err)
	}
	
	// 2. Check template content
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Errorf("Error reading template file %q: %v", templatePath, err)
	} else {
		// Verify it contains the expected template define block
		expectedDefine := `{{ define "card" }}`
		if !strings.Contains(string(content), expectedDefine) {
			t.Errorf("Template content missing expected define block. Content: %q", string(content))
		}
		
		// Verify it contains a placeholder div with the template name
		expectedDiv := `<div class="card-template">`
		if !strings.Contains(string(content), expectedDiv) {
			t.Errorf("Template content missing expected div. Content: %q", string(content))
		}
	}
	
	// --- Test Case: Error on non-existent module ---
	nonExistentID := "non-existent-module"
	err = AddTemplateToModule(nonExistentID, templateName, tempDir)
	if err == nil {
		t.Errorf("Expected error for non-existent module, but got nil")
	}
	
	// --- Test Case: Different template name/extension ---
	cssTemplateName := "custom.css"
	err = AddTemplateToModule(moduleID, cssTemplateName, tempDir)
	if err != nil {
		t.Fatalf("AddTemplateToModule with CSS template failed: %v", err)
	}
	
	cssTemplatePath := filepath.Join(moduleTemplatesDir, cssTemplateName)
	cssContent, err := os.ReadFile(cssTemplatePath)
	if err != nil {
		t.Errorf("Error reading CSS template file: %v", err)
	} else {
		// For CSS templates, verify it has the proper define block
		expectedCssDefine := `{{ define "custom" }}`
		if !strings.Contains(string(cssContent), expectedCssDefine) {
			t.Errorf("CSS template content missing expected define block. Content: %q", string(cssContent))
		}
	}
}