package main

import (
	"bufio" // Added for reading user input
	"flag"
	"fmt"
	"go-module-builder/internal/modulemanager" // Import the new manager package
	"go-module-builder/internal/storage"
	"go-module-builder/internal/templating"
	"go-module-builder/pkg/fsutils"
	"log"
	"log/slog" // Import slog for the manager
	"os"
	"os/exec" // Added for opening browser
	"path/filepath"
	"runtime" // Added for OS detection
	"strings" // Added for trimming user input
)

const (
	metadataDir    = ".module_metadata" // Directory to store JSON metadata files
	modulesBaseDir = "modules"          // Default directory to store actual module content
)

func main() {
	fmt.Println("Module Builder CLI - Initial Setup")

	// --- Setup Phase ---
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	fmt.Printf("Operating in: %s\n", projectRoot)

	storagePath := filepath.Join(projectRoot, metadataDir)
	moduleStorageDir := filepath.Join(projectRoot, modulesBaseDir)

	store, err := storage.NewJSONStore(storagePath)
	if err != nil {
		log.Fatalf("Error initializing storage: %v", err)
	}
	// Initialize the templating engine
	templateEngine := templating.NewEngine(store)

	// Initialize Module Manager
	// Use standard log for CLI output, but maybe a structured logger for the manager itself?
	// For now, let's pass nil to use the manager's default discard logger for internal operations.
	// Or create a simple slog logger. Let's create one.
	cliLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})) // Log manager errors to stderr
	manager := modulemanager.NewManager(store, cliLogger, projectRoot, moduleStorageDir)

	fmt.Printf("Using storage path: %s\n", storagePath)
	fmt.Printf("Using modules base path: %s\n", moduleStorageDir)

	// --- Command Parsing using 'flag' package ---
	// Define subcommands
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
	previewCmd := flag.NewFlagSet("preview", flag.ExitOnError)
	addTemplateCmd := flag.NewFlagSet("add-template", flag.ExitOnError)
	purgeRemovedCmd := flag.NewFlagSet("purge-removed", flag.ExitOnError)
	// --- New: Define update subcommand ---
	updateCmd := flag.NewFlagSet("update", flag.ExitOnError)

	// Flags for create command
	createName := createCmd.String("name", "", "Name of the module to create (required)")
	createSlug := createCmd.String("slug", "", "Optional custom URL slug (default: module UUID)") // NEW FLAG

	// Flags for delete command
	deleteID := deleteCmd.String("id", "", "ID(s) of the module(s) to delete (comma-separated)")
	deleteForce := deleteCmd.Bool("force", false, "Force delete files and metadata immediately (optional)")
	deleteNukeAll := deleteCmd.Bool("nuke-all", false, "DANGER: Delete ALL modules and metadata (for development)")

	// Flags for preview command
	previewID := previewCmd.String("id", "", "ID of the module to preview (required)")

	// Flags for add-template command
	addTemplateName := addTemplateCmd.String("name", "", "Filename for the new template (e.g., card.html) (required)")
	addTemplateModuleID := addTemplateCmd.String("moduleId", "", "ID of the module to add the template to (required)")

	// --- New: Flags for update command ---
	updateID := updateCmd.String("id", "", "ID of the module to update (required)")
	updateName := updateCmd.String("name", "", "New name for the module (optional)") // Make optional
	updateSlug := updateCmd.String("slug", "", "New URL slug for the module (optional)")
	updateGroup := updateCmd.String("group", "", "New group for the module (optional)")
	updateLayout := updateCmd.String("layout", "", "New layout file override (optional)")
	updateDesc := updateCmd.String("desc", "", "New description for the module (optional)")
	// Note: IsActive and Assets might need different handling (e.g., separate commands or flags)

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "list":
		listCmd.Parse(os.Args[2:])
		handleListModules(store)
	case "create":
		createCmd.Parse(os.Args[2:])
		if *createName == "" {
			fmt.Println("Error: -name flag is required for create command")
			createCmd.Usage()
			return
		}
		// Call the manager's CreateModule method
		_, err := manager.CreateModule(*createName, *createSlug)
		if err != nil {
			// Use standard log for fatal CLI errors
			log.Fatalf("Error creating module via manager: %v", err)
		}
		// Success message is now handled within the manager method's logging
	case "delete":
		deleteCmd.Parse(os.Args[2:])
		// Check flags *after* parsing
		if !*deleteNukeAll && *deleteID == "" {
			fmt.Println("Error: Missing required -id flag (or use --nuke-all) for delete command")
			deleteCmd.Usage()
			os.Exit(1)
		}
		// Pass necessary paths and flags to the handler
		// Pass the manager instance to the handler
		handleDeleteModule(manager, *deleteID, *deleteForce, *deleteNukeAll)
	case "preview":
		previewCmd.Parse(os.Args[2:])
		if *previewID == "" {
			fmt.Println("Error: -id flag is required for preview command")
			previewCmd.Usage()
			return
		}
		handlePreviewModule(templateEngine, *previewID)
	case "add-template":
		addTemplateCmd.Parse(os.Args[2:])
		if *addTemplateName == "" || *addTemplateModuleID == "" {
			fmt.Println("Error: -name and -moduleId flags are required for add-template command")
			addTemplateCmd.Usage()
			return
		}
		// Call the manager's AddTemplate method
		_, err := manager.AddTemplate(*addTemplateModuleID, *addTemplateName)
		if err != nil {
			log.Fatalf("Error adding template via manager: %v", err)
		}
		// Success message handled by manager logging
	case "purge-removed":
		purgeRemovedCmd.Parse(os.Args[2:])
		// Call the manager's PurgeRemovedModules method
		handlePurgeRemovedModules(manager) // Pass manager instead of store

	// --- New: Handle update command ---
	case "update":
		updateCmd.Parse(os.Args[2:])
		// Only ID is strictly required now
		if *updateID == "" {
			fmt.Println("Error: -id flag is required for update command")
			updateCmd.Usage()
			return
		}
		// Check if at least one update flag was provided
		if *updateName == "" && *updateSlug == "" && *updateGroup == "" && *updateLayout == "" && *updateDesc == "" {
			fmt.Println("Error: At least one update flag (-name, -slug, -group, -layout, -desc) must be provided")
			updateCmd.Usage()
			return
		}
		// Pass all flags to the handler
		// Call the manager's UpdateModule method
		err := manager.UpdateModule(*updateID, *updateName, *updateSlug, *updateGroup, *updateLayout, *updateDesc)
		if err != nil {
			log.Fatalf("Error updating module via manager: %v", err)
		}
		// Success message is handled by manager logging

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
	}

	fmt.Println("\nCLI finished.")
}

func printUsage() {
	fmt.Println("\nUsage: builder-cli <command> [options]")
	fmt.Println("Available commands:")
	fmt.Println("  list          List all known modules")
	fmt.Println("  create -name <module-name> [-slug <custom-slug>]")
	fmt.Println("                Create a new module (slug defaults to UUID if not provided)")
	fmt.Println("  update -id <module-id> [-name <new-name>] [-slug <new-slug>] [-group <group>] [-layout <layout>] [-desc <desc>]")
	fmt.Println("                Update module metadata (provide at least one optional flag)")
	fmt.Println("  delete -id <module-id,...> [--force] | --nuke-all")
	fmt.Println("                Delete modules by ID, or use --nuke-all to delete everything")
	fmt.Println("  preview -id <module-id>")
	fmt.Println("                Combine and print module templates to console")
	fmt.Println("  add-template -name <filename> -moduleId <module-id>")
	fmt.Println("                Add a new template file to a module")
	fmt.Println("  purge-removed Permanently delete all modules marked as 'removed'")
	// Add more commands as they are implemented
}

func handleListModules(store storage.DataStore) {
	fmt.Println("\nListing modules...")
	// Load all module details instead of just IDs
	modules, err := store.ReadAll()
	if err != nil {
		log.Fatalf("Error listing modules: %v", err)
	}
	if len(modules) == 0 {
		fmt.Println("No modules found.")
	} else {
		fmt.Println("Found modules:")
		for _, module := range modules {
			// Count non-base templates
			nonBaseTemplateCount := 0
			for _, t := range module.Templates {
				if !t.IsBase {
					nonBaseTemplateCount++
				}
			}

			// Determine status string based on IsActive
			statusStr := "Inactive"
			if module.IsActive {
				statusStr = "Active"
			}
			// Print details including the non-base template count and status
			fmt.Printf("- ID: %s\n  Name: %s\n  Status: %s\n  Path: %s\n  Additional Templates: %d\n\n",
				module.ID,
				module.Name,
				statusStr, // Use the derived status string
				module.Directory,
				nonBaseTemplateCount)
		}
	}
}

// func handleCreateModule(store storage.DataStore, moduleBaseDir, moduleName, customSlug string) {
// 	// --- This logic is now moved to internal/modulemanager/manager.go ---
// }

// Helper function for confirmation prompts
func askForConfirmation(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/N]: ", prompt)
		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Error reading confirmation: %v", err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" || response == "" {
			return false
		}
		// Ask again if input is invalid
	}
}

// handleDeleteModule handles deleting modules.
// If nukeAll is true, it deletes ALL modules and metadata.
// Otherwise, it deletes modules based on comma-separated IDs.
// Pass ModuleManager instead of store and moduleBaseDir
func handleDeleteModule(manager *modulemanager.ModuleManager, moduleIDs string, force, nukeAll bool) {
	// --- Nuke All Logic (Remains in CLI for now) ---
	if nukeAll {
		fmt.Println("\n!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
		fmt.Println("!!! DANGER: --nuke-all flag detected.                    !!!")
		fmt.Println("!!! This will permanently delete ALL module directories  !!!")
		fmt.Println("!!! (modules/, modules_removed/) and ALL metadata       !!!")
		fmt.Println("!!! (.module_metadata/).                                 !!!")
		fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
		if !askForConfirmation("Are you sure you want to proceed?") {
			fmt.Println("Operation cancelled.")
			return
		}

		fmt.Println("Proceeding with --nuke-all...")

		// Get paths from manager or config if possible in future?
		// For now, assume standard structure relative to project root.
		moduleBaseDir := manager.GetModulesDir() // Need to add GetModulesDir() to manager
		removedModulesDir := filepath.Join(manager.GetProjectRoot(), "modules_removed")
		storagePath := manager.GetStoreBasePath() // Need to add GetStoreBasePath() to manager

		pathsToDelete := []string{moduleBaseDir, removedModulesDir, storagePath}
		failedDeletes := 0
		failedCreates := 0

		for _, p := range pathsToDelete {
			fmt.Printf("Attempting to remove directory: %s\n", p)
			if _, err := os.Stat(p); err == nil {
				err = os.RemoveAll(p)
				if err != nil {
					log.Printf("Error removing directory %s: %v", p, err)
					failedDeletes++
				}
			} else if !os.IsNotExist(err) {
				log.Printf("Error checking directory %s before remove: %v", p, err)
				failedDeletes++
			} else {
				fmt.Printf("Directory %s does not exist, skipping removal.\n", p)
			}
		}

		fmt.Println("Attempting to recreate directories...")
		for _, p := range pathsToDelete { // Recreate the same paths
			err := fsutils.CreateDir(p)
			if err != nil {
				log.Printf("Error recreating directory %s: %v", p, err)
				failedCreates++
			} else {
				fmt.Printf("Recreated directory: %s\n", p)
			}
		}

		fmt.Println("\n--- Nuke All Summary ---")
		if failedDeletes > 0 {
			fmt.Printf("Errors during deletion: %d (check logs)\n", failedDeletes)
		}
		if failedCreates > 0 {
			fmt.Printf("Errors during recreation: %d (check logs)\n", failedCreates)
		}
		if failedDeletes == 0 && failedCreates == 0 {
			fmt.Println("All module directories and metadata successfully nuked and reset.")
		}
		return // Stop processing after nuke
	}

	// --- ID-Based Delete Logic (Uses ModuleManager) ---
	ids := strings.Split(moduleIDs, ",")
	fmt.Printf("\nAttempting to process delete for %d module ID(s) (Force: %v)\n", len(ids), force)

	successCount := 0
	failCount := 0

	for _, id := range ids {
		trimmedID := strings.TrimSpace(id)
		if trimmedID == "" {
			continue // Skip empty strings resulting from extra commas
		}

		fmt.Printf("--- Processing ID: %s ---\n", trimmedID)

		// Confirmation prompt (remains in CLI)
		if force {
			// Need to load module name for prompt if possible, or use generic prompt
			// Let's try loading first, but handle if it fails (e.g., already deleted)
			moduleName := trimmedID                                  // Default to ID if name can't be loaded
			mod, loadErr := manager.GetStore().LoadModule(trimmedID) // Need GetStore() method
			if loadErr == nil {
				moduleName = mod.Name
			} else if !os.IsNotExist(loadErr) {
				log.Printf("Warning: Could not load module %s to confirm name before force delete: %v", trimmedID, loadErr)
			} // If IsNotExist, proceed with generic prompt

			fmt.Printf("WARNING: You are about to permanently delete module '%s' (ID: %s) and all its files.\n", moduleName, trimmedID)
			if !askForConfirmation("Are you sure you want to proceed?") {
				fmt.Println("Operation cancelled for this ID.")
				failCount++ // Count cancellation as failure/skip
				continue
			}
		}

		// Call the manager's DeleteModule method
		err := manager.DeleteModule(trimmedID, force)
		if err != nil {
			// Log the error from the manager
			log.Printf("Error processing delete for ID %s: %v", trimmedID, err)
			failCount++
		} else {
			// Success message is now handled by manager's logging
			successCount++
		}
	}

	fmt.Printf("\n--- Bulk Delete Summary ---\n")
	fmt.Printf("Successfully processed: %d\n", successCount)
	fmt.Printf("Failed/Skipped: %d\n", failCount)
	fmt.Printf("Total IDs provided: %d\n", len(ids))
}

func handlePreviewModule(engine *templating.Engine, moduleID string) {
	fmt.Printf("\nGenerating preview for module ID: %s\n", moduleID)

	// 1. Combine templates into rendered HTML string
	combinedOutput, err := engine.CombineTemplates(moduleID)
	if err != nil {
		log.Fatalf("Error generating preview content: %v", err)
	}

	// 2. Create a temporary HTML file
	tempFile, err := os.CreateTemp("", fmt.Sprintf("module-preview-%s-*.html", moduleID))
	if err != nil {
		log.Fatalf("Error creating temporary preview file: %v", err)
	}
	defer tempFile.Close() // Ensure file is closed

	// 3. Write the rendered HTML to the temp file
	_, err = tempFile.WriteString(combinedOutput)
	if err != nil {
		log.Fatalf("Error writing to temporary preview file: %v", err)
	}

	// Get the full path of the temp file
	tempFilePath := tempFile.Name()
	fmt.Printf("Preview HTML saved to: %s\n", tempFilePath)

	// 4. Open the temporary file in the default browser
	if err := openBrowser(tempFilePath); err != nil {
		log.Printf("Warning: Failed to open preview in browser: %v", err)
		fmt.Println("Please open the file manually in your browser.")
	} else {
		fmt.Println("Attempting to open preview in your default browser...")
	}
}

// openBrowser tries to open the given URL/file path in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Use "rundll32" for broader compatibility on Windows
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start() // Use Start for non-blocking execution
}

// func handleAddTemplate(store storage.DataStore, moduleID, templateName string) {
// 	// --- This logic is now moved to internal/modulemanager/manager.go ---
// }

// Pass ModuleManager instead of store
func handlePurgeRemovedModules(manager *modulemanager.ModuleManager) {
	fmt.Println("\nAttempting to purge all removed modules...")

	// Confirmation prompt remains in the CLI
	// We need to know if there *are* any modules to purge first.
	// Let's peek using the store via the manager.
	modules, err := manager.GetStore().ReadAll()
	if err != nil {
		log.Fatalf("Error reading module metadata before purge confirmation: %v", err)
	}
	removedCount := 0
	for _, mod := range modules {
		if !mod.IsActive {
			removedCount++
		}
	}

	if removedCount == 0 {
		fmt.Println("No inactive modules found. Nothing to purge.")
		return
	}

	fmt.Printf("WARNING: You are about to permanently delete the files and metadata for %d inactive module(s).\n", removedCount)
	// We could list them here again if desired, but the manager logs details during the actual purge.
	if !askForConfirmation("Are you sure you want to proceed?") {
		fmt.Println("Operation cancelled.")
		return
	}

	// Call the manager method to perform the purge
	purgedCount, err := manager.PurgeRemovedModules()
	if err != nil {
		// This error is likely from the initial ReadAll inside the manager method
		log.Fatalf("Error during purge operation: %v", err)
	}

	// Report summary based on manager's return value (manager logs details)
	fmt.Println("\nPurge operation finished.")
	fmt.Printf("Successfully purged metadata for %d modules.\n", purgedCount)
	if purgedCount < removedCount {
		fmt.Println("Warning: Some modules may not have been fully purged (check logs above).")
	}
}

// func handleUpdateModule(store storage.DataStore, moduleID, newName, newSlug, newGroup, newLayout, newDesc string) {
// 	// --- This logic is now moved to internal/modulemanager/manager.go ---
// }
