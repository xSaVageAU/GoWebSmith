package storage

import (
	"errors" // Added for errors.Is
	"go-module-builder/internal/model"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Helper function to create a sample module for testing
func createSampleModule(id, name string) *model.Module {
	now := time.Now()
	return &model.Module{
		ID:          id,
		Name:        name,
		Directory:   filepath.Join("modules", id), // Example path
		Status:      "active",
		CreatedAt:   now,
		LastUpdated: now,
		Templates: []model.Template{
			{Name: "base.html", Path: "templates/base.html", IsBase: true, Order: 0},
			{Name: "style.css", Path: "templates/style.css", IsBase: false, Order: 1},
		},
	}
}

func TestNewJSONStore(t *testing.T) {
	tempDir := t.TempDir() // Creates a temporary directory for the test
	metadataPath := filepath.Join(tempDir, ".test_metadata")

	store, err := NewJSONStore(metadataPath)
	if err != nil {
		t.Fatalf("NewJSONStore() failed: %v", err)
	}

	if store == nil {
		t.Fatal("NewJSONStore() returned nil store")
	}

	// Check if the base directory was created
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Errorf("NewJSONStore() did not create the base directory: %s", metadataPath)
	}

	// Check GetBasePath
	if store.GetBasePath() != metadataPath {
		t.Errorf("GetBasePath() returned %q, want %q", store.GetBasePath(), metadataPath)
	}
}

func TestSaveLoadModule(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := filepath.Join(tempDir, ".test_metadata")
	store, err := NewJSONStore(metadataPath)
	if err != nil {
		t.Fatalf("NewJSONStore() failed: %v", err)
	}

	moduleID := "test-save-load-123"
	originalModule := createSampleModule(moduleID, "SaveLoad Test Module")

	// Test Save
	err = store.SaveModule(originalModule)
	if err != nil {
		t.Fatalf("SaveModule() failed: %v", err)
	}

	// Check if file exists
	expectedFilePath := filepath.Join(metadataPath, moduleID+".json")
	if _, err := os.Stat(expectedFilePath); os.IsNotExist(err) {
		t.Fatalf("SaveModule() did not create the expected file: %s", expectedFilePath)
	}

	// Test Load
	loadedModule, err := store.LoadModule(moduleID)
	if err != nil {
		t.Fatalf("LoadModule() failed: %v", err)
	}

	// Compare loaded module with original
	// Need to be careful with time comparison due to potential precision differences
	originalModule.CreatedAt = originalModule.CreatedAt.Truncate(time.Second)
	originalModule.LastUpdated = originalModule.LastUpdated.Truncate(time.Second)
	loadedModule.CreatedAt = loadedModule.CreatedAt.Truncate(time.Second)
	loadedModule.LastUpdated = loadedModule.LastUpdated.Truncate(time.Second)

	if !reflect.DeepEqual(originalModule, loadedModule) {
		t.Errorf("LoadModule() loaded module does not match original.\nOriginal: %+v\nLoaded:   %+v", originalModule, loadedModule)
	}
}

func TestLoadModule_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := filepath.Join(tempDir, ".test_metadata")
	store, err := NewJSONStore(metadataPath)
	if err != nil {
		t.Fatalf("NewJSONStore() failed: %v", err)
	}

	nonExistentID := "does-not-exist-456"
	_, err = store.LoadModule(nonExistentID)

	if err == nil {
		t.Fatalf("LoadModule() succeeded for non-existent ID %s, expected error", nonExistentID)
	}

	// Use errors.Is to check if the error (or any wrapped error) is os.ErrNotExist
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadModule() returned error %q, expected an error wrapping os.ErrNotExist", err)
	}
}

func TestDeleteModule(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := filepath.Join(tempDir, ".test_metadata")
	store, err := NewJSONStore(metadataPath)
	if err != nil {
		t.Fatalf("NewJSONStore() failed: %v", err)
	}

	moduleID := "test-delete-789"
	moduleToDelete := createSampleModule(moduleID, "Delete Test Module")

	// Save it first
	err = store.SaveModule(moduleToDelete)
	if err != nil {
		t.Fatalf("Setup failed: SaveModule() failed: %v", err)
	}

	// Check file exists before delete
	expectedFilePath := filepath.Join(metadataPath, moduleID+".json")
	if _, err := os.Stat(expectedFilePath); os.IsNotExist(err) {
		t.Fatalf("Setup failed: Module file %s was not created before delete", expectedFilePath)
	}

	// Test Delete
	err = store.DeleteModule(moduleID)
	if err != nil {
		t.Fatalf("DeleteModule() failed: %v", err)
	}

	// Check file is gone
	if _, err := os.Stat(expectedFilePath); err == nil {
		t.Fatalf("DeleteModule() did not remove the file: %s", expectedFilePath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("Error checking for deleted file %s: %v", expectedFilePath, err)
	}

	// Try loading it, should fail
	_, err = store.LoadModule(moduleID)
	if err == nil {
		t.Fatalf("LoadModule() succeeded after DeleteModule(), expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadModule() after delete returned error %q, expected an error wrapping os.ErrNotExist", err)
	}
}

func TestReadAll(t *testing.T) {
	tempDir := t.TempDir()
	metadataPath := filepath.Join(tempDir, ".test_metadata")
	store, err := NewJSONStore(metadataPath)
	if err != nil {
		t.Fatalf("NewJSONStore() failed: %v", err)
	}

	// Create and save multiple modules
	module1 := createSampleModule("mod1", "Module One")
	module2 := createSampleModule("mod2", "Module Two")
	module3 := createSampleModule("mod3", "Module Three")

	modulesToSave := []*model.Module{module1, module2, module3}
	savedModulesMap := make(map[string]*model.Module)

	for _, mod := range modulesToSave {
		if err := store.SaveModule(mod); err != nil {
			t.Fatalf("Setup failed: SaveModule() failed for %s: %v", mod.ID, err)
		}
		// Truncate time for comparison later
		mod.CreatedAt = mod.CreatedAt.Truncate(time.Second)
		mod.LastUpdated = mod.LastUpdated.Truncate(time.Second)
		savedModulesMap[mod.ID] = mod
	}

	// Test ReadAll
	loadedModules, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() failed: %v", err)
	}

	if len(loadedModules) != len(modulesToSave) {
		t.Fatalf("ReadAll() returned %d modules, want %d", len(loadedModules), len(modulesToSave))
	}

	// Verify the content of loaded modules
	for _, loadedMod := range loadedModules {
		originalMod, ok := savedModulesMap[loadedMod.ID]
		if !ok {
			t.Errorf("ReadAll() loaded unexpected module ID: %s", loadedMod.ID)
			continue
		}

		// Truncate time for comparison
		loadedMod.CreatedAt = loadedMod.CreatedAt.Truncate(time.Second)
		loadedMod.LastUpdated = loadedMod.LastUpdated.Truncate(time.Second)

		if !reflect.DeepEqual(originalMod, loadedMod) {
			t.Errorf("ReadAll() loaded module %s does not match original.\nOriginal: %+v\nLoaded:   %+v", loadedMod.ID, originalMod, loadedMod)
		}
	}
}
