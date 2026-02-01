package porcelain

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
)

// Invoked from main.go. ShowStatus handles the 'gegit status' command to show the working tree status.
func ShowStatus(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("status",
		"Displays paths that have differences between the index file and the current HEAD commit, paths that have differences between the working tree and the index file, and paths in the working tree that are not tracked by Git (and are not ignored by gitignore).",
		"gegit status")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	if len(pos) != 0 {
		// Invalid usage
		fmt.Println("usage: gegit status")
		os.Exit(1)
	}

	// Get HeadTree Map
	headTreeSHA, ok, err := plumbing.ReadHEADTreeSHA()
	headTreeEntryMap := map[string]types.TreeEntry{}

	if ok {
		headTreeEntryMap, _ = plumbing.FlattenTree(headTreeSHA)
	}

	// Load the index
	entries, err := plumbing.LoadIndex()
	if err != nil {
		fmt.Println("Error loading index:", err)
		return
	}

	// Create a map for quick lookup of existing entries
	indexMap := plumbing.IndexToMap(entries)

	// Create path -> hash Map for workTree
	workTreeMap := map[string][20]byte{}

	// Walk the working directory to find all files
	_ = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println("Error accessing path:", err)
			return nil
		}

		// Skip the root directory itself
		if path == "." {
			return nil
		}

		// Skip the .gegit directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Skip directories, only add files
		if d.IsDir() {
			return nil
		}

		// Clean and normalize the path
		cleanPath := filepath.ToSlash(filepath.Clean(path))
		data, _ := os.ReadFile(cleanPath)
		sha, err := plumbing.HashObject(types.BlobObject, data)
		if err == nil {
			workTreeMap[cleanPath] = sha
		} else {
			fmt.Println("Error Hashing File:", err)
		}
		return nil
	})

	staged := map[string]types.StatusType{}
	unstaged := map[string]types.StatusType{}
	untracked := []string{}

	// Changes to be committed (HEAD <-> INDEX)
	for path, idxEntry := range indexMap {
		headTreeEntry, exists := headTreeEntryMap[path]
		if !exists {
			// Does not exist in HEAD, will be added as a new file
			staged[path] = types.AddedStatus
		} else if headTreeEntry.SHA != idxEntry.SHA1 {
			// Exists in HEAD, but has a different SHA, that means it was modified
			staged[path] = types.ModifiedStatus
		}
	}

	// Changes not staged (INDEX <-> WORKTREE)
	for path, idxEntry := range indexMap {
		workTreeSHA, exists := workTreeMap[path]
		if !exists {
			// Does not exist in workTree, but present in index so deletion has not been added yet.
			unstaged[path] = types.DeletedStatus
		} else if workTreeSHA != idxEntry.SHA1 {
			// Changes exist in worktree, but not in index even though file exists. So, modifications have not been added yet.
			unstaged[path] = types.ModifiedStatus
		}
	}

	// Staged Deletions
	for path := range headTreeEntryMap {
		if _, exists := indexMap[path]; !exists {
			// Present in HEAD, but not in index, so stagedDelete.
			staged[path] = types.DeletedStatus
		}
	}

	// Untracked Files
	for path := range workTreeMap {
		if _, exists := indexMap[path]; !exists {
			// Present in Worktree, but no entry in Index, so untracked.
			untracked = append(untracked, path)
		}
	}

	// First Line : On branch <branchName> or HEAD detached at <sha>
	head, _ := plumbing.ReadHEADInfo()

	if head.Detached {
		fmt.Printf("HEAD detached at %s\n", hex.EncodeToString(head.SHA[:]))
	} else {
		branch := filepath.Base(head.Branch)
		fmt.Printf("On branch %s\n", branch)
	}

	// Also mention if any commits are not present
	if headTreeSHA == [20]byte{} {
		fmt.Println("No commits yet")
	}
	if len(staged)+len(unstaged)+len(untracked) == 0 {
		fmt.Println("Nothing to commit, working tree clean")
		return
	}

	// Changes to be committed - Section
	if len(staged) > 0 {
		fmt.Printf("\n%sChanges to be commited:%s\n", constants.BoldColor, constants.ResetColor)

		for _, path := range utils.SortedKeys(staged) {
			switch staged[path] {
			case types.AddedStatus:
				printStatusLine(constants.GreenColor, "new file:", path)
			case types.ModifiedStatus:
				printStatusLine(constants.GreenColor, "modified:", path)
			case types.DeletedStatus:
				printStatusLine(constants.GreenColor, "deleted:", path)
			}
		}
	}

	// Changes not staged for commit - Section
	if len(unstaged) > 0 {
		fmt.Printf("\n%sChanges not staged for commit:%s\n", constants.BoldColor, constants.ResetColor)
		fmt.Println("\t(use \"git add <file>...\" to update what will be committed)")

		for _, path := range utils.SortedKeys(unstaged) {
			switch unstaged[path] {
			case types.ModifiedStatus:
				printStatusLine(constants.RedColor, "modified:", path)
			case types.DeletedStatus:
				printStatusLine(constants.RedColor, "deleted:", path)
			}
		}
	}

	// Untracked files
	if len(untracked) > 0 {
		fmt.Printf("\n%sUntracked files:%s\n", constants.BoldColor, constants.ResetColor)

		sort.Strings(untracked)
		for _, f := range untracked {
			fmt.Printf("\t%s%s%s\n", constants.RedColor, f, constants.ResetColor)
		}
	}
}

// Utility to print Status Line
func printStatusLine(color, label, path string) {
	fmt.Printf("\t%s%-12s%s %s\n", color, label, path, constants.ResetColor)
}
