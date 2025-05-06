package modulemanager

import (
	"fmt"
	"go-module-builder/internal/generator"
	"go-module-builder/internal/model"
	"go-module-builder/internal/storage"
	"go-module-builder/pkg/fsutils" // Added for CreateDir
	"io"
	"log/slog"      // Using slog for consistency
	"os"            // Added for file operations
	"path/filepath" // Added for path joining
	"time"          // Added for LastUpdated timestamp

	"github.com/google/uuid"
)

// ModuleManager provides methods for managing modules (create, delete, update, etc.)
// It encapsulates the core logic previously found in the CLI handlers.
type ModuleManager struct {
	store       storage.DataStore
	logger      *slog.Logger
	modulesDir  string // Base directory where module files are stored (e.g., "modules")
	projectRoot string // Project root directory
}

// NewManager creates a new ModuleManager instance.
func NewManager(store storage.DataStore, logger *slog.Logger, projectRoot string, modulesDir string) *ModuleManager {
	if logger == nil {
		// Provide a default discard logger if none is provided
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &ModuleManager{
		store:       store,
		logger:      logger,
		projectRoot: projectRoot,
		modulesDir:  modulesDir, // Store the base modules directory path
	}
}

// CreateModule handles the creation of a new module's boilerplate and metadata.
// It takes the desired module name and an optional custom slug.
// It returns the newly created module's metadata or an error.
func (m *ModuleManager) CreateModule(moduleName, customSlug string) (*model.Module, error) {
	m.logger.Info("Creating module", "name", moduleName, "customSlug", customSlug)

	// 1. Generate unique ID
	moduleID := uuid.New().String()
	m.logger.Debug("Generated module ID", "id", moduleID)

	// 2. Get generator config using the manager's modulesDir
	genConfig := generator.DefaultGeneratorConfig(m.modulesDir)

	// 3. Generate boilerplate files/dirs, passing the custom slug
	newModule, err := generator.GenerateModuleBoilerplate(genConfig, moduleName, moduleID, customSlug)
	if err != nil {
		m.logger.Error("Error generating module boilerplate", "error", err, "moduleName", moduleName, "moduleID", moduleID)
		// Consider more graceful error handling / cleanup here? For now, just return error.
		return nil, fmt.Errorf("generating module boilerplate failed: %w", err)
	}

	// 4. Save module metadata using the manager's store
	err = m.store.SaveModule(newModule)
	if err != nil {
		m.logger.Error("Error saving module metadata", "error", err, "moduleName", moduleName, "moduleID", moduleID)
		// Consider cleanup of generated files if metadata save fails? Complex.
		return nil, fmt.Errorf("saving module metadata failed: %w", err)
	}

	m.logger.Info("Successfully created module", "name", moduleName, "id", moduleID, "directory", newModule.Directory)
	return newModule, nil
}

// DeleteModule handles deleting a module by ID.
// It supports both soft delete (moving files, marking inactive) and hard delete (removing files and metadata).
// Returns an error if the deletion fails.
// Note: Confirmation logic (askForConfirmation) might need adjustment for non-CLI use cases later.
func (m *ModuleManager) DeleteModule(moduleID string, force bool) error {
	m.logger.Info("Processing delete request", "moduleID", moduleID, "force", force)

	// 1. Load module metadata first
	module, err := m.store.LoadModule(moduleID)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Warn("Module metadata not found, cannot delete.", "moduleID", moduleID)
			// Return a specific error or nil? Let's return an error.
			return fmt.Errorf("module metadata for ID %s not found", moduleID)
		}
		m.logger.Error("Error loading module metadata for delete", "moduleID", moduleID, "error", err)
		return fmt.Errorf("loading module metadata failed for ID %s: %w", moduleID, err)
	}

	if force {
		// --- Force Delete Logic ---
		m.logger.Warn("Performing force delete", "moduleID", moduleID, "name", module.Name)
		// NOTE: Confirmation should be handled by the caller (CLI/UI)

		var deleteErr error
		// Delete the actual module directory - module.Directory should already be the correct absolute path
		if module.Directory != "" {
			if _, err := os.Stat(module.Directory); err == nil { // Use module.Directory directly
				m.logger.Info("Attempting to delete module directory", "path", module.Directory)
				err = os.RemoveAll(module.Directory) // Use module.Directory directly
				if err != nil {
					m.logger.Error("Failed to force delete module directory", "path", module.Directory, "error", err)
					// Record the error but continue to attempt metadata deletion
					deleteErr = fmt.Errorf("failed to delete directory %s: %w", module.Directory, err)
				}
			} else if !os.IsNotExist(err) {
				// Log error if stat failed for reasons other than not existing
				m.logger.Error("Could not stat module directory before force delete", "path", module.Directory, "error", err)
				deleteErr = fmt.Errorf("failed to stat directory %s: %w", module.Directory, err)
			} else {
				m.logger.Info("Module directory not found, skipping delete.", "path", module.Directory)
			}
		}

		// Delete the metadata (even if directory deletion had issues, try to clean up metadata)
		err = m.store.DeleteModule(moduleID)
		if err != nil {
			m.logger.Error("Error force deleting module metadata", "moduleID", moduleID, "error", err)
			// Combine errors if directory deletion also failed
			if deleteErr != nil {
				deleteErr = fmt.Errorf("failed to delete directory (%v) and metadata: %w", deleteErr, err)
			} else {
				deleteErr = fmt.Errorf("failed to delete metadata: %w", err)
			}
		}

		if deleteErr == nil {
			m.logger.Info("Successfully force deleted module", "moduleID", moduleID, "name", module.Name)
			return nil // Success
		}
		return deleteErr // Return combined or single error

	} else {
		// --- Soft Delete Logic (Mark as Inactive) ---
		if !module.IsActive {
			m.logger.Info("Module is already inactive, skipping soft delete.", "moduleID", moduleID, "name", module.Name)
			return nil // Not an error, just nothing to do
		}
		m.logger.Info("Performing soft delete", "moduleID", moduleID, "name", module.Name)

		removedModulesBaseDir := filepath.Join(m.projectRoot, "modules_removed") // Use projectRoot
		newModulePathRelative := filepath.Join("modules_removed", moduleID)      // Relative path for metadata
		newModulePathAbsolute := filepath.Join(removedModulesBaseDir, moduleID)  // Absolute path for move

		// Ensure the base removed directory exists
		if err := fsutils.CreateDir(removedModulesBaseDir); err != nil {
			m.logger.Error("Failed to create directory for removed modules", "path", removedModulesBaseDir, "moduleID", moduleID, "error", err)
			return fmt.Errorf("failed to create removed modules directory '%s': %w", removedModulesBaseDir, err)
		}

		moveFailed := false
		// originalModulePathAbsolute := filepath.Join(m.projectRoot, module.Directory) // Don't need this, module.Directory is absolute

		// Check if the original directory exists before trying to move
		if _, err := os.Stat(module.Directory); err == nil { // Use module.Directory directly
			// Attempt to move the directory
			m.logger.Info("Moving directory", "from", module.Directory, "to", newModulePathAbsolute)
			err = os.Rename(module.Directory, newModulePathAbsolute) // Use module.Directory directly
			if err != nil {
				m.logger.Error("Failed to move module directory to removed location", "moduleID", moduleID, "from", module.Directory, "to", newModulePathAbsolute, "error", err)
				moveFailed = true
			}
		} else if os.IsNotExist(err) {
			m.logger.Warn("Original module directory not found, only updating metadata status.", "path", module.Directory, "moduleID", moduleID)
			newModulePathRelative = module.Directory // Keep original relative path in metadata if dir was missing
		} else {
			// Stat failed for another reason
			m.logger.Error("Failed to check original module directory", "path", module.Directory, "moduleID", moduleID, "error", err)
			return fmt.Errorf("failed to check original module directory '%s': %w", module.Directory, err)
		}

		if moveFailed {
			// Don't update metadata if move failed
			return fmt.Errorf("failed to move module directory for ID %s", moduleID)
		}

		// Update metadata to mark as inactive
		module.IsActive = false
		module.Directory = newModulePathRelative // Update path to the new relative location
		module.LastUpdated = time.Now()

		err = m.store.SaveModule(module) // Use SaveModule which acts like Update
		if err != nil {
			// Attempt to rollback the move? Complex. For now, just log and return error.
			m.logger.Error("Error updating module metadata to 'removed' status", "moduleID", moduleID, "error", err)
			return fmt.Errorf("failed to update module metadata for ID %s: %w", moduleID, err)
		}

		m.logger.Info("Successfully marked module as removed", "moduleID", moduleID, "name", module.Name, "newPath", newModulePathRelative)
		return nil // Success
	}
}

// --- Getter Methods ---

// GetStore returns the underlying DataStore instance.
func (m *ModuleManager) GetStore() storage.DataStore {
	return m.store
}

// GetModulesDir returns the base directory path for modules.
func (m *ModuleManager) GetModulesDir() string {
	return m.modulesDir
}

// GetProjectRoot returns the project root path.
func (m *ModuleManager) GetProjectRoot() string {
	return m.projectRoot
}

// GetStoreBasePath returns the base path used by the underlying store.
func (m *ModuleManager) GetStoreBasePath() string {
	// Assuming the store interface has or will have a GetBasePath method
	if m.store != nil {
		return m.store.GetBasePath()
	}
	return "" // Or handle error appropriately
}

// UpdateModule handles updating the metadata of an existing module.
// It takes the module ID and optional new values for name, slug, group, layout, and description.
// Returns an error if the update fails.
func (m *ModuleManager) UpdateModule(moduleID, newName, newSlug, newGroup, newLayout, newDesc string) error {
	m.logger.Info("Updating module", "moduleID", moduleID)

	// 1. Load the module metadata
	module, err := m.store.LoadModule(moduleID)
	if err != nil {
		m.logger.Error("Error loading module metadata for update", "moduleID", moduleID, "error", err)
		return fmt.Errorf("loading module metadata failed for ID %s: %w", moduleID, err)
	}

	// 2. Update fields based on provided non-empty values
	updated := false
	if newName != "" {
		m.logger.Debug("Updating Name", "moduleID", moduleID, "old", module.Name, "new", newName)
		module.Name = newName
		updated = true
	}
	if newSlug != "" {
		// Consider adding validation/sanitization for user-provided slugs here
		m.logger.Debug("Updating Slug", "moduleID", moduleID, "old", module.Slug, "new", newSlug)
		module.Slug = newSlug
		updated = true
	}
	if newGroup != "" {
		m.logger.Debug("Updating Group", "moduleID", moduleID, "old", module.Group, "new", newGroup)
		module.Group = newGroup
		updated = true
	}
	if newLayout != "" {
		m.logger.Debug("Updating Layout", "moduleID", moduleID, "old", module.Layout, "new", newLayout)
		module.Layout = newLayout
		updated = true
	}
	if newDesc != "" {
		m.logger.Debug("Updating Description", "moduleID", moduleID, "old", module.Description, "new", newDesc)
		module.Description = newDesc
		updated = true
	}

	if !updated {
		m.logger.Info("No update values provided, nothing to change.", "moduleID", moduleID)
		return nil // Not an error if nothing was provided to update
	}

	module.LastUpdated = time.Now()

	// 3. Save updated module metadata
	if err := m.store.SaveModule(module); err != nil {
		m.logger.Error("Error saving updated module metadata", "moduleID", moduleID, "error", err)
		return fmt.Errorf("saving updated module metadata failed for ID %s: %w", moduleID, err)
	}

	m.logger.Info("Successfully updated module metadata", "moduleID", moduleID)
	return nil
}

// AddTemplate adds a new template file to an existing module.
// It creates the physical file and updates the module's metadata.
// Returns the updated module metadata or an error.
func (m *ModuleManager) AddTemplate(moduleID, templateName string) (*model.Module, error) {
	m.logger.Info("Adding template to module", "moduleID", moduleID, "templateName", templateName)

	// 1. Load the module metadata
	module, err := m.store.LoadModule(moduleID)
	if err != nil {
		m.logger.Error("Error loading module metadata for add-template", "moduleID", moduleID, "error", err)
		return nil, fmt.Errorf("loading module metadata failed for ID %s: %w", moduleID, err)
	}

	// 2. Check if template name already exists in metadata
	for _, t := range module.Templates {
		if t.Name == templateName {
			m.logger.Error("Template name already exists in metadata", "moduleID", moduleID, "templateName", templateName)
			return nil, fmt.Errorf("template '%s' already exists in module %s metadata", templateName, moduleID)
		}
	}

	// 3. Call the generator to create the physical template file
	// Use the manager's modulesDir which should be the base "modules" directory
	err = generator.AddTemplateToModule(moduleID, templateName, m.modulesDir)
	if err != nil {
		m.logger.Error("Error creating template file via generator", "moduleID", moduleID, "templateName", templateName, "error", err)
		return nil, fmt.Errorf("creating template file failed: %w", err)
	}

	// 4. Determine the next order number
	maxOrder := -1
	for _, t := range module.Templates {
		if t.Order > maxOrder {
			maxOrder = t.Order
		}
	}
	newOrder := maxOrder + 1

	// 5. Create new Template metadata
	templateSubDir := "templates" // Standard subdirectory within a module
	relativePath := filepath.Join(templateSubDir, templateName)
	newTemplate := model.Template{
		Name:   templateName,
		Path:   relativePath,
		IsBase: false, // New templates added this way are not base templates
		Order:  newOrder,
	}

	// 6. Append to module's template list in metadata
	module.Templates = append(module.Templates, newTemplate)
	module.LastUpdated = time.Now()

	// 7. Save updated module metadata
	if err := m.store.SaveModule(module); err != nil {
		m.logger.Error("Error updating module metadata after adding template", "moduleID", moduleID, "templateName", templateName, "error", err)
		// Consider rolling back file creation? Complex.
		return nil, fmt.Errorf("saving updated module metadata failed for ID %s: %w", moduleID, err)
	}

	m.logger.Info("Template added successfully and metadata updated", "moduleID", moduleID, "templateName", templateName, "order", newOrder)
	return module, nil // Return the updated module
}

// PurgeRemovedModules finds all inactive modules and permanently deletes their files and metadata.
// Returns the number of modules successfully purged and a potential error (e.g., if reading metadata fails).
// Individual deletion errors are logged but don't stop the process.
// Note: Confirmation should be handled by the caller.
func (m *ModuleManager) PurgeRemovedModules() (purgedCount int, readErr error) {
	m.logger.Info("Attempting to purge all removed modules...")

	modules, err := m.store.ReadAll()
	if err != nil {
		m.logger.Error("Error reading module metadata for purge", "error", err)
		return 0, fmt.Errorf("reading module metadata failed: %w", err) // Return error here
	}

	removedModules := make([]*model.Module, 0)
	for _, mod := range modules {
		if !mod.IsActive { // Find inactive modules
			removedModules = append(removedModules, mod)
		}
	}

	if len(removedModules) == 0 {
		m.logger.Info("No inactive modules found. Nothing to purge.")
		return 0, nil // No error, just nothing done
	}

	m.logger.Info("Found inactive modules to purge", "count", len(removedModules))
	// NOTE: Confirmation prompt is handled by the caller (CLI)

	purgedCount = 0 // Initialize counter
	failedDirDelete := 0
	failedMetaDelete := 0

	// Iterate only over the pre-filtered removed modules
	for _, module := range removedModules {
		m.logger.Debug("Purging removed module", "moduleID", module.ID, "name", module.Name)

		// 1. Attempt to delete the directory (module.Directory should be absolute path)
		if module.Directory != "" {
			if _, err := os.Stat(module.Directory); err == nil {
				m.logger.Info("Deleting directory for purged module", "path", module.Directory)
				err = os.RemoveAll(module.Directory)
				if err != nil {
					m.logger.Error("Failed to delete directory during purge", "path", module.Directory, "error", err)
					failedDirDelete++
				}
			} else if !os.IsNotExist(err) {
				// Log error if stat failed for reasons other than not existing
				m.logger.Error("Could not stat module directory before purge", "path", module.Directory, "error", err)
				failedDirDelete++
			} else {
				m.logger.Info("Directory for purged module not found, skipping delete.", "path", module.Directory)
			}
		}

		// 2. Attempt to delete the metadata (even if directory deletion failed/skipped)
		m.logger.Info("Deleting metadata for purged module", "moduleID", module.ID)
		err = m.store.DeleteModule(module.ID)
		if err != nil {
			m.logger.Error("Failed to delete metadata during purge", "moduleID", module.ID, "error", err)
			failedMetaDelete++
		} else {
			purgedCount++ // Only increment if metadata deletion succeeds
		}
	}

	m.logger.Info("Purge complete.", "purgedCount", purgedCount, "dirDeleteFailures", failedDirDelete, "metaDeleteFailures", failedMetaDelete)
	// We don't return an error for individual delete failures, only for the initial ReadAll failure.
	return purgedCount, nil
}

// TODO: Consider if NukeAll logic belongs in the manager or stays CLI-only. (Leaning towards CLI-only)
