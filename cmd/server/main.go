package main

import (
	// Keep only imports needed by main() and generateSelfSignedCert()
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"html/template"
	"log"      // Keep standard log for initial setup/fatal errors
	"log/slog" // Import slog
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath" // Keep for app struct initialization
	"time"

	"go-module-builder/internal/model"
	"go-module-builder/internal/storage"

	"github.com/spf13/viper"
)

// Struct definitions (application, PageData, LayoutData) are now in routes.go

// --- Main Function ---

func main() {
	// 1. Setup Viper configuration
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")      // look for config in the working directory
	viper.SetEnvPrefix("GOWS")    // Prefix for environment variables (optional)
	viper.AutomaticEnv()          // Read in environment variables that match

	// Set default values (optional, but good practice)
	viper.SetDefault("server.port", "8443")
	viper.SetDefault("server.certFile", "cert.pem")
	viper.SetDefault("server.keyFile", "key.pem")

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			log.Println("Warning: config.yaml not found, using defaults/flags/env vars.")
		} else {
			// Config file was found but another error was produced
			log.Fatalf("Fatal error reading config file: %v", err)
		}
	}

	// 2. Define and parse flags (still needed to potentially override config/defaults)
	// Note: We don't need the cfg struct anymore if using Viper directly
	portFlag := flag.String("port", viper.GetString("server.port"), "Port to listen on for HTTPS")
	certFileFlag := flag.String("cert-file", viper.GetString("server.certFile"), "Path to TLS certificate file")
	keyFileFlag := flag.String("key-file", viper.GetString("server.keyFile"), "Path to TLS key file")
	toggleModuleList := flag.Bool("toggle-module-list", false, "Toggle the /modules/list page (default: disabled)")
	flag.Parse()

	// Bind flags to Viper AFTER parsing, so flags take precedence
	// Note: Viper doesn't have direct binding for stdlib flag, this is manual override check
	// A more robust way uses spf13/pflag, but let's keep it simple for now.
	// We'll just use the flag values directly if they differ from the viper value (which includes config/defaults)
	finalPort := *portFlag
	finalCertFile := *certFileFlag
	finalKeyFile := *keyFileFlag

	// Log module list page status
	if *toggleModuleList {
		log.Println("Module list page enabled at /modules/list")
	} else {
		log.Println("Module list page is disabled. Use -toggle-module-list to enable it.")
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	projRoot := wd

	metadataDir := filepath.Join(projRoot, ".module_metadata")
	templatesDir := filepath.Join(projRoot, "web", "templates")
	modulesDir := filepath.Join(projRoot, "modules")
	log.Printf("Loading module metadata from: %s", metadataDir)
	log.Printf("Loading layout templates from: %s", templatesDir)
	log.Printf("Using modules directory: %s", modulesDir)

	// --- Module Discovery ---
	var modules []*model.Module
	store, err := storage.NewJSONStore(metadataDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Metadata directory not found at %s. No modules loaded.", metadataDir)
			modules = make([]*model.Module, 0)
		} else {
			log.Fatalf("Error initializing storage: %v", err)
		}
	} else {
		modules, err = store.ReadAll()
		if err != nil {
			log.Printf("Warning: Error reading module metadata: %v", err)
			modules = make([]*model.Module, 0)
		}
	}

	log.Printf("Discovered %d modules:", len(modules))
	for _, mod := range modules {
		statusStr := "Inactive"
		if mod.IsActive {
			statusStr = "Active"
		}
		log.Printf("  - ID: %s, Name: %s, Status: %s", mod.ID, mod.Name, statusStr)
	}
	// --- End Module Discovery ---

	// --- Template Parsing ---
	log.Println("Parsing templates...")
	modTemplates := make(map[string]*template.Template)

	// 1. Parse base/layout templates first
	layoutPattern := filepath.Join(templatesDir, "*.html")
	layoutFiles, err := filepath.Glob(layoutPattern)
	if err != nil || len(layoutFiles) == 0 {
		log.Fatalf("Error finding or no layout templates found matching %s: %v", layoutPattern, err)
	}
	log.Printf("Parsing base layout templates: %v", layoutFiles)
	baseTmpl, err := template.ParseFiles(layoutFiles...)
	if err != nil {
		log.Fatalf("Error parsing base layout templates: %v", err)
	}

	// 2. For each active module, clone base templates and parse module templates into the clone
	for _, mod := range modules {
		if mod.IsActive {
			moduleTemplatesDir := filepath.Join(modulesDir, mod.ID, "templates")
			htmlPattern := filepath.Join(moduleTemplatesDir, "*.[th][mt][lm]l")
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

				moduleSet := template.New(mod.ID)
				moduleSet, err := moduleSet.ParseFiles(moduleFiles...)
				if err != nil {
					log.Printf("ERROR: Failed to parse module templates for %s: %v", mod.ID, err)
					continue
				}

				clonedTemplates, err := baseTmpl.Clone()
				if err != nil {
					log.Printf("ERROR: Failed to clone base templates for module %s: %v", mod.ID, err)
					continue
				}

				for _, tmpl := range moduleSet.Templates() {
					if tmpl.Name() == mod.ID {
						continue
					}
					addTmpl, err := clonedTemplates.AddParseTree(tmpl.Name(), tmpl.Tree)
					if err != nil {
						log.Printf("ERROR: Failed to add template '%s' from module %s to cloned set: %v", tmpl.Name(), mod.ID, err)
						continue
					}
					clonedTemplates = addTmpl
				}

				clonedTemplates, err = clonedTemplates.ParseFiles(moduleFiles...)
				if err != nil {
					log.Printf("CRITICAL: Failed to parse templates for module %s (%s). The specific error was: %v", mod.Name, mod.ID, err)
					log.Printf("CRITICAL: Module %s will NOT be available.", mod.ID)
					continue
				}

				modTemplates[mod.ID] = clonedTemplates
				log.Printf("Successfully prepared templates for module %s", mod.ID)

			} else {
				log.Printf("No template files (.html, .tmpl, .css) found for active module %s in %s", mod.ID, moduleTemplatesDir)
			}
		}
	}
	log.Println("Finished template preparation.")
	// --- End Template Parsing ---

	// --- Initialize Logger ---
	logLevel := new(slog.LevelVar)                                                            // Create a variable to hold the level
	logLevel.Set(slog.LevelDebug)                                                             // Set the level to DEBUG
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})) // Pass options

	// --- Initialize Application Struct ---
	app := &application{ // application struct is defined in routes.go
		logger:              logger, // Pass logger
		projectRoot:         projRoot,
		isModuleListEnabled: *toggleModuleList,
		loadedModules:       modules,
		baseTemplates:       baseTmpl,
		moduleTemplates:     modTemplates,
		// Mutex is zero-value ready
	}

	// --- Create Router ---
	router := app.routes() // routes method is defined in routes.go
	// Removed 'if router == nil' check as app.routes() always returns a valid handler

	// --- Certificate Handling ---
	// Use final config values derived from Viper/Flags
	certPath := finalCertFile
	if !filepath.IsAbs(certPath) {
		certPath = filepath.Join(app.projectRoot, certPath)
	}
	keyPath := finalKeyFile
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(app.projectRoot, keyPath)
	}

	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	if os.IsNotExist(certErr) || os.IsNotExist(keyErr) {
		log.Println("Certificate or key file not found.")
		err = generateSelfSignedCert(certPath, keyPath) // Use local function
		if err != nil {
			log.Fatalf("Failed to generate self-signed certificate/key: %v", err)
		}
	} else if certErr != nil || keyErr != nil {
		log.Fatalf("Error checking certificate/key files: certErr=%v, keyErr=%v", certErr, keyErr)
	} else {
		log.Printf("Using existing certificate and key files: %s, %s", certPath, keyPath)
	}

	// --- Start Server ---
	log.Println("--------------------------------------------------------------------")
	log.Println("WARNING: Starting server with a self-signed certificate.")
	log.Println("Your browser will likely show security warnings (e.g., NET::ERR_CERT_AUTHORITY_INVALID).")
	log.Println("This is expected. You may need to click 'Advanced' and 'Proceed' to access the site.")
	log.Println("For production use, configure a proper reverse proxy (like Caddy) with valid certificates.")
	log.Println("--------------------------------------------------------------------")

	addr := ":" + finalPort // Use final port value
	fmt.Printf("Starting HTTPS server on https://localhost%s\n", addr)
	log.Printf("Listening on port %s...", finalPort) // Use final port value

	err = http.ListenAndServeTLS(addr, certPath, keyPath, router) // Pass the router
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// Router setup (createServerMux) is now in routes.go
// HTTP Handlers (handleRootRequest, etc.) are now in routes.go

// --- Utility Functions ---

// generateSelfSignedCert creates a self-signed certificate and key file.
// This function remains here as it doesn't depend on application state.
func generateSelfSignedCert(certPath, keyPath string) error {
	log.Printf("Generating self-signed certificate and key: %s, %s", certPath, keyPath)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	notBefore := time.Now()
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

		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", certPath, err)
	}
	defer certOut.Close() // Use defer for cleanup
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		// certOut.Close() // No need to close explicitly due to defer
		return fmt.Errorf("failed to write data to %s: %w", certPath, err)
	}
	// if err := certOut.Close(); err != nil { // No need to close explicitly due to defer
	// 	return fmt.Errorf("failed to close %s: %w", certPath, err)
	// }
	log.Printf("Successfully generated %s", certPath)

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", keyPath, err)
	}
	defer keyOut.Close() // Use defer for cleanup
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		// keyOut.Close() // No need to close explicitly due to defer
		return fmt.Errorf("unable to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		// keyOut.Close() // No need to close explicitly due to defer
		return fmt.Errorf("failed to write data to %s: %w", keyPath, err)
	}
	// if err := keyOut.Close(); err != nil { // No need to close explicitly due to defer
	// 	return fmt.Errorf("failed to close %s: %w", keyPath, err)
	// }
	log.Printf("Successfully generated %s", keyPath)

	return nil
}
