package porcelain

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/types"
)

// Invoked from main.go. LSTree handles the 'gegit ls-tree' command to list the contents of a tree object. It only calls this function if first argument is "ls-tree".
func LSTree(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("ls-tree",
		"Lists the contents of a given tree-ish object (commit or tree), like what \"/bin/ls -a\" does in the current working directory. ",
		"gegit ls-tree [-d] [-r] [-t] <tree-ish>")
	d := fls.Bool("d", false, "Show only the named tree entry itself, not its children.")
	r := fls.Bool("r", false, "Recurse into sub-trees.")
	t := fls.Bool("t", false, "Show tree entries even when going to recurse them. Has no effect if -r was not passed. -d implies -t.")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// Check if there is only one non-flag argument i.e. treeish
	if len(pos) != 1 {
		fmt.Println("usage: gegit ls-tree [-d] [-r] [-t] <tree-ish>")
		os.Exit(1)
	}

	// Resolve Treeish object
	treeSHA, err := plumbing.ResolveTreeish(pos[0])
	if err != nil {
		fmt.Printf("Error resolving tree object %s:%s\n", pos[0], err)
		os.Exit(1)
	}

	// Get All Entries (recursive) for this treeSHA.
	treeEntries, err := plumbing.FlattenTree(treeSHA)
	if err != nil {
		fmt.Printf("Error flattening tree object %s: %s\n", pos[0], err)
		os.Exit(1)
	}

	// Result Entries
	resultEntries := []types.TreeEntry{}

	// Based on flags, filter the output. Default behavior is similar to cat-file.
	for path, te := range treeEntries {

		// If -r flag is present
		if *r {

			// If -d flag is present, only add the trees recursively. -t flag doesn't matter (-r-d, -r-d-t)
			if *d {
				if te.Type == types.TreeObject {
					resultEntries = append(resultEntries, te)
				}
				continue
			}
			// If -t flag is present, add everything recursively including trees. (-r-t)
			if *t {
				resultEntries = append(resultEntries, te)
				continue
			}
			// If no other flags are present, add everything recursively except trees (-r)
			if te.Type != types.TreeObject {
				resultEntries = append(resultEntries, te)
			}
			continue
		}

		// If -d flag is present, but -r is not present. Irrespective of -t flag, only show tree entries at current Depth (-d-t, -d)
		if *d {
			if te.Type == types.TreeObject && filepath.Base(path) == path {
				resultEntries = append(resultEntries, te)
			}
			continue
		}

		// Irrespective whether -t flag is present, but -r and -d are not present. Show all entries at current level. (-t, )
		if filepath.Base(path) == path {
			resultEntries = append(resultEntries, te)
			continue
		}
	}

	// Sort the result based on Path / Filename
	sort.Slice(resultEntries, func(i, j int) bool {
		return resultEntries[i].Name < resultEntries[j].Name
	})

	// Print the resultEntries
	for _, e := range resultEntries {
		fmt.Printf("%06o %s %x\t%s\n",
			e.Mode, e.Type, e.SHA, e.Name)
	}
}
