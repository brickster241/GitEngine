package porcelain

import (
	"fmt"
	"os"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/types"
)

// Invoked from main.go. ReadTreeToIndex handles the 'gegit read-tree' command to read a treeish object and write it to the current index.
func ReadTreeToIndex(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("read-tree",
		"Reads the tree information given by <tree-ish> into the index, but does not actually update any of the files it 'caches'.",
		"gegit read-tree <tree-ish>")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// Check if there is exactly one pos argument i.e. <tree-ish>
	if len(pos) != 1 {
		fmt.Println("usage: gegit read-tree <tree-ish>")
		os.Exit(1)
	}

	// Get the resolved treeSHA
	treeSHA, err := plumbing.ResolveTreeish(pos[0])
	if err != nil {
		fmt.Printf("Error resolving tree-ish object %s: %s\n", pos[0], err)
		os.Exit(1)
	}

	// Get Tree Entries and convert them to []IndexEntry
	treeEntries, err := plumbing.FlattenTree(treeSHA)
	if err != nil {
		fmt.Println("Error fetching Tree contents:", err)
		os.Exit(1)
	}
	treeIndexEntries := []types.IndexEntry{}

	// Iterate through the entries and convert them to []IndexEntry, with default values for everything else. Only blobs will be used.
	for _, te := range treeEntries {
		if te.Type == types.BlobObject {
			treeIndexEntries = append(treeIndexEntries, types.IndexEntry{
				Filename: te.Name,
				SHA1:     te.SHA,
				Mode:     te.Mode,
			})
		}
	}

	// Write the Index based on these new []IndexEntry slice. Will automatically sort based on Filename.
	if err := plumbing.WriteIndex(treeIndexEntries); err != nil {
		fmt.Println("Error updating Index:", err)
		os.Exit(1)
	}
}
