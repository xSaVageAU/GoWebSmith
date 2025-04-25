package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// 1. Define and parse command-line flags
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	// Get working directory to construct absolute paths
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	staticDir := filepath.Join(projectRoot, "web", "static")
	log.Printf("Serving static files from: %s", staticDir)

	// Ensure static directory exists (optional, but good practice)
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		log.Printf("Warning: Could not create static directory %s: %v", staticDir, err)
	}

	// 2. Create a new ServeMux (router)
	mux := http.NewServeMux()

	// 3. Setup static file server
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// 4. Define a simple handler for the root path
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Ensure this handler only responds to the exact root path
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Basic HTML structure linking the CSS
		fmt.Fprintf(w, "<!DOCTYPE html>\n")
		fmt.Fprintf(w, "<html>\n")
		fmt.Fprintf(w, "<head>\n")
		fmt.Fprintf(w, "  <title>Go Module Builder Server (Phase 2)</title>\n")
		fmt.Fprintf(w, "  <link rel=\"stylesheet\" href=\"/static/test.css\">\n") // Link the CSS
		fmt.Fprintf(w, "</head>\n")
		fmt.Fprintf(w, "<body>\n")
		fmt.Fprintf(w, "  <h1>Go Module Builder - Server Running!</h1>\n")
		fmt.Fprintf(w, "  <p>Listening on port %s</p>\n", *port)
		fmt.Fprintf(w, "  <p><a href=\"/static/test.css\">View Static CSS Source</a></p>")
		fmt.Fprintf(w, "</body>\n")
		fmt.Fprintf(w, "</html>\n")
	})

	// 5. Start the HTTP server
	addr := ":" + *port
	fmt.Printf("Starting server on http://localhost%s\n", addr)
	log.Printf("Listening on port %s...", *port)

	err = http.ListenAndServe(addr, mux)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
