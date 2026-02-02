package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
)

// Invoked from main.go. WriteTreeFromIndex handles the 'gegit write-tree' command to create a tree object from the current index and write it to object database.
func WriteTreeFromIndex(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("write-tree",
		"Creates a tree object using the current index. The name of the new tree object is printed to standard output.",
		"gegit write-tree")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// There should be no extra arguments
	if len(pos) != 0 {
		fmt.Println("usage: gegit write-tree")
		os.Exit(1)
	}

	// Load Index
	entries, err := plumbing.LoadIndex()
	if err != nil {
		fmt.Println("Error loading .git/index:", err)
		os.Exit(1)
	}

	// Index should not be empty
	if len(entries) == 0 {
		fmt.Println("Error: index should not be empty")
		os.Exit(1)
	}
	// Build Tree from index entries
	treeNode := plumbing.BuildTreeFromIndex(entries)

	// Write Tree into .git/objects
	treeSHA, err := plumbing.WriteTree(treeNode)
	if err != nil {
		fmt.Println("Error writing tree:", err)
		os.Exit(1)
	}

	// Output the written TreeSHA
	fmt.Println(hex.EncodeToString(treeSHA[:]))
}
