package templating

import (
	"go-module-builder/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockDataStore implements the storage.DataStore interface for testing
type mockDataStore struct {
	modules  map[string]*model.Module
	basePath string
}

// LoadModule retrieves a module by ID
func (m *mockDataStore) LoadModule(id string) (*model.Module, error) {
	module, exists := m.modules[id]
	if !exists {
		return nil, os.ErrNotExist
	}
	return module, nil
}

// SaveModule implementation for the mock
func (m *mockDataStore) SaveModule(module *model.Module) error {
	m.modules[module.ID] = module
	return nil
}

// GetAllModuleIDs returns all module IDs
func (m *mockDataStore) GetAllModuleIDs() ([]string, error) {
	ids := make([]string, 0, len(m.modules))
	for id := range m.modules {
		ids = append(ids, id)
	}
	return ids, nil
}

// DeleteModule removes a module from the mock store
func (m *mockDataStore) DeleteModule(moduleID string) error {
	delete(m.modules, moduleID)
	return nil
}

// ReadAll returns all modules
func (m *mockDataStore) ReadAll() ([]*model.Module, error) {
	modules := make([]*model.Module, 0, len(m.modules))
	for _, module := range m.modules {
		modules = append(modules, module)
	}
	return modules, nil
}

// GetBasePath returns the storage base path
func (m *mockDataStore) GetBasePath() string {
	return m.basePath
}

func TestCombineTemplates(t *testing.T) {
	// Create a temporary directory for our test module
	tempDir := t.TempDir()

	// Create test module with necessary structure
	moduleID := "test-module-123"
	moduleDir := filepath.Join(tempDir, moduleID)
	templatesDir := filepath.Join(moduleDir, "templates")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create a base.html template file with a "page" template
	baseHTML := `{{ define "page" }}
<div class="module-page">
	<h1>{{ .Name }}</h1>
	{{ template "content" . }}
	<style>{{ template "module-style" . }}</style>
</div>
{{ end }}`

	// Create a content.html template file with a "content" template
	contentHTML := `{{ define "content" }}
<div class="module-content">
	<p>This is the content area for {{ .ID }}</p>
	<p>Active: {{ .IsActive }}</p> {{/* Changed from Status */}}
</div>
{{ end }}`

	// Create a style.css template file with a "module-style" template
	styleCSS := `{{ define "module-style" }}
.module-page {
	background-color: #f5f5f5;
	padding: 20px;
}
.module-content {
	border: 1px solid #ddd;
}
{{ end }}`

	// Write test template files
	if err := os.WriteFile(filepath.Join(templatesDir, "base.html"), []byte(baseHTML), 0644); err != nil {
		t.Fatalf("Failed to write base.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "content.html"), []byte(contentHTML), 0644); err != nil {
		t.Fatalf("Failed to write content.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "style.css"), []byte(styleCSS), 0644); err != nil {
		t.Fatalf("Failed to write style.css: %v", err)
	}

	// Create mock module data
	mockModule := &model.Module{
		ID:        moduleID,
		Name:      "Test Module",
		Directory: moduleDir,
		IsActive:  true, // Changed from Status: "active"
		CreatedAt: time.Now(),
		Templates: []model.Template{
			{Name: "base.html", Path: "templates/base.html"},
			{Name: "content.html", Path: "templates/content.html"},
			{Name: "style.css", Path: "templates/style.css"},
		},
	}

	// Create mock data store with our test module
	mockStore := &mockDataStore{
		modules: map[string]*model.Module{
			moduleID: mockModule,
		},
	}

	// Create the engine with our mock store
	engine := NewEngine(mockStore)

	// Execute the test
	result, err := engine.CombineTemplates(moduleID)
	if err != nil {
		t.Fatalf("CombineTemplates failed: %v", err)
	}

	// Verify the result contains expected content from all templates
	expectedStrings := []string{
		`<h1>Test Module</h1>`,
		`<p>This is the content area for test-module-123</p>`,
		`<p>Active: true</p>`, // Changed from Status: active
		`.module-page {`,
		`.module-content {`,
	}

	for _, str := range expectedStrings {
		if !strings.Contains(result, str) {
			t.Errorf("Expected result to contain %q, but it doesn't.\nResult: %s", str, result)
		}
	}
}

func TestCombineTemplates_RemovedModule(t *testing.T) {
	// Create a mock data store with a removed module
	moduleID := "removed-module-456"
	mockStore := &mockDataStore{
		modules: map[string]*model.Module{
			moduleID: {
				ID:       moduleID,
				IsActive: false, // Changed from Status: "removed"
			},
		},
	}

	// Create the engine with our mock store
	engine := NewEngine(mockStore)

	// Execute the test
	_, err := engine.CombineTemplates(moduleID)

	// Verify we get the expected error
	if err == nil {
		t.Fatal("Expected error for removed module, but got nil")
	}

	expectedErrMsg := "cannot preview module removed-module-456 because it is inactive" // Updated error message check
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain %q, but got: %v", expectedErrMsg, err)
	}
}

func TestCombineTemplates_NoPageTemplate(t *testing.T) {
	// Create a temporary directory for our test module
	tempDir := t.TempDir()

	// Create test module with necessary structure
	moduleID := "no-page-template-789"
	moduleDir := filepath.Join(tempDir, moduleID)
	templatesDir := filepath.Join(moduleDir, "templates")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create a template file WITHOUT a "page" template definition
	contentHTML := `{{ define "content" }}
<div class="module-content">
	<p>No page template defined</p>
</div>
{{ end }}`

	// Write test template file
	if err := os.WriteFile(filepath.Join(templatesDir, "content.html"), []byte(contentHTML), 0644); err != nil {
		t.Fatalf("Failed to write content.html: %v", err)
	}

	// Create mock module data
	mockModule := &model.Module{
		ID:        moduleID,
		Name:      "No Page Template Module",
		Directory: moduleDir,
		IsActive:  true, // Changed from Status: "active"
		Templates: []model.Template{
			{Name: "content.html", Path: "templates/content.html"},
		},
	}

	// Create mock data store with our test module
	mockStore := &mockDataStore{
		modules: map[string]*model.Module{
			moduleID: mockModule,
		},
	}

	// Create the engine with our mock store
	engine := NewEngine(mockStore)

	// Execute the test
	_, err := engine.CombineTemplates(moduleID)

	// Verify we get the expected error about missing "page" template
	if err == nil {
		t.Fatal("Expected error for missing page template, but got nil")
	}

	// Check for either of the possible error messages about undefined templates
	expectedErrMsg1 := "failed to execute template 'page'"
	expectedErrMsg2 := "is undefined"
	if !strings.Contains(err.Error(), expectedErrMsg1) || !strings.Contains(err.Error(), expectedErrMsg2) {
		t.Errorf("Expected error message to contain %q and %q, but got: %v", expectedErrMsg1, expectedErrMsg2, err)
	}
}

func TestCombineTemplates_ModuleNotFound(t *testing.T) {
	// Create a mock data store with no modules
	mockStore := &mockDataStore{
		modules: map[string]*model.Module{},
	}

	// Create the engine with our mock store
	engine := NewEngine(mockStore)

	// Execute the test with a non-existent module ID
	_, err := engine.CombineTemplates("non-existent-id")

	// Verify we get the expected error
	if err == nil {
		t.Fatal("Expected error for non-existent module, but got nil")
	}

	expectedErrMsg := "failed to load module"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain %q, but got: %v", expectedErrMsg, err)
	}
}
