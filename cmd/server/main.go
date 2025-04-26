package main

import (
	"bytes" // Added for rendering sub-templates
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort" // Added for sorting
	"strings"
	"sync" // Added for map concurrency
	"time"

	"go-module-builder/internal/model"
	"go-module-builder/internal/storage"
)

// Global variable to hold loaded modules
var loadedModules []*model.Module

// Global variable to hold the base layout templates
var baseTemplates *template.Template

// Global map to hold template sets for each module (ModuleID -> *template.Template)
var moduleTemplates map[string]*template.Template
var moduleTemplatesMutex sync.RWMutex // Mutex for safe concurrent access

var projectRoot string

// Global variable to store the state of the module list toggle
var isModuleListEnabled bool

// Constants for certificate files
const (
	certFile = "cert.pem"
	keyFile  = "key.pem"
)

// PageData holds the data passed to the main layout and page templates
// for a specific module page
type PageData struct {
	Module          *model.Module
	RenderedContent template.HTML // Pre-rendered HTML of sorted sub-templates
}

// LayoutData holds the data passed to the main layout template
type LayoutData struct {
	IsModuleListEnabled bool
	PageContent         any // Can be nil, []*model.Module, or PageData
}

// generateSelfSignedCert creates a self-signed certificate and key file.
func generateSelfSignedCert(certPath, keyPath string) error {
	log.Printf("Generating self-signed certificate and key: %s, %s", certPath, keyPath)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	notBefore := time.Now()
	// Set validity to 10 years for simplicity, adjust as needed
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Self-Signed Org"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,

		// Use localhost and 127.0.0.1 as default DNS names/IPs
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Create certificate file
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", certPath, err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		certOut.Close() // Ensure file is closed even on error
		return fmt.Errorf("failed to write data to %s: %w", certPath, err)
	}
	if err := certOut.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", certPath, err)
	}
	log.Printf("Successfully generated %s", certPath)

	// Create key file
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) // Restrictive permissions
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", keyPath, err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		keyOut.Close()
		return fmt.Errorf("unable to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		keyOut.Close()
		return fmt.Errorf("failed to write data to %s: %w", keyPath, err)
	}
	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", keyPath, err)
	}
	log.Printf("Successfully generated %s", keyPath)

	return nil
}

func main() {
	// 1. Define and parse command-line flags
	port := flag.String("port", "8443", "Port to listen on for HTTPS") // Default to 8443 for HTTPS
	toggleModuleList := flag.Bool("toggle-module-list", false, "Toggle the /modules/list page (default: disabled)")
	flag.Parse()

	// Store the flag state globally
	isModuleListEnabled = *toggleModuleList

	// Log module list page status
	if isModuleListEnabled {
		log.Println("Module list page enabled at /modules/list")
	} else {
		log.Println("Module list page is disabled. Use -toggle-module-list to enable it.")
	}

	// Get working directory and store globally
	var err error
	projectRoot, err = os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	staticDir := filepath.Join(projectRoot, "web", "static")
	metadataDir := filepath.Join(projectRoot, ".module_metadata")  // Path to metadata
	templatesDir := filepath.Join(projectRoot, "web", "templates") // Path to main templates
	modulesDir := filepath.Join(projectRoot, "modules")            // Path to modules directory
	log.Printf("Serving static files from: %s", staticDir)
	log.Printf("Loading module metadata from: %s", metadataDir)
	log.Printf("Loading layout templates from: %s", templatesDir)

	// Ensure static directory exists (optional, but good practice)
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		log.Printf("Warning: Could not create static directory %s: %v", staticDir, err)
	}

	// --- Module Discovery ---
	store, err := storage.NewJSONStore(metadataDir)
	if err != nil {
		// Log non-fatal error if metadata dir doesn't exist yet
		if os.IsNotExist(err) {
			log.Printf("Metadata directory not found at %s. No modules loaded.", metadataDir)
			loadedModules = make([]*model.Module, 0) // Initialize as empty slice
		} else {
			log.Fatalf("Error initializing storage: %v", err)
		}
	} else {
		loadedModules, err = store.ReadAll()
		if err != nil {
			log.Printf("Warning: Error reading module metadata: %v", err)
			loadedModules = make([]*model.Module, 0) // Initialize as empty on error
		}
	}

	log.Printf("Discovered %d modules:", len(loadedModules))
	for _, mod := range loadedModules {
		log.Printf("  - ID: %s, Name: %s, Status: %s", mod.ID, mod.Name, mod.Status)
	}
	// --- End Module Discovery ---

	// Initialize map
	moduleTemplates = make(map[string]*template.Template)

	// --- Template Parsing (Revised with Cloning) ---
	log.Println("Parsing templates...")

	// 1. Parse base/layout templates first
	layoutPattern := filepath.Join(templatesDir, "*.html")
	layoutFiles, err := filepath.Glob(layoutPattern)
	if err != nil || len(layoutFiles) == 0 {
		log.Fatalf("Error finding or no layout templates found matching %s: %v", layoutPattern, err)
		// Cannot proceed without layout
	} else {
		log.Printf("Parsing base layout templates: %v", layoutFiles)
		baseTemplates, err = template.ParseFiles(layoutFiles...)
		if err != nil {
			log.Fatalf("Error parsing base layout templates: %v", err)
		}
	}

	// 2. For each active module, clone base templates and parse module templates into the clone
	for _, mod := range loadedModules {
		if mod.Status == "active" {
			moduleTemplatesDir := filepath.Join(modulesDir, mod.ID, "templates")

			// Find all relevant template files (.html, .tmpl, .css)
			htmlPattern := filepath.Join(moduleTemplatesDir, "*.[th][mt][lm]l") // *.html, *.tmpl
			cssPattern := filepath.Join(moduleTemplatesDir, "*.css")

			htmlFiles, errHtml := filepath.Glob(htmlPattern)
			cssFiles, errCss := filepath.Glob(cssPattern)

			if errHtml != nil {
				log.Printf("Warning: Error finding html/tmpl templates for module %s (%s): %v", mod.Name, mod.ID, errHtml)
			}
			if errCss != nil {
				log.Printf("Warning: Error finding css templates for module %s (%s): %v", mod.Name, mod.ID, errCss)
			}

			moduleFiles := append(htmlFiles, cssFiles...)

			if len(moduleFiles) > 0 {
				log.Printf("Preparing templates for module %s from files: %v", mod.ID, moduleFiles)

				// --- Modification Start ---
				// 1. Parse module files into a temporary, separate set first
				moduleSet := template.New(mod.ID) // Create a new set for the module
				moduleSet, err := moduleSet.ParseFiles(moduleFiles...)
				if err != nil {
					log.Printf("ERROR: Failed to parse module templates for %s: %v", mod.ID, err)
					continue // Skip this module if parsing fails
				}

				// 2. Clone the base template set
				clonedTemplates, err := baseTemplates.Clone()
				if err != nil {
					log.Printf("ERROR: Failed to clone base templates for module %s: %v", mod.ID, err)
					continue // Skip this module if cloning fails
				}

				// 3. Add the successfully parsed module templates to the cloned set
				for _, tmpl := range moduleSet.Templates() {
					if tmpl.Name() == mod.ID { // Skip the top-level template container itself
						continue
					}
					addTmpl, err := clonedTemplates.AddParseTree(tmpl.Name(), tmpl.Tree)
					if err != nil {
						log.Printf("ERROR: Failed to add template '%s' from module %s to cloned set: %v", tmpl.Name(), mod.ID, err)
						// Decide if we should continue or fail the whole module preparation
						// For now, let's log and continue, but this might lead to runtime errors
						continue
					}
					clonedTemplates = addTmpl // Update clonedTemplates with the result of AddParseTree
				}

				// Parse module files into the *cloned* set
				clonedTemplates, err = clonedTemplates.ParseFiles(moduleFiles...)
				if err != nil {
					// --- Log the specific error more clearly ---
					log.Printf("CRITICAL: Failed to parse templates for module %s (%s). The specific error was: %v", mod.Name, mod.ID, err)
					log.Printf("CRITICAL: Module %s will NOT be available.", mod.ID)
					// --- End logging change ---
					continue // Skip this module if parsing fails
				}
				// --- Modification End ---

				// Store the completed template set for this module
				moduleTemplatesMutex.Lock()
				moduleTemplates[mod.ID] = clonedTemplates // Store the combined set
				moduleTemplatesMutex.Unlock()
				log.Printf("Successfully prepared templates for module %s", mod.ID)

			} else {
				log.Printf("No template files (.html, .tmpl, .css) found for active module %s in %s", mod.ID, moduleTemplatesDir)
			}
		}
	}
	log.Println("Finished template preparation.")
	// --- End Template Parsing ---

	// 2. Create a new ServeMux (router)
	mux := http.NewServeMux()

	// 3. Setup static file servers
	// Main static files
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))
	// Module static files
	mux.HandleFunc("/modules/", handleModuleStaticRequest) // Add handler for module static files

	// 4. Define page handlers
	mux.HandleFunc("/", handleRootRequest)
	mux.HandleFunc("/view/module/", handleModulePageRequest) // Renamed handler and changed path

	// Conditionally register the module list handler
	if isModuleListEnabled {
		mux.HandleFunc("/modules/list", handleModuleListRequest)
	}

	// 5. Check for certs, generate if needed, and start HTTPS server
	certPath := filepath.Join(projectRoot, certFile)
	keyPath := filepath.Join(projectRoot, keyFile)

	// Check if both files exist
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	if os.IsNotExist(certErr) || os.IsNotExist(keyErr) {
		log.Println("Certificate or key file not found.")
		err = generateSelfSignedCert(certPath, keyPath)
		if err != nil {
			log.Fatalf("Failed to generate self-signed certificate/key: %v", err)
		}
	} else if certErr != nil || keyErr != nil {
		// Handle other errors during stat (e.g., permission issues)
		log.Fatalf("Error checking certificate/key files: certErr=%v, keyErr=%v", certErr, keyErr)
	} else {
		log.Printf("Using existing certificate and key files: %s, %s", certPath, keyPath)
	}

	// Log warning about self-signed certs
	log.Println("--------------------------------------------------------------------")
	log.Println("WARNING: Starting server with a self-signed certificate.")
	log.Println("Your browser will likely show security warnings (e.g., NET::ERR_CERT_AUTHORITY_INVALID).")
	log.Println("This is expected. You may need to click 'Advanced' and 'Proceed' to access the site.")
	log.Println("For production use, configure a proper reverse proxy (like Caddy) with valid certificates.")
	log.Println("--------------------------------------------------------------------")

	// Start the HTTPS server
	addr := ":" + *port
	fmt.Printf("Starting HTTPS server on https://localhost%s\n", addr) // Update protocol in message
	log.Printf("Listening on port %s...", *port)

	err = http.ListenAndServeTLS(addr, certPath, keyPath, mux) // Use ListenAndServeTLS
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// handleRootRequest serves the main layout for the root path
func handleRootRequest(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if baseTemplates == nil {
		http.Error(w, "Internal Server Error - Base templates not loaded", http.StatusInternalServerError)
		return
	}

	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	layoutData := LayoutData{
		IsModuleListEnabled: isModuleListEnabled,
		PageContent:         nil, // No specific content for root page
	}

	if isHTMX {
		log.Println("HTMX request detected for root. Rendering fragment and clearing module header.")
		// Clear the module header using OOB swap
		_, err := w.Write([]byte(`<span id="module-header-info" hx-swap-oob="innerHTML"></span>`))
		if err != nil {
			log.Printf("Error writing OOB header clear for root: %v", err)
			// Don't necessarily stop, try rendering main content anyway
		}
		// Render only the default page content block (or a specific home fragment if defined)
		err = baseTemplates.ExecuteTemplate(w, "page", layoutData) // Pass LayoutData
		if err != nil {
			log.Printf("Error executing page template for root (HTMX): %v", err)
			// Avoid writing generic error if header already sent
			if !strings.Contains(err.Error(), "multiple response.WriteHeader calls") {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	} else {
		// Execute the full layout template directly from the base set for standard requests
		log.Println("Standard request for root. Rendering full layout.html")
		err := baseTemplates.ExecuteTemplate(w, "layout.html", layoutData) // Pass LayoutData
		if err != nil {
			log.Printf("Error executing layout template for root: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// handleModuleListRequest serves the list of modules
func handleModuleListRequest(w http.ResponseWriter, r *http.Request) {
	if baseTemplates == nil {
		http.Error(w, "Internal Server Error - Base templates not loaded", http.StatusInternalServerError)
		return
	}

	// Filter only active modules for listing
	activeModules := make([]*model.Module, 0)
	for _, mod := range loadedModules {
		if mod.Status == "active" {
			activeModules = append(activeModules, mod)
		}
	}

	layoutData := LayoutData{
		IsModuleListEnabled: isModuleListEnabled,
		PageContent:         activeModules, // Pass active modules as PageContent
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	log.Println("Rendering module list page (/modules/list)")
	err := baseTemplates.ExecuteTemplate(w, "layout.html", layoutData) // Pass LayoutData
	if err != nil {
		log.Printf("Error executing layout template for module list: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleModulePageRequest serves a specific module's main page
func handleModulePageRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Extract Module ID from URL path /view/module/{id}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) != 3 || pathParts[0] != "view" || pathParts[1] != "module" {
		http.NotFound(w, r)
		return
	}
	moduleID := pathParts[2]

	// 2. Find the module by ID
	var targetModule *model.Module
	for _, mod := range loadedModules {
		if mod.ID == moduleID {
			targetModule = mod
			break
		}
	}

	// 3. Handle not found or inactive module
	if targetModule == nil {
		log.Printf("Module with ID %s not found", moduleID)
		http.NotFound(w, r)
		return
	}
	if targetModule.Status != "active" {
		log.Printf("Module %s (%s) is not active (status: %s)", targetModule.Name, moduleID, targetModule.Status)
		http.Error(w, "Module not available", http.StatusForbidden)
		return
	}

	// 4. Get the specific template set for this module
	moduleTemplatesMutex.RLock()
	moduleSpecificTemplates, ok := moduleTemplates[moduleID]
	moduleTemplatesMutex.RUnlock()

	if !ok {
		log.Printf("Template set not found for module %s. Was it parsed correctly?", moduleID)
		http.Error(w, "Internal Server Error - Module templates not loaded", http.StatusInternalServerError)
		return
	}

	// 5. Prepare data: Filter, sort, and pre-render sub-templates
	var renderableTemplates []model.Template
	for _, t := range targetModule.Templates {
		if !t.IsBase && (strings.HasSuffix(t.Name, ".html") || strings.HasSuffix(t.Name, ".tmpl")) {
			renderableTemplates = append(renderableTemplates, t)
		}
	}
	sort.SliceStable(renderableTemplates, func(i, j int) bool {
		return renderableTemplates[i].Order < renderableTemplates[j].Order
	})

	// Render sorted templates into a buffer
	var renderedContentBuf bytes.Buffer
	for _, tmplToRender := range renderableTemplates {
		// Derive the defined template name from the filename (e.g., "card.html" -> "card")
		definedName := strings.TrimSuffix(tmplToRender.Name, filepath.Ext(tmplToRender.Name))

		log.Printf("Rendering sub-template with defined name: %s (from file: %s) for module %s", definedName, tmplToRender.Name, moduleID)
		// Execute using the derived definedName
		err := moduleSpecificTemplates.ExecuteTemplate(&renderedContentBuf, definedName, targetModule) // Pass Module data
		if err != nil {
			log.Printf("ERROR rendering sub-template '%s' for module %s: %v", definedName, moduleID, err)
			// Decide how to handle partial failures. For now, log and continue.
			// You might want to return an error to the user instead.
			// Append an error message to the buffer?
			// renderedContentBuf.WriteString(fmt.Sprintf("<p>Error rendering %s</p>", definedName))
		} else {
			// Add a newline or separator if desired between rendered templates
			// renderedContentBuf.WriteString("\n")
		}
	}

	pageData := PageData{
		Module:          targetModule,
		RenderedContent: template.HTML(renderedContentBuf.String()), // Mark as safe HTML
	}
	log.Printf("Prepared %d sorted templates, rendered into combined content for module %s page", len(renderableTemplates), moduleID)

	layoutData := LayoutData{
		IsModuleListEnabled: isModuleListEnabled,
		PageContent:         pageData, // Pass the module-specific PageData
	}

	// 6. Determine if it's an HTMX request
	isHTMX := r.Header.Get("HX-Request") == "true"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX {
		log.Printf("HTMX request detected for module %s. Rendering OOB header and page fragment.", moduleID)

		// Render OOB header swap first
		headerSwapHTML := fmt.Sprintf(`<span id="module-header-info" hx-swap-oob="innerHTML">Module: %s</span>`, template.HTMLEscapeString(targetModule.Name))
		_, err := w.Write([]byte(headerSwapHTML))
		if err != nil {
			log.Printf("Error writing OOB header swap for module %s: %v", moduleID, err)
			return // Stop processing if header write fails
		}

		// Render the main content ('page' block) passing PageContent as context
		err = moduleSpecificTemplates.ExecuteTemplate(w, "page", layoutData.PageContent)
		if err != nil {
			log.Printf("Error executing 'page' template for module %s (HTMX): %v", moduleID, err)
			return
		}
		log.Printf("Successfully rendered OOB header and page fragment for module %s.", moduleID)

	} else {
		// Standard request: Render the full layout
		log.Printf("Standard request for module %s. Rendering full layout: layout.html", moduleID)
		err := moduleSpecificTemplates.ExecuteTemplate(w, "layout.html", layoutData) // Pass LayoutData
		if err != nil {
			log.Printf("Error executing 'layout.html' template for module %s: %v", moduleID, err)
			if strings.Contains(err.Error(), "template\" is undefined") {
				http.Error(w, "Internal Server Error - Module template missing", http.StatusInternalServerError)
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	}
}

// handleModuleStaticRequest serves static files from a module's directory
func handleModuleStaticRequest(w http.ResponseWriter, r *http.Request) {
	// Expected path: /modules/{module_id}/static/{file_path}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")

	// Basic validation: must have at least 4 parts (modules, id, static, filename)
	if len(pathParts) < 4 || pathParts[0] != "modules" || pathParts[2] != "static" {
		http.NotFound(w, r)
		return
	}

	moduleID := pathParts[1]
	// Join the remaining parts to get the relative file path
	relativeFilePath := filepath.Join(pathParts[3:]...)

	// Construct the actual file path on disk
	// NOTE: Files are currently generated into the 'templates' subdir by the generator
	// Adjust this path if the generator changes where it puts static assets
	filePath := filepath.Join(projectRoot, "modules", moduleID, "templates", relativeFilePath)

	// Check if file exists and serve it
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Static file not found for module %s: %s (requested path: %s)", moduleID, filePath, r.URL.Path)
		http.NotFound(w, r)
		return
	}

	log.Printf("Serving static file for module %s: %s", moduleID, filePath)
	http.ServeFile(w, r, filePath)
}
