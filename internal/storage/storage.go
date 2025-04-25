package storage

import "go-module-builder/internal/model"

// DataStore defines the operations needed for persisting module data.
// This allows swapping implementations (e.g., JSON files vs. database) later.
type DataStore interface {
	// SaveModule persists the module's metadata.
	SaveModule(module *model.Module) error

	// LoadModule retrieves a module's metadata by its ID.
	LoadModule(moduleID string) (*model.Module, error)

	// GetAllModuleIDs returns a list of all known module IDs.
	GetAllModuleIDs() ([]string, error)

	// DeleteModule removes a module's metadata (and potentially its files).
	// Consider if file deletion belongs here or in a separate service.
	DeleteModule(moduleID string) error

	// ReadAll retrieves metadata for all modules.
	ReadAll() ([]*model.Module, error)

	// GetBasePath returns the storage base path.
	GetBasePath() string

	// TODO: Add methods for snapshots/history if needed later
	// SaveSnapshotMetadata(...) error
	// LoadSnapshotMetadata(...) (*SnapshotInfo, error)
}
