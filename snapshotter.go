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

	// Subcommand dispatch.
	//   snapshotter                        -> snapshot the working directory
	//   snapshotter <filename>             -> open the interactive history viewer
	//   snapshotter restore <id> <path>    -> restore one file to a commit
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "restore":
			runRestore(engine, os.Args[2:])
			return
		case "-h", "--help", "help":
			printUsage()
			return
		default:
			runHistory(engine, workDir, os.Args[1:])
			return
		}
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
		fmt.Printf("  %s %s\n", changeGlyph(change.Action), describeChange(change))
	}
}

// changeGlyph returns the compact per-line marker for an action kind.
func changeGlyph(action store.ActionType) string {
	switch action {
	case store.ActionCreate:
		return "[+]"
	case store.ActionModify:
		return "[~]"
	case store.ActionDelete:
		return "[-]"
	case store.ActionIgnored:
		return "[i]"
	case store.ActionMove:
		return "[>]"
	default:
		return "[?]"
	}
}

// describeChange renders a single change as a "(action) path" line.
func describeChange(change store.Change) string {
	if change.Action == store.ActionMove {
		return fmt.Sprintf("(moved) %s -> %s", change.OldPath, change.Path)
	}
	return fmt.Sprintf("(%s) %s", labelFor(change.Action), change.Path)
}

// labelFor maps an action kind to its lowercase worded label.
func labelFor(action store.ActionType) string {
	switch action {
	case store.ActionCreate:
		return "created"
	case store.ActionModify:
		return "modified"
	case store.ActionDelete:
		return "deleted"
	case store.ActionIgnored:
		return "ignored"
	case store.ActionMove:
		return "moved"
	default:
		return string(action)
	}
}

// printUsage writes a short usage summary to standard error.
func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: snapshotter [<filename> | restore <commit-id> <path> | help]")
	fmt.Fprintln(os.Stderr, "  (no argument)               snapshot the current working directory")
	fmt.Fprintln(os.Stderr, "  <filename>                  open the interactive history viewer for a file")
	fmt.Fprintln(os.Stderr, "  restore <commit-id> <path>  restore a single file to its state at a commit")
}

// runRestore implements `snapshotter restore <commit-id> <path>`: it resets a
// single file to the content it held at the given commit.
func runRestore(engine *store.Engine, args []string) {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: snapshotter restore <commit-id> <path>")
		os.Exit(2)
	}
	commitID, path := args[0], args[1]

	result, err := engine.Restore(commitID, path)
	if err != nil {
		log.Fatalf("Restore failed: %v", err)
	}

	if result.BackupPath != "" {
		fmt.Printf("Backed up divergent content to: %s\n", result.BackupPath)
	}
	if !result.Restored {
		fmt.Printf("%s is already at its state for commit %s (no changes).\n", path, commitID)
		return
	}
	fmt.Printf("Restored %s to commit %s.\n", path, commitID)
}
