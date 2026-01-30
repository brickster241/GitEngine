package main

import (
	"fmt"
	"os"

	"github.com/brickster241/GitEngine/porcelain"
)

// Entry point of the application - Check for all commands.
func main() {

	// When you create a build, the first argument is always the name of the executable.
	if len(os.Args) == 1 {

		// No arguments provided
		fmt.Printf("gegit: command cannot be empty. See 'gegit help' for available commands.\n")
		fmt.Println("usage: gegit <command> [<args>]")
		os.Exit(0)
	}
	switch os.Args[1] {

	case "init":
		// Initialize a new repository
		porcelain.InitRepo(os.Args[1:])
	case "add":
		// Add files to the staging area / index
		porcelain.AddFiles(os.Args[1:])
	case "status":
		// Show the working tree status
		porcelain.ShowStatus(os.Args[1:])
	case "commit":
		// Commit changes to the repository
		porcelain.CommitChanges(os.Args[1:])
	case "config":
		// Get or Set keys in .git/config
		porcelain.GetOrSetConfig(os.Args[1:])
	case "cat-file":
		// Show type, size and content for repository objects
		porcelain.CatFileRepoObject(os.Args[1:])
	case "hash-object":
		// Compute object id from a file
		porcelain.HashAndWriteObject(os.Args[1:])
	case "update-index":
		// Register file contents in the working tree to the index
		porcelain.RegisterFileAndUpdateIndex(os.Args[1:])
	default:
		// Command not found
		fmt.Printf("gegit: '%s' is not a git command. See 'gegit help' for available commands.\n", os.Args[1])
		fmt.Println("usage: gegit <command> [<args>]")
	}
}
