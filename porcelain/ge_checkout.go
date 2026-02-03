package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
)

// Invoked from main.go. CheckoutCommit handles 'gegit checkout' command to switch branches or restore working tree files.
func CheckoutCommit(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("checkout",
		"Switch branches, with git checkout <branch> or Restore a different version of a file, for example with git checkout <commit> <filename> or git checkout <filename>.",
		"gegit checkout [-b <new-branch>] <commit-ish> [-- <path>]")
	b := fls.String("b", "", "Create a new branch named <new-branch>, start it at <start-point> (defaults to the current commit), and check out the new branch.")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// Store content which will be written in .git/HEAD
	var headContent string

	switch {
	case *b != "":
		var startPoint string
		// If branch is not empty, then exactly there should be one non-flag argument for startPoint commitish.
		if len(pos) == 0 {
			startPoint = "HEAD"
		} else if len(pos) != 1 {
			fmt.Println("usage: gegit checkout [-b <new-branch>] <commit-ish> [-- <path>]")
			os.Exit(1)
		} else {
			// Default startPoint is HEAD
			startPoint = pos[0]
		}

		// Resolve startPoint commitish
		commitSHA, err := plumbing.ResolveCommitish(startPoint)
		if err != nil {
			fmt.Printf("Error resolving commit-ish '%s': %s", startPoint, err)
			os.Exit(1)
		}

		// Create Branch with specified branchName and commitSHA
		if err := plumbing.CreateBranchRef(*b, commitSHA); err != nil {
			fmt.Println("Error creating branch:", err)
			os.Exit(1)
		}

		// Get TreeSHA from the commitSHA
		commit, err := plumbing.ReadCommit(commitSHA)
		if err != nil {
			fmt.Println("Error reading commit SHA:", err)
			os.Exit(1)
		}

		// Update WorkTree, HEAD and Index to TreeSHA. Branch Name is *b
		if err := plumbing.CheckoutToTreeSHA(commit.TreeSHA, "ref: "+filepath.Join("refs", "heads", *b)+"\n"); err != nil {
			fmt.Println("Error Checking out to Tree SHA:", err)
			os.Exit(1)
		}

	case len(pos) == 1:
		// Extract commitish string, keep track whether head should be detached or not.
		commitIsh := pos[0]
		var commitSHA [20]byte

		// Check whether commitIsh is an existing branch Name
		branchSHA, exists := plumbing.ReadBranchRef(commitIsh)
		if exists {
			// commitIsh is a valid branch name
			commitSHA = branchSHA
		} else {
			// Check if it is a valid commitIsh, HEAD wil be detached.
			SHA, err := plumbing.ResolveCommitish(commitIsh)
			if err != nil {
				fmt.Println("Error resolving commitIsh:", err)
				os.Exit(1)
			}
			commitSHA = SHA
		}

		// Get TreeSHA from the commitSHA
		commit, err := plumbing.ReadCommit(commitSHA)
		if err != nil {
			fmt.Println("Error reading commit SHA:", err)
			os.Exit(1)
		}

		// If branch exists, use HEAD not detached content
		if exists {
			headContent = "ref: " + filepath.Join("refs", "heads", commitIsh) + "\n"
		} else {
			// commitIsh is a valid HEX SHA string
			headContent = hex.EncodeToString(commitSHA[:]) + "\n"
		}

		// Update WorkTree, HEAD and Index to TreeSHA.
		if err := plumbing.CheckoutToTreeSHA(commit.TreeSHA, headContent); err != nil {
			fmt.Println("Error Checking out to Tree SHA:", err)
			os.Exit(1)
		}

	case len(pos) >= 2:
		// Checkout paths from commitish
		commitIsh := pos[0]
		var commitSHA [20]byte
		filePaths := pos[1:]

		// Check whether commitIsh is an existing branch Name
		branchSHA, exists := plumbing.ReadBranchRef(commitIsh)
		if exists {
			// commitIsh is a valid branch name
			commitSHA = branchSHA
		} else {
			// Check if it is a valid commitIsh, HEAD wil be detached.
			SHA, err := plumbing.ResolveCommitish(commitIsh)
			if err != nil {
				fmt.Println("Error resolving commitIsh:", err)
				os.Exit(1)
			}
			commitSHA = SHA
		}

		// Get TreeSHA from the commitSHA
		commit, err := plumbing.ReadCommit(commitSHA)
		if err != nil {
			fmt.Println("Error reading commit SHA:", err)
			os.Exit(1)
		}

		// Get All Tree Entries by Flatten Tree
		treeEntries, err := plumbing.FlattenTree(commit.TreeSHA)
		if err != nil {
			fmt.Println("Error fetching commit Tree entries:", err)
			os.Exit(1)
		}

		// Get current Index Entries
		indexEntries, err := plumbing.LoadIndex()
		if err != nil {
			fmt.Println("Error loading .git/index:", err)
			os.Exit(1)
		}
		// Generate a path to IndexEntries map
		indexEntryMap := plumbing.IndexToMap(indexEntries)

		// Iterate through each path in the list, and check whether the path exist in treeEntries and indexEntryMap.
		for _, fPath := range filePaths {
			cleanPath := filepath.ToSlash(filepath.Clean(fPath))

			// If cleanPath not present in the tree, remove from Index and remove from worktree(if exists)
			te, ok := treeEntries[cleanPath]
			if !ok {
				// Delete from Index , Worktree (if exists)
				delete(indexEntryMap, cleanPath)
				if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
					fmt.Printf("Error deleting %s from WorkTree: %s\n", cleanPath, err)
					os.Exit(1)
				}
			} else {
				// Write to path with updated blob content.
				shaHex := hex.EncodeToString(te.SHA[:])
				_, content, err := plumbing.ReadObject(shaHex)
				if err != nil {
					fmt.Printf("Error reading blob for file '%s':%s\n", cleanPath, err)
					os.Exit(1)
				}

				// Make parent directories if not present. Then write to file with updated content.
				if err := os.MkdirAll(filepath.Dir(cleanPath), constants.DefaultDirPerm); err != nil {
					fmt.Println("Error creating Directories:", err)
					os.Exit(1)
				}
				if err := os.WriteFile(cleanPath, content, constants.DefaultFilePerm); err != nil {
					fmt.Printf("Error writing to file '%s': %s\n", cleanPath, err)
					os.Exit(1)
				}

				// Update Index Entry if exists else create one.
				if ie, ok := indexEntryMap[cleanPath]; ok {
					ie.SHA1 = te.SHA
					ie.Mode = te.Mode
					ie.Filename = te.Name
				} else {
					indexEntryMap[cleanPath] = types.IndexEntry{
						SHA1:     te.SHA,
						Filename: te.Name,
						Mode:     te.Mode,
					}
				}
			}
		}

		// Iterate through the entries and extract []IndexEntry.
		updatedIndexEntries := make([]types.IndexEntry, 0, len(indexEntryMap))
		for _, ie := range indexEntryMap {
			updatedIndexEntries = append(updatedIndexEntries, ie)
		}

		// Write the Index based on these new []IndexEntry slice. Will automatically sort based on Filename.
		if err := plumbing.WriteIndex(updatedIndexEntries); err != nil {
			fmt.Printf("couldn't update .git/index: %s\n", err)
			os.Exit(1)
		}

	default:
		fmt.Println("usage: gegit checkout [-b <new-branch>] <commit-ish> [-- <path>]")
		os.Exit(1)
	}
}
