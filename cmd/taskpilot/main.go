package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/relentlessworks/taskpilot/internal/api"
	"github.com/relentlessworks/taskpilot/internal/auth"
	"github.com/relentlessworks/taskpilot/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "taskpilot.json", "path to data file")
	tokenSecret := flag.String("secret", "", "secret for signing tokens (defaults to random)")
	flag.Parse()

	// Layered config: defaults < env < flags
	// Flags take priority if explicitly set; otherwise env overrides defaults
	if v := os.Getenv("TASKPILOT_ADDR"); v != "" && *addr == ":8080" {
		*addr = v
	}
	if v := os.Getenv("TASKPILOT_DB"); v != "" && *dbPath == "taskpilot.json" {
		*dbPath = v
	}
	if v := os.Getenv("TASKPILOT_SECRET"); v != "" && *tokenSecret == "" {
		*tokenSecret = v
	}

	// Initialize store
	db, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize auth
	authSvc := auth.New(*tokenSecret)

	// Initialize API server
	srv := api.NewServer(db, authSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.Router)

	log.Printf("TaskPilot listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
