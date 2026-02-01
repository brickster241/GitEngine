package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/types"
)

// Invoked from main.go. CatFileObject handles the 'gegit cat-file' command to display type, size or content for a specific repo object.
func CatFileRepoObject(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("cat-file",
		"Output the contents or other properties such as size, type or delta information of one or more objects.",
		"gegit cat-file (-p | -t | -s) <object>")
	pp := fls.Bool("p", false, "Pretty-print the contents of <object> based on its type.")
	size := fls.Bool("s", false, "Instead of the content, show the object size identified by <object>.")
	ty := fls.Bool("t", false, "Instead of the content, show the object type identified by <object>.")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// Check args length and only a single flag is present
	if len(pos) != 1 || !((*pp && !*size && !*ty) || (!*pp && *size && !*ty) || (!*pp && !*size && *ty)) {
		fmt.Println("usage: gegit cat-file (-p | -t | -s) <object>")
		os.Exit(1)
	}

	// Check whether it can be resolved to Commitish or Treeish object
	sha, err := plumbing.ResolveCommitish(pos[0])
	if err != nil {
		// Try tree-ish
		sha, err = plumbing.ResolveTreeish(pos[0])
		if err != nil {
			fmt.Println("fatal: Not a valid object name:", pos[0])
		}
	}

	// Get Object Type & Raw content
	var shaHex string
	if sha != [20]byte{} {
		shaHex = hex.EncodeToString(sha[:])
	} else {
		shaHex = pos[0]
	}
	objType, content, err := plumbing.ReadObject(shaHex)
	if err != nil {
		fmt.Println("Error reading object:", err)
		os.Exit(1)
	}

	// Parse flags
	if *size {
		// Print size
		fmt.Println(len(content))
	} else if *ty {
		// Print type
		fmt.Println(objType)
	} else if *pp {
		// Pretty print
		if objType != types.TreeObject {
			fmt.Println(string(content))
		} else {
			// ReadTree (single-level)
			entries, _ := plumbing.ReadTreeCurrentLevel(shaHex)
			for _, e := range entries {
				fmt.Printf("%06o %s %x\t%s\n",
					e.Mode, e.Type, e.SHA, e.Name)
			}
		}
	}
}
