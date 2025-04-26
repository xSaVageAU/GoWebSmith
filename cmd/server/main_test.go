package main

import (
	"go-module-builder/internal/model"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestStaticFileHandler(t *testing.T) {
	// --- Setup ---
	// Store original global state and restore later
	originalProjectRoot := projectRoot
	originalIsModuleListEnabled := isModuleListEnabled

	// Set globals needed by createServerMux or handlers it attaches
	projectRoot = getProjectRoot(t) // Set global for the test
	isModuleListEnabled = false     // Set to a known state for the test

	// Restore original globals after test finishes
	t.Cleanup(func() {
		projectRoot = originalProjectRoot
		isModuleListEnabled = originalIsModuleListEnabled
	})

	// Get the mux (router) by calling the function extracted in main.go
	mux := createServerMux()
	if mux == nil {
		t.Fatal("createServerMux returned nil")
	}

	// --- Create Request & Recorder ---
	req := httptest.NewRequest("GET", "/static/test.css", nil)
	rr := httptest.NewRecorder()

	// --- Execute Request ---
	mux.ServeHTTP(rr, req)

	// --- Assertions ---
	// 1. Check Status Code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// 2. Check Content-Type Header
	// Go's default FileServer adds charset=utf-8 for text types
	expectedContentType := "text/css; charset=utf-8"
	if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
		t.Errorf("handler returned wrong content type: got %q want %q",
			ctype, expectedContentType)
	}

	// 3. Check Body Content
	expectedBodyPath := filepath.Join(projectRoot, "web", "static", "test.css")
	expectedBodyBytes, err := os.ReadFile(expectedBodyPath)
	if err != nil {
		t.Fatalf("Failed to read expected body file %s: %v", expectedBodyPath, err)
	}
	expectedBody := string(expectedBodyBytes)

	// Trim whitespace from both actual and expected as files might have trailing newlines etc.
	if body := strings.TrimSpace(rr.Body.String()); body != strings.TrimSpace(expectedBody) {
		t.Errorf("handler returned unexpected body:\nGot:\n%s\nWant:\n%s",
			body, expectedBody)
	}
}

func TestHandleRootRequest(t *testing.T) {
	// --- Setup ---
	// Store original global state and restore later
	originalProjectRoot := projectRoot
	originalIsModuleListEnabled := isModuleListEnabled
	originalBaseTemplates := baseTemplates

	// Set globals needed for this test
	projectRoot = getProjectRoot(t)
	isModuleListEnabled = true
	
	// We need to parse base templates for the root handler to work
	var err error
	templatesPath := filepath.Join(projectRoot, "web", "templates")
	baseTemplates, err = template.ParseFiles(filepath.Join(templatesPath, "layout.html"))
	if err != nil {
		t.Fatalf("Failed to parse templates for test: %v", err)
	}
	
	// Restore original globals after test finishes
	t.Cleanup(func() {
		projectRoot = originalProjectRoot
		isModuleListEnabled = originalIsModuleListEnabled
		baseTemplates = originalBaseTemplates
	})

	// Get the router using our helper function
	mux := createServerMux()
	if mux == nil {
		t.Fatal("createServerMux returned nil")
	}

	// --- Test Standard Request ---
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

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

	mux.ServeHTTP(rrHtmx, reqHtmx)

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
	// Store original global state and restore later
	originalProjectRoot := projectRoot
	originalModuleTemplates := moduleTemplates
	originalBaseTemplates := baseTemplates
	originalLoadedModules := loadedModules

	// Set globals needed for this test
	projectRoot = getProjectRoot(t)
	
	// Parse base templates
	var err error
	templatesPath := filepath.Join(projectRoot, "web", "templates")
	baseTemplates, err = template.ParseFiles(filepath.Join(templatesPath, "layout.html"))
	if err != nil {
		t.Fatalf("Failed to parse templates for test: %v", err)
	}
	
	// Create a test module
	testModule := &model.Module{
		ID:          "test-module-123",
		Name:        "Test Module",
		Status:      "active",
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
		Templates:   []model.Template{
			{Name: "base.html", Path: "templates/base.html", IsBase: true, Order: 0},
			{Name: "content.html", Path: "templates/content.html", IsBase: false, Order: 1},
		},
	}
	
	// Create a mock moduleTemplates map
	moduleTemplates = make(map[string]*template.Template)
	// Clone base templates for the test module
	moduleTemplates[testModule.ID], err = baseTemplates.Clone()
	if err != nil {
		t.Fatalf("Failed to clone base templates: %v", err)
	}
	
	// Add the test module to loadedModules
	loadedModules = []*model.Module{testModule}
	
	// Restore original globals after test finishes
	t.Cleanup(func() {
		projectRoot = originalProjectRoot
		moduleTemplates = originalModuleTemplates
		baseTemplates = originalBaseTemplates
		loadedModules = originalLoadedModules
	})

	// Get the router using our helper function
	mux := createServerMux()
	if mux == nil {
		t.Fatal("createServerMux returned nil")
	}

	// --- Test Valid Module Request ---
	req := httptest.NewRequest("GET", "/view/module/test-module-123", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

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

	// --- Test HTMX Module Request ---
	reqHtmx := httptest.NewRequest("GET", "/view/module/test-module-123", nil)
	reqHtmx.Header.Add("HX-Request", "true") // Set HTMX header
	rrHtmx := httptest.NewRecorder()

	mux.ServeHTTP(rrHtmx, reqHtmx)

	// Verify HTMX header swap is present
	if !strings.Contains(rrHtmx.Body.String(), `id="module-header-info"`) || 
	   !strings.Contains(rrHtmx.Body.String(), `Test Module`) {
		t.Errorf("HTMX module response missing OOB swap with module name")
	}

	// --- Test Invalid Module ID ---
	reqInvalid := httptest.NewRequest("GET", "/view/module/non-existent", nil)
	rrInvalid := httptest.NewRecorder()

	mux.ServeHTTP(rrInvalid, reqInvalid)

	// Verify 404 for non-existent module
	if status := rrInvalid.Code; status != http.StatusNotFound {
		t.Errorf("Invalid module request returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestHandleModuleStaticRequest(t *testing.T) {
	// --- Setup ---
	// Store original global state and restore later
	originalProjectRoot := projectRoot

	// Set global projectRoot for the test
	projectRoot = getProjectRoot(t)
	
	// Create a test directory structure for module static files
	moduleID := "test-static-module"
	modulePath := filepath.Join(projectRoot, "modules", moduleID)
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
		projectRoot = originalProjectRoot
		os.RemoveAll(modulePath)
	})

	// Get the router using our helper function
	mux := createServerMux()
	if mux == nil {
		t.Fatal("createServerMux returned nil")
	}

	// --- Test Valid Static File Request ---
	req := httptest.NewRequest("GET", "/modules/"+moduleID+"/static/test.css", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

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

	mux.ServeHTTP(rrInvalid, reqInvalid)

	// Verify 404 for non-existent file
	if status := rrInvalid.Code; status != http.StatusNotFound {
		t.Errorf("Invalid static file request returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestHandleModuleListRequest(t *testing.T) {
	// --- Setup ---
	// Store original global state and restore later
	originalProjectRoot := projectRoot
	originalIsModuleListEnabled := isModuleListEnabled
	originalBaseTemplates := baseTemplates
	originalLoadedModules := loadedModules

	// Set globals needed for this test
	projectRoot = getProjectRoot(t)
	isModuleListEnabled = true
	
	// Parse base templates
	var err error
	templatesPath := filepath.Join(projectRoot, "web", "templates")
	baseTemplates, err = template.ParseFiles(filepath.Join(templatesPath, "layout.html"))
	if err != nil {
		t.Fatalf("Failed to parse templates for test: %v", err)
	}
	
	// Create test modules
	activeModule := &model.Module{
		ID:     "active-test-module",
		Name:   "Active Module",
		Status: "active",
	}
	
	removedModule := &model.Module{
		ID:     "removed-test-module",
		Name:   "Removed Module",
		Status: "removed",
	}
	
	// Set loadedModules with both active and removed modules
	loadedModules = []*model.Module{activeModule, removedModule}
	
	// Restore original globals after test finishes
	t.Cleanup(func() {
		projectRoot = originalProjectRoot
		isModuleListEnabled = originalIsModuleListEnabled
		baseTemplates = originalBaseTemplates
		loadedModules = originalLoadedModules
	})

	// Get the router using our helper function
	mux := createServerMux()
	if mux == nil {
		t.Fatal("createServerMux returned nil")
	}

	// --- Test Module List Request ---
	req := httptest.NewRequest("GET", "/modules/list", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

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
