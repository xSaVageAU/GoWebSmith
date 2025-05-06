package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath" // Added for joining paths

	// Added for module type
	"go-module-builder/internal/storage" // Added for storage interface
)

// adminApplication holds the application-wide dependencies for the admin server.
type adminApplication struct {
	logger      *slog.Logger
	moduleStore storage.DataStore // Corrected interface name
	projectRoot string            // Added project root
}

func main() {
	// --- Initialize Logger ---
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// --- Initialize Application Struct ---
	// Get working directory (assuming run from project root)
	wd, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get working directory", "error", err)
		os.Exit(1)
	}
	projRoot := wd
	metadataDir := filepath.Join(projRoot, ".module_metadata")
	logger.Info("Using metadata directory", "path", metadataDir)

	// --- Initialize Storage ---
	store, err := storage.NewJSONStore(metadataDir)
	if err != nil {
		// Log non-fatal error if dir doesn't exist, fatal otherwise
		if os.IsNotExist(err) {
			logger.Warn("Metadata directory not found, no modules will be loaded initially.", "path", metadataDir)
			// Create an empty store or handle appropriately if needed
			// For now, we might proceed with an uninitialized store or a dummy one
			// Let's proceed, routes.go handler will need to check if store is nil or handle error
		} else {
			logger.Error("Failed to initialize module store", "error", err)
			os.Exit(1)
		}
	}

	// --- Initialize Application Struct ---
	app := &adminApplication{
		logger:      logger,
		moduleStore: store, // Assign the initialized store
		projectRoot: projRoot,
	}

	// --- Configuration (Placeholder) ---
	// TODO: Integrate with Viper or flags later
	adminPort := "8081"

	// --- Start Server ---
	addr := ":" + adminPort
	logger.Info("Starting admin server", "address", fmt.Sprintf("http://localhost%s", addr))

	// Get the router from the routes method
	router := app.routes() // This now uses the app variable

	err = http.ListenAndServe(addr, router) // Corrected assignment from := to =
	if err != nil {
		logger.Error("Admin server failed to start", "error", err)
		os.Exit(1) // Keep os.Exit(1)
	}
}
