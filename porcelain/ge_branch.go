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
)

// Invoked from main.go. BranchOps handles the 'gegit branch' command to list, create, rename or delete branch refs.
func BranchOps(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("branch",
		"List, create, rename or delete branches.",
		"usage: gegit branch [-d] <branch-name> | -m <old-branch> <new-branch> | -c <existing-branch> <new-branch>")
	m := fls.Bool("m", false, "With a -m option, <old-branch> will be renamed to <new-branch>.")
	c := fls.Bool("c", false, "The -c option has the exact same semantics as -m, except instead of the branch being renamed, it will be copied to a new name.")
	d := fls.Bool("d", false, "With a -d option, <branch-name> will be deleted. You may specify more than one branch for deletion.")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// No. of set flags
	count := fls.NFlag()

	switch count {
	case 0:
		// If there are no flags set, check whether the format is gegit branch <branch_name> otherwise return an error.
		switch len(pos) {
		// No extra arguments : List all branches
		case 0:
			branchList := []string{}
			if err := filepath.WalkDir(".git/refs/heads", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					fmt.Println("Error accessing path:", err)
					return nil
				}
				// Skip the root directory itself
				if path == ".git/refs/heads" {
					return nil
				}

				// Skip directories, only add files
				if d.IsDir() {
					return nil
				}

				// It is a file, so add the remaining relative path to a list
				rel, err := filepath.Rel(".git/refs/heads", path)
				if err != nil {
					return err
				}
				branchList = append(branchList, rel)

				return nil
			}); err != nil {
				fmt.Println("Error fetching branch list:", err)
				os.Exit(1)
			}

			// Sort the slice of branches
			sort.Strings(branchList)

			// Get current HEAD Info
			headInfo, err := plumbing.ReadHEADInfo()
			if err != nil {
				fmt.Println("Error fetching HEAD Info:", err)
				os.Exit(1)
			}

			// Check if a commit is present at HEAD.
			if headInfo.SHA == [20]byte{} {
				fmt.Println("No commits at HEAD")
				os.Exit(1)
			}

			// If HEAD is detached, add an extra line.
			if headInfo.Detached {
				hexSHA := hex.EncodeToString(headInfo.SHA[:])
				fmt.Printf("* %s(HEAD detached at %s)%s\n", constants.YellowColor, hexSHA[:7], constants.ResetColor)
			}
			for _, branch := range branchList {
				if branch == headInfo.Branch {
					fmt.Printf("* %s%s%s\n", constants.GreenColor, branch, constants.ResetColor)
				} else {
					fmt.Printf("  %s\n", branch)
				}
			}

		// Exactly one extra argument : Create a new branch -> gegit branch <branch_name>, pointing to HEAD but don't switch it.
		case 1:
			// Check whether the branch actually already exists
			_, exists := plumbing.ReadBranchRef(pos[0])
			if exists {
				fmt.Printf("Error: Branch named '%s' already exists\n", pos[0])
				os.Exit(1)
			}

			// Get current HEAD SHA by reading .git/HEAD
			headInfo, err := plumbing.ReadHEADInfo()
			if err != nil {
				fmt.Println("Error fetching HEAD Info:", err)
				os.Exit(1)
			}

			// If there is no SHA in head ref
			if headInfo.SHA == [20]byte{} {
				fmt.Println("Error: HEAD branch ref missing")
				os.Exit(1)
			}

			// Create Branch Ref
			if err := plumbing.CreateBranchRef(pos[0], headInfo.SHA); err != nil {
				fmt.Println("Error creating branch:", err)
				os.Exit(1)
			}

		// Default case: Invalid usage
		default:
			fmt.Println("usage: gegit branch [-d] <branch-name> | -m <old-branch> <new-branch> | -c <existing-branch> <new-branch>")
			os.Exit(1)
		}

	case 1: // If there is exactly one flag set, i.e. m or c or d.

		// git branch -d <branch_name>
		if *d {

			// Check if the HEAD is symbolic and branch_name is the current branch
			headInfo, err := plumbing.ReadHEADInfo()
			if err != nil {
				fmt.Printf("Error: could not fetch HEAD -> %s\n", err)
				os.Exit(1)
			}

			for _, curr := range pos {
				// Check whether the branch actually already exists
				_, exists := plumbing.ReadBranchRef(curr)
				if !exists {
					fmt.Printf("Error: Branch named '%s' doesn't exist\n", curr)
					os.Exit(1)
				}

				if !headInfo.Detached && headInfo.Branch == curr {
					fmt.Printf("Error: Cannot delete current Branch named '%s'\n", curr)
					os.Exit(1)
				}

				// Remove .git/refs/heads/<branch_name> file
				cleanPath := filepath.Join(".git", "refs", "heads", curr)
				if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
					fmt.Printf("Error: could not delete %s -> %s\n", cleanPath, err)
					os.Exit(1)
				}
			}
		} else

		// git branch -m <old_branch> <new_branch>
		if *m {

			if len(pos) != 2 {
				// Invalid usage
				fmt.Println("usage: gegit branch [-d] <branch-name> | -m <old-branch> <new-branch> | -c <existing-branch> <new-branch>")
				os.Exit(1)
			}
			old_branch := pos[0]
			new_branch := pos[1]

			// Check whether the old branch actually exists
			_, exists := plumbing.ReadBranchRef(old_branch)
			if !exists {
				fmt.Printf("Error: Branch named '%s' doesn't exist\n", old_branch)
				os.Exit(1)
			}

			// Check whether the renamed branch already exists
			_, exists = plumbing.ReadBranchRef(new_branch)
			if exists {
				fmt.Printf("Error: Renamed Branch '%s' already exists\n", new_branch)
				os.Exit(1)
			}

			// Rename .git/refs/heads/<old_branch> to .gits/refs/heads/<new_branch>
			oldPath := filepath.Clean(filepath.Join(".git", "refs", "heads", old_branch))
			newPath := filepath.Clean(filepath.Join(".git", "refs", "heads", new_branch))
			headPath := filepath.Join(".git", "HEAD")
			headContent := "ref: " + filepath.Join("refs", "heads", new_branch) + "\n"

			if err := os.Rename(oldPath, newPath); err != nil {
				fmt.Printf("Error: Could not rename Branch '%s' to %s -> %s\n", old_branch, new_branch, err)
				os.Exit(1)
			}

			// If old branch is the current branch, then update .git/HEAD if it is symbolic
			headInfo, err := plumbing.ReadHEADInfo()
			if err != nil {
				fmt.Printf("Error: could not fetch HEAD -> %s\n", err)
				os.Exit(1)
			}

			// Write to .git/HEAD with ref: refs/heads/<new_branch>\n
			if !headInfo.Detached && headInfo.Branch == old_branch {
				if err := os.WriteFile(headPath, []byte(headContent), constants.DefaultFilePerm); err != nil {
					fmt.Printf("Error writing to file '%s': %s\n", headPath, err)
					os.Exit(1)
				}
			}
		} else

		// git branch -c <old_branch> <new_branch>
		if *c {

			if len(pos) != 2 {
				// Invalid usage
				fmt.Println("usage: gegit branch [-d] <branch-name> | -m <old-branch> <new-branch> | -c <existing-branch> <new-branch>")
				os.Exit(1)
			}

			old_branch := pos[0]
			new_branch := pos[1]

			// Check whether the old branch exists
			sha, exists := plumbing.ReadBranchRef(old_branch)
			if !exists {
				fmt.Printf("Error: Branch named '%s' doesn't exist\n", old_branch)
				os.Exit(1)
			}

			// Check whether the new branch already exists
			_, exists = plumbing.ReadBranchRef(new_branch)
			if exists {
				fmt.Printf("Error: Branch '%s' already exists\n", new_branch)
				os.Exit(1)
			}

			// Create new branch pointing to same SHA
			if err := plumbing.CreateBranchRef(new_branch, sha); err != nil {
				fmt.Printf("Error: Could not create branch '%s' -> %s\n", new_branch, err)
				os.Exit(1)
			}
		}

	default:
		// Invalid usage
		fmt.Println("usage: gegit branch [-d] <branch-name> | -m <old-branch> <new-branch> | -c <existing-branch> <new-branch>")
		os.Exit(1)
	}

}
