package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"snapshotter/internal/store"
)

func main() {
	// For now, default action is to snapshot the current working directory
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}

	snapshotDir := filepath.Join(workDir, ".snapshots")

	// Initialize the engine
	engine, err := store.NewEngine(workDir, snapshotDir)
	if err != nil {
		log.Fatalf("Failed to initialize snapshot engine: %v", err)
	}

	// Trigger a snapshot
	event, err := engine.Snapshot(workDir)
	if err != nil {
		log.Fatalf("Snapshot failed: %v", err)
	}

	// Output results
	if event == nil {
		fmt.Println("No changes detected.")
		return
	}

	fmt.Printf("Snapshot created: %s\n", event.ID)
	for _, change := range event.Changes {
		switch change.Action {
		case store.ActionCreate:
			fmt.Printf("  [+] %s\n", change.Path)
		case store.ActionModify:
			fmt.Printf("  [~] %s\n", change.Path)
		case store.ActionDelete:
			fmt.Printf("  [-] %s\n", change.Path)
		case store.ActionIgnored:
			fmt.Printf("  [i] %s\n", change.Path)
		case store.ActionMove:
			fmt.Printf("  [>] %s -> %s\n", change.OldPath, change.Path)
		}
	}
}
