package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/darccio/diffty/internal/server"
	"github.com/darccio/diffty/internal/storage"
)

func main() {
	// Command line flags
	port := flag.Int("port", 10101, "Port to run the server on")
	flag.Parse()

	// Initialize storage for review state
	store, err := storage.NewJSONStorage()
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Setup server and routes
	templateDir := filepath.Join("internal", "server", "templates")
	srv, err := server.New(store, templateDir)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting diffty server at http://localhost%s", addr)

	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
