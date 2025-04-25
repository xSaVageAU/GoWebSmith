package storage

import (
	"encoding/json"
	"fmt"
	"go-module-builder/internal/model"
	"os"
	"path/filepath"
	"strings"
	// "go-module-builder/pkg/fsutils" // Will likely need this later
)

// JSONStore implements the DataStore interface using JSON files.
// It stores module metadata as individual JSON files.
type JSONStore struct {
	// BasePath is the directory where module metadata files (*.json) are stored.
	BasePath string
}

// NewJSONStore creates a new JSONStore instance.
// It ensures the base storage directory exists.
func NewJSONStore(basePath string) (*JSONStore, error) {
	// Use os.MkdirAll for robust directory creation
	err := os.MkdirAll(basePath, 0755) // Use standard permission bits
	if err != nil {
		return nil, fmt.Errorf("failed to create storage directory '%s': %w", basePath, err)
	}
	return &JSONStore{BasePath: basePath}, nil
}

// GetBasePath returns the base path of the JSON store.
func (s *JSONStore) GetBasePath() string {
	return s.BasePath
}

// SaveModule persists the module's metadata to a JSON file.
func (js *JSONStore) SaveModule(module *model.Module) error {
	if module.ID == "" {
		return fmt.Errorf("module ID cannot be empty")
	}
	// We store metadata only; actual template content is in module files.
	// Ensure module.Directory is set correctly before saving.
	filePath := filepath.Join(js.BasePath, module.ID+".json")

	// Use MarshalIndent for readable JSON files
	data, err := json.MarshalIndent(module, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal module %s: %w", module.ID, err)
	}

	// Use os.WriteFile for simpler file writing
	err = os.WriteFile(filePath, data, 0644) // Standard file permissions
	if err != nil {
		return fmt.Errorf("failed to write module file %s: %w", filePath, err)
	}
	fmt.Printf("Placeholder: Saved module metadata to %s\n", filePath) // Placeholder log
	return nil
}

// LoadModule retrieves a module's metadata from its JSON file.
func (js *JSONStore) LoadModule(moduleID string) (*model.Module, error) {
	if moduleID == "" {
		return nil, fmt.Errorf("module ID cannot be empty")
	}
	filePath := filepath.Join(js.BasePath, moduleID+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		// Handle file not found specifically
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("module %s not found: %w", moduleID, err)
		}
		return nil, fmt.Errorf("failed to read module file %s: %w", filePath, err)
	}

	var module model.Module
	err = json.Unmarshal(data, &module)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal module data from %s: %w", filePath, err)
	}
	fmt.Printf("Placeholder: Loaded module metadata from %s\n", filePath) // Placeholder log
	return &module, nil
}

// GetAllModuleIDs scans the BasePath directory for *.json files and extracts IDs.
func (js *JSONStore) GetAllModuleIDs() ([]string, error) {
	files, err := os.ReadDir(js.BasePath)
	if err != nil {
		// If the base path itself doesn't exist yet, return empty list, no error
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read storage directory %s: %w", js.BasePath, err)
	}

	var ids []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			id := strings.TrimSuffix(file.Name(), ".json") // Use strings.TrimSuffix
			ids = append(ids, id)
		}
	}
	fmt.Printf("Placeholder: Found module IDs: %v\n", ids) // Placeholder log
	return ids, nil
}

// DeleteModule removes the module's JSON metadata file.
// Note: This implementation does NOT delete the module's actual content directory.
func (js *JSONStore) DeleteModule(moduleID string) error {
	if moduleID == "" {
		return fmt.Errorf("module ID cannot be empty")
	}
	filePath := filepath.Join(js.BasePath, moduleID+".json")

	err := os.Remove(filePath)
	if err != nil {
		// Make it non-fatal if the file doesn't exist (idempotent delete)
		if os.IsNotExist(err) {
			fmt.Printf("Placeholder: Module metadata file %s already deleted or never existed.\n", filePath)
			return nil
		}
		return fmt.Errorf("failed to delete module file %s: %w", filePath, err)
	}
	fmt.Printf("Placeholder: Deleted module metadata file %s\n", filePath) // Placeholder log
	return nil
}

// ReadAll retrieves metadata for all modules by loading each one individually.
func (js *JSONStore) ReadAll() ([]*model.Module, error) {
	ids, err := js.GetAllModuleIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get module IDs: %w", err)
	}

	modules := make([]*model.Module, 0, len(ids))
	for _, id := range ids {
		module, err := js.LoadModule(id)
		if err != nil {
			// If a single module fails to load, we might want to log it and continue,
			// or return the error immediately. Let's return immediately for now.
			return nil, fmt.Errorf("failed to load module %s during ReadAll: %w", id, err)
		}
		modules = append(modules, module)
	}

	fmt.Printf("Placeholder: Loaded %d modules for ReadAll\n", len(modules)) // Placeholder log
	return modules, nil
}
