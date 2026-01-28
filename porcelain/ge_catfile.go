package porcelain

import (
	"fmt"
	"os"
	"slices"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils/types"
)

// Invoked from main.go. CatFileObject handles the 'gegit cat-file' command to display type, size or content for a specific repo object.
func CatFileRepoObject(args []string) {

	// Check args length and whether acceptable flag is present.
	flags := []string{"-p", "-s", "-t"}
	if len(args) != 3 || !slices.Contains(flags, args[1]) {
		fmt.Println("usage: gegit cat-file -p|-s|-t <sha>")
		os.Exit(1)
	}

	// Get Object Type & Raw content
	objType, content, err := plumbing.ReadObject(args[2])
	if err != nil {
		fmt.Println("Error reading object:", err)
		return
	}

	// Parse flags
	switch args[1] {
	case "-s":
		// Print size
		fmt.Println(len(content))
	case "-t":
		// Print type
		fmt.Println(objType)
	case "-p":
		// Pretty print
		if objType != types.TreeObject {
			fmt.Println(string(content))
		} else {
			// ReadTree (single-level)
			entries, _ := plumbing.ReadTree(args[2])
			for _, e := range entries {
				fmt.Printf("%s %s %x\t%s\n",
					e.Mode, e.Type, e.SHA, e.Name)
			}
		}
	}
}
