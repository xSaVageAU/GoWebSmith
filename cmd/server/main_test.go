package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// Add more tests here later if desired (e.g., TestHandleRootRequest, TestHandleModulePageRequest)
