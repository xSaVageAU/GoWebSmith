package main

import (
	"go-module-builder/internal/model"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync" // Added for application struct
	"testing"
	"time"
)

// Helper function to get project root reliably from test file location
func getProjectRoot(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	// Go up two levels from cmd/server to reach the project root
	return filepath.Dir(filepath.Dir(wd))
}

// Helper to create a minimal valid application instance for testing
func newTestApplication(t *testing.T) *application {
	projRoot := getProjectRoot(t)

	// Parse minimal base template needed for most tests
	templatesPath := filepath.Join(projRoot, "web", "templates")
	baseTmpl, err := template.ParseFiles(filepath.Join(templatesPath, "layout.html"))
	if err != nil {
		t.Fatalf("Failed to parse base layout template for test setup: %v", err)
	}

	return &application{
		projectRoot:          projRoot,
		isModuleListEnabled:  false, // Default, override in specific tests if needed
		loadedModules:        make([]*model.Module, 0),
		baseTemplates:        baseTmpl,
		moduleTemplates:      make(map[string]*template.Template),
		moduleTemplatesMutex: sync.RWMutex{}, // Initialize mutex
	}
}

func TestStaticFileHandler(t *testing.T) {
	// --- Setup ---
	app := newTestApplication(t) // Use helper to create app instance

	// Get the router by calling the method on the app instance
	router := app.routes() // Use the new method name
	if router == nil {
		t.Fatal("app.routes returned nil")
	}

	// --- Create Request & Recorder ---
	req := httptest.NewRequest("GET", "/static/test.css", nil)
	rr := httptest.NewRecorder()

	// --- Execute Request ---
	router.ServeHTTP(rr, req) // Serve using the router

	// --- Assertions ---
	// 1. Check Status Code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// 2. Check Content-Type Header
	expectedContentType := "text/css; charset=utf-8"
	if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
		t.Errorf("handler returned wrong content type: got %q want %q",
			ctype, expectedContentType)
	}

	// 3. Check Body Content
	// Use app.projectRoot now
	expectedBodyPath := filepath.Join(app.projectRoot, "web", "static", "test.css")
	expectedBodyBytes, err := os.ReadFile(expectedBodyPath)
	if err != nil {
		t.Fatalf("Failed to read expected body file %s: %v", expectedBodyPath, err)
	}
	expectedBody := string(expectedBodyBytes)

	if body := strings.TrimSpace(rr.Body.String()); body != strings.TrimSpace(expectedBody) {
		t.Errorf("handler returned unexpected body:\nGot:\n%s\nWant:\n%s",
			body, expectedBody)
	}
}

func TestHandleRootRequest(t *testing.T) {
	// --- Setup ---
	app := newTestApplication(t)
	app.isModuleListEnabled = true // Override default for this test

	// Base templates are parsed in newTestApplication helper

	// Get the router using the method
	router := app.routes() // Use the new method name
	if router == nil {
		t.Fatal("app.routes returned nil")
	}

	// --- Test Standard Request ---
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req) // Serve using the router

	// Verify response code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Root handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Verify content type
	expectedContentType := "text/html; charset=utf-8"
	if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
		t.Errorf("Root handler returned wrong content type: got %q want %q",
			ctype, expectedContentType)
	}

	// Verify layout was rendered (basic check)
	if !strings.Contains(rr.Body.String(), "<html") || !strings.Contains(rr.Body.String(), "<body") {
		t.Errorf("Root handler response doesn't appear to be a complete HTML page")
	}

	// --- Test HTMX Request ---
	reqHtmx := httptest.NewRequest("GET", "/", nil)
	reqHtmx.Header.Add("HX-Request", "true") // Set HTMX header
	rrHtmx := httptest.NewRecorder()

	router.ServeHTTP(rrHtmx, reqHtmx) // Serve using the router

	// Verify response code
	if status := rrHtmx.Code; status != http.StatusOK {
		t.Errorf("Root handler (HTMX) returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// For HTMX requests, we should get a fragment, not a full HTML page
	if strings.Contains(rrHtmx.Body.String(), "<html") || strings.Contains(rrHtmx.Body.String(), "<body") {
		t.Errorf("HTMX root handler response shouldn't contain full HTML structure")
	}

	// Verify the OOB swap for clearing module header
	if !strings.Contains(rrHtmx.Body.String(), `id="module-header-info"`) {
		t.Errorf("HTMX response missing OOB swap for module header")
	}
}

func TestHandleModulePageRequest(t *testing.T) {
	// --- Setup ---
	app := newTestApplication(t)
	// Base templates parsed in helper

	// Create a test module
	testModule := &model.Module{
		ID:          "test-module-123",
		Name:        "Test Module",
		Slug:        "test-module-slug", // Add slug for testing
		IsActive:    true,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
		Templates: []model.Template{
			// Mock template definitions needed for rendering logic
			{Name: "base.html", Path: "templates/base.html", IsBase: true, Order: 0},
			{Name: "content.html", Path: "templates/content.html", IsBase: false, Order: 1},
			{Name: "widget.tmpl", Path: "templates/widget.tmpl", IsBase: false, Order: 2},
		},
	}

	// Add the test module to the app's loadedModules
	app.loadedModules = []*model.Module{testModule}

	// Create a mock module template set by cloning base and adding mock definitions
	// Note: We aren't actually parsing module files here, just setting up the map entry
	// and ensuring the base layout exists in the cloned set.
	clonedTemplates, err := app.baseTemplates.Clone()
	if err != nil {
		t.Fatalf("Failed to clone base templates: %v", err)
	}
	// Add dummy definitions for the templates listed in the model, so ExecuteTemplate doesn't fail immediately
	// In a real scenario with file parsing, these would be defined by {{define "name"}}
	_, err = clonedTemplates.Parse(`{{define "content"}}<div>Mock Content</div>{{end}}`)
	if err != nil {
		t.Fatalf("Failed to parse mock content template: %v", err)
	}
	_, err = clonedTemplates.Parse(`{{define "widget"}}<span>Mock Widget</span>{{end}}`)
	if err != nil {
		t.Fatalf("Failed to parse mock widget template: %v", err)
	}

	app.moduleTemplates[testModule.ID] = clonedTemplates

	// Get the router
	router := app.routes() // Use the new method name
	if router == nil {
		t.Fatal("app.routes returned nil")
	}

	// --- Test Valid Module Request ---
	req := httptest.NewRequest("GET", "/test-module-slug", nil) // Use slug in path
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req) // Serve using the router

	// Verify response code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Module page handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Verify content type
	expectedContentType := "text/html; charset=utf-8"
	if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
		t.Errorf("Module page handler returned wrong content type: got %q want %q",
			ctype, expectedContentType)
	}
	// Check if the rendered content contains parts of the mock templates
	bodyStr := rr.Body.String()
	if !strings.Contains(bodyStr, "Mock Content") || !strings.Contains(bodyStr, "Mock Widget") {
		t.Errorf("Module page response body does not contain expected rendered sub-template content. Got: %s", bodyStr)
	}

	// --- Test HTMX Module Request ---
	reqHtmx := httptest.NewRequest("GET", "/test-module-slug", nil) // Use slug in path
	reqHtmx.Header.Add("HX-Request", "true")                        // Set HTMX header
	rrHtmx := httptest.NewRecorder()

	router.ServeHTTP(rrHtmx, reqHtmx) // Serve using the router

	// Verify HTMX header swap is present
	htmxBodyStr := rrHtmx.Body.String()
	if !strings.Contains(htmxBodyStr, `id="module-header-info"`) ||
		!strings.Contains(htmxBodyStr, `Test Module`) {
		t.Errorf("HTMX module response missing OOB swap with module name")
	}
	// Check if the rendered content contains parts of the mock templates
	if !strings.Contains(htmxBodyStr, "Mock Content") || !strings.Contains(htmxBodyStr, "Mock Widget") {
		t.Errorf("HTMX module response body does not contain expected rendered sub-template content. Got: %s", htmxBodyStr)
	}

	// --- Test Invalid Module Slug ---
	reqInvalid := httptest.NewRequest("GET", "/non-existent-slug", nil) // Use different slug
	rrInvalid := httptest.NewRecorder()

	router.ServeHTTP(rrInvalid, reqInvalid) // Serve using the router

	// Verify 404 for non-existent slug
	if status := rrInvalid.Code; status != http.StatusNotFound {
		t.Errorf("Invalid module slug request returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestHandleModuleStaticRequest(t *testing.T) {
	// --- Setup ---
	app := newTestApplication(t) // Use helper

	// Create a test directory structure for module static files
	moduleID := "test-static-module"
	// Use app.projectRoot
	modulePath := filepath.Join(app.projectRoot, "modules", moduleID)
	moduleTemplatesPath := filepath.Join(modulePath, "templates")
	staticFilePath := filepath.Join(moduleTemplatesPath, "test.css")

	// Create test directories if they don't exist
	if err := os.MkdirAll(moduleTemplatesPath, 0755); err != nil {
		t.Fatalf("Failed to create test module directories: %v", err)
	}

	// Create a test static file
	staticContent := "body { background-color: #f0f0f0; }"
	if err := os.WriteFile(staticFilePath, []byte(staticContent), 0644); err != nil {
		t.Fatalf("Failed to create test static file: %v", err)
	}

	// Clean up test files after test completes
	t.Cleanup(func() {
		// Only remove the test directory, no need to restore globals
		os.RemoveAll(modulePath)
	})

	// Get the router
	router := app.routes() // Use the new method name
	if router == nil {
		t.Fatal("app.routes returned nil")
	}

	// --- Test Valid Static File Request ---
	req := httptest.NewRequest("GET", "/modules/"+moduleID+"/static/test.css", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req) // Serve using the router

	// Verify response code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Module static handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Verify content type
	expectedContentType := "text/css; charset=utf-8"
	if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
		t.Errorf("Module static handler returned wrong content type: got %q want %q",
			ctype, expectedContentType)
	}

	// Verify file content
	if body := strings.TrimSpace(rr.Body.String()); body != staticContent {
		t.Errorf("Module static handler returned wrong content: got %q want %q",
			body, staticContent)
	}

	// --- Test Non-existent Static File Request ---
	reqInvalid := httptest.NewRequest("GET", "/modules/"+moduleID+"/static/non-existent.css", nil)
	rrInvalid := httptest.NewRecorder()

	router.ServeHTTP(rrInvalid, reqInvalid) // Serve using the router

	// Verify 404 for non-existent file
	if status := rrInvalid.Code; status != http.StatusNotFound {
		t.Errorf("Invalid static file request returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestHandleModuleListRequest(t *testing.T) {
	// --- Setup ---
	app := newTestApplication(t)
	app.isModuleListEnabled = true // Enable list for this test
	// Base templates parsed in helper

	// Create test modules
	activeModule := &model.Module{
		ID:       "active-test-module",
		Name:     "Active Module",
		IsActive: true,
	}
	removedModule := &model.Module{
		ID:       "removed-test-module",
		Name:     "Removed Module",
		IsActive: false,
	}

	// Set loadedModules on the app instance
	app.loadedModules = []*model.Module{activeModule, removedModule}

	// Get the router
	router := app.routes() // Use the new method name
	if router == nil {
		t.Fatal("app.routes returned nil")
	}

	// --- Test Module List Request ---
	req := httptest.NewRequest("GET", "/modules/list", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req) // Serve using the router

	// Verify response code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Module list handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Verify content type
	expectedContentType := "text/html; charset=utf-8"
	if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
		t.Errorf("Module list handler returned wrong content type: got %q want %q",
			ctype, expectedContentType)
	}

	// Verify active module is included
	if !strings.Contains(rr.Body.String(), "Active Module") {
		t.Errorf("Module list response should contain active module name")
	}

	// Verify removed module is NOT included
	if strings.Contains(rr.Body.String(), "Removed Module") {
		t.Errorf("Module list response should not contain removed module name")
	}
}
