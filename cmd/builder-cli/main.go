package main

import (
	"bufio" // Added for reading user input
	"flag"
	"fmt"
	"go-module-builder/internal/generator"
	"go-module-builder/internal/model"
	"go-module-builder/internal/storage"
	"go-module-builder/internal/templating"
	"go-module-builder/pkg/fsutils"
	"log"
	"os"
	"os/exec" // Added for opening browser
	"path/filepath"
	"runtime" // Added for OS detection
	"strings" // Added for trimming user input
	"time"

	"github.com/google/uuid"
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
	updateName := updateCmd.String("name", "", "New name for the module (required)")

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
		handleCreateModule(store, moduleStorageDir, *createName)
	case "delete":
		deleteCmd.Parse(os.Args[2:])
		// Check flags *after* parsing
		if !*deleteNukeAll && *deleteID == "" {
			fmt.Println("Error: Missing required -id flag (or use --nuke-all) for delete command")
			deleteCmd.Usage()
			os.Exit(1)
		}
		// Pass necessary paths and flags to the handler
		handleDeleteModule(store, moduleStorageDir, *deleteID, *deleteForce, *deleteNukeAll)
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
		handleAddTemplate(store, *addTemplateModuleID, *addTemplateName)
	case "purge-removed":
		purgeRemovedCmd.Parse(os.Args[2:])
		handlePurgeRemovedModules(store)

	// --- New: Handle update command ---
	case "update":
		updateCmd.Parse(os.Args[2:])
		if *updateID == "" || *updateName == "" {
			fmt.Println("Error: -id and -name flags are required for update command")
			updateCmd.Usage()
			return
		}
		handleUpdateModule(store, *updateID, *updateName)

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
	fmt.Println("  create -name <module-name>")
	fmt.Println("                Create a new module")
	fmt.Println("  update -id <module-id> -name <new-name>") // Added update command
	fmt.Println("                Update a module's metadata (e.g., rename)")
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

func handleCreateModule(store storage.DataStore, moduleBaseDir, moduleName string) {
	fmt.Printf("\nCreating module: %s\n", moduleName)

	// 1. Generate unique ID
	moduleID := uuid.New().String()
	fmt.Printf("Generated Module ID: %s\n", moduleID)

	// 2. Get generator config
	genConfig := generator.DefaultGeneratorConfig(moduleBaseDir)

	// 3. Generate boilerplate files/dirs
	newModule, err := generator.GenerateModuleBoilerplate(genConfig, moduleName, moduleID)
	if err != nil {
		log.Fatalf("Error generating module boilerplate: %v", err)
		// Consider more graceful error handling / cleanup here
	}

	// 4. Save module metadata
	err = store.SaveModule(newModule)
	if err != nil {
		log.Fatalf("Error saving module metadata: %v", err)
		// Consider cleanup of generated files if metadata save fails
	}

	fmt.Printf("Successfully created module '%s' with ID '%s' in directory '%s'\n", moduleName, moduleID, newModule.Directory)
}

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
func handleDeleteModule(store storage.DataStore, moduleBaseDir, moduleIDs string, force, nukeAll bool) {
	// --- Nuke All Logic ---
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

		removedModulesDir := filepath.Join(filepath.Dir(moduleBaseDir), "modules_removed") // Construct removed path relative to base
		storagePath := store.GetBasePath()

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

	// --- ID-Based Delete Logic (Original logic starts here) ---
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

		// 1. Load module metadata first to get directory path etc.
		module, err := store.LoadModule(trimmedID)
		if err != nil {
			// If it's already gone, maybe that's okay?
			if os.IsNotExist(err) {
				fmt.Printf("Module metadata for ID %s not found. Skipping.\n", trimmedID)
				// Consider this a success in the context of bulk delete?
				// Or maybe just don't count it? Let's not count it.
				continue
			}
			log.Printf("Error loading module metadata for ID %s: %v. Skipping this ID.", trimmedID, err)
			failCount++
			continue
		}

		var deleteErr error
		if force {
			// --- Force Delete Logic ---
			fmt.Printf("WARNING: You are about to permanently delete module '%s' (ID: %s) and all its files.\n", module.Name, trimmedID)
			if !askForConfirmation("Are you sure you want to proceed?") {
				fmt.Println("Operation cancelled.")
				continue
			}

			fmt.Println("Performing force delete...")

			// Delete the actual module directory
			if module.Directory != "" {
				if _, err := os.Stat(module.Directory); err == nil {
					fmt.Printf("Attempting to delete module directory: %s\n", module.Directory)
					err = os.RemoveAll(module.Directory)
					if err != nil {
						log.Printf("Warning: failed to force delete module directory %s: %v", module.Directory, err)
						// Decide if we should proceed with metadata deletion or stop
						// Let's proceed but record the error
						deleteErr = fmt.Errorf("failed to delete directory: %w", err)
					}
				} else if !os.IsNotExist(err) {
					// Log error if stat failed for reasons other than not existing
					log.Printf("Warning: could not stat module directory %s before force delete: %v", module.Directory, err)
					deleteErr = fmt.Errorf("failed to stat directory: %w", err)
				}
			}

			// Delete the metadata (only if directory deletion didn't fail critically?)
			if deleteErr == nil { // Only delete metadata if directory part was ok or skipped
				err = store.DeleteModule(trimmedID)
				if err != nil {
					log.Printf("Error force deleting module metadata for ID %s: %v", trimmedID, err)
					deleteErr = fmt.Errorf("failed to delete metadata: %w", err)
				}
			}

			if deleteErr == nil {
				fmt.Printf("Successfully force deleted module '%s' (ID: %s).\n", module.Name, trimmedID)
				successCount++
			} else {
				failCount++
			}

		} else {
			// --- Soft Delete Logic (Mark as Inactive) ---
			if !module.IsActive { // Check if already inactive
				fmt.Printf("Module '%s' (ID: %s) is already inactive. Skipping.\n", module.Name, trimmedID)
				continue // Skip if already inactive
			}
			fmt.Println("Performing soft delete (moving files and updating status)...")

			removedModulesDir := "modules_removed" // Define the directory for removed modules
			newModulePath := filepath.Join(removedModulesDir, trimmedID)

			// Ensure the base removed directory exists
			if err := fsutils.CreateDir(removedModulesDir); err != nil {
				log.Printf("Failed to create directory for removed modules '%s' for ID %s: %v. Skipping this ID.", removedModulesDir, trimmedID, err)
				failCount++
				continue
			}

			moveFailed := false
			// Check if the original directory exists before trying to move
			if _, err := os.Stat(module.Directory); err == nil {
				// Attempt to move the directory
				fmt.Printf("Moving directory %s to %s\n", module.Directory, newModulePath)
				err = os.Rename(module.Directory, newModulePath)
				if err != nil {
					log.Printf("Failed to move module directory to removed location for ID %s: %v", trimmedID, err)
					moveFailed = true
					// Don't update metadata if move failed?
					// Let's skip metadata update if move fails
				}
			} else if os.IsNotExist(err) {
				fmt.Printf("Warning: Original module directory %s not found for ID %s. Only updating metadata status.\n", module.Directory, trimmedID)
				newModulePath = module.Directory // Keep original path in metadata if dir was missing
			} else {
				// Stat failed for another reason
				log.Printf("Failed to check original module directory %s for ID %s: %v. Skipping this ID.", module.Directory, trimmedID, err)
				failCount++
				continue
			}

			if moveFailed {
				failCount++
				continue // Skip metadata update if move failed
			}

			// Update metadata to mark as inactive
			module.IsActive = false          // Mark as inactive
			module.Directory = newModulePath // Update path to the new location (or original if dir was missing)
			module.LastUpdated = time.Now()

			err = store.SaveModule(module) // Use SaveModule which acts like Update
			if err != nil {
				// Attempt to rollback the move? Complex.
				log.Printf("Error updating module metadata to 'removed' status for ID %s: %v", trimmedID, err)
				failCount++
				continue
			}

			fmt.Printf("Successfully marked module '%s' (ID: %s) as removed.\n", module.Name, trimmedID)
			fmt.Printf("Module files moved to: %s\n", newModulePath)
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

func handleAddTemplate(store storage.DataStore, moduleID, templateName string) {
	fmt.Printf("\nAdding template '%s' to module ID: %s\n", templateName, moduleID)

	// 1. Load the module metadata
	module, err := store.LoadModule(moduleID)
	if err != nil {
		log.Fatalf("Error loading module metadata for ID %s: %v", moduleID, err)
	}

	// 2. Check if template name already exists in metadata
	for _, t := range module.Templates {
		if t.Name == templateName {
			log.Fatalf("Error: Template '%s' already exists in module %s metadata.", templateName, moduleID)
		}
	}

	// 3. Call the generator to create the physical template file
	modulesDir := filepath.Dir(module.Directory) // Get the parent dir (e.g., "modules")
	err = generator.AddTemplateToModule(moduleID, templateName, modulesDir)
	if err != nil {
		log.Fatalf("Error creating template file via generator: %v", err)
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
	templateSubDir := "templates"
	relativePath := filepath.Join(templateSubDir, templateName)
	newTemplate := model.Template{
		Name:   templateName,
		Path:   relativePath,
		IsBase: false,
		Order:  newOrder,
	}

	// 6. Append to module's template list in metadata
	module.Templates = append(module.Templates, newTemplate)
	module.LastUpdated = time.Now()

	// 7. Save updated module metadata
	if err := store.SaveModule(module); err != nil {
		log.Fatalf("Error updating module metadata for ID %s after adding template: %v", moduleID, err)
	}

	fmt.Printf("Template '%s' added successfully and metadata updated with order %d.\n", templateName, newOrder)
}

func handlePurgeRemovedModules(store storage.DataStore) {
	fmt.Println("\nAttempting to purge all removed modules...")

	modules, err := store.ReadAll()
	if err != nil {
		log.Fatalf("Error reading module metadata for purge: %v", err)
	}

	removedModules := make([]*model.Module, 0)
	for _, mod := range modules {
		if !mod.IsActive { // Find inactive modules
			removedModules = append(removedModules, mod)
		}
	}

	if len(removedModules) == 0 {
		fmt.Println("No inactive modules found. Nothing to purge.")
		return
	}

	fmt.Printf("WARNING: You are about to permanently delete the files and metadata for %d inactive module(s).\n", len(removedModules))
	for _, mod := range removedModules {
		fmt.Printf("  - %s (%s) - Marked as Inactive\n", mod.Name, mod.ID)
	}
	if !askForConfirmation("Are you sure you want to proceed?") {
		fmt.Println("Operation cancelled.")
		return
	}

	purgedCount := 0
	failedDirDelete := 0
	failedMetaDelete := 0

	// Iterate only over the pre-filtered removed modules
	for _, module := range removedModules {
		fmt.Printf("Purging removed module '%s' (ID: %s)...\n", module.Name, module.ID)

		// 1. Attempt to delete the directory
		if module.Directory != "" {
			if _, err := os.Stat(module.Directory); err == nil {
				fmt.Printf("  Deleting directory: %s\n", module.Directory)
				err = os.RemoveAll(module.Directory)
				if err != nil {
					log.Printf("  Warning: Failed to delete directory %s: %v", module.Directory, err)
					failedDirDelete++
				}
			} else if !os.IsNotExist(err) {
				// Log error if stat failed for reasons other than not existing
				log.Printf("  Warning: Could not stat module directory %s before purge: %v", module.Directory, err)
				failedDirDelete++
			} else {
				fmt.Printf("  Directory %s not found, skipping delete.\n", module.Directory)
			}
		}

		// 2. Attempt to delete the metadata (even if directory deletion failed/skipped)
		fmt.Printf("  Deleting metadata for ID: %s\n", module.ID)
		err = store.DeleteModule(module.ID)
		if err != nil {
			log.Printf("  Error: Failed to delete metadata for module ID %s: %v", module.ID, err)
			failedMetaDelete++
		} else {
			purgedCount++
		}
	}

	fmt.Println("\nPurge complete.")
	fmt.Printf("Successfully purged metadata for %d modules.\n", purgedCount)
	if failedDirDelete > 0 {
		fmt.Printf("Warning: Failed to delete directories for %d modules (check logs above).\n", failedDirDelete)
	}
	if failedMetaDelete > 0 {
		fmt.Printf("Error: Failed to delete metadata for %d modules (check logs above).\n", failedMetaDelete)
	}
	if purgedCount == 0 && failedDirDelete == 0 && failedMetaDelete == 0 {
		fmt.Println("No inactive modules found.")
	}
}

// --- New: Handler function for update command ---
func handleUpdateModule(store storage.DataStore, moduleID, newName string) {
	fmt.Printf("\nUpdating module ID: %s\n", moduleID)

	// 1. Load the module metadata
	module, err := store.LoadModule(moduleID)
	if err != nil {
		log.Fatalf("Error loading module metadata for ID %s: %v", moduleID, err)
	}

	// 2. Update fields
	oldName := module.Name
	module.Name = newName
	module.LastUpdated = time.Now()

	// 3. Save updated module metadata
	if err := store.SaveModule(module); err != nil {
		log.Fatalf("Error saving updated module metadata for ID %s: %v", moduleID, err)
	}

	fmt.Printf("Successfully updated module %s. Renamed from '%s' to '%s'.\n", moduleID, oldName, newName)
}
