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

	// If there are no flags set, check whether the format is gegit branch <branch_name> otherwise return an error.
	if count == 0 {

		switch len(pos) {
		// List all branches
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

		// Create a new branch -> gegit branch <branch_name>, pointing to HEAD but don't switch it.
		case 1:
			fmt.Println("Creating bRANCH")

		// Invalid usage
		default:
			fmt.Println("usage: gegit branch [-d] <branch-name> | -m <old-branch> <new-branch> | -c <existing-branch> <new-branch>")
			os.Exit(1)
		}
	} else if count == 1 {
		// If there is exactly one flag set, i.e. m or c or d.

	} else {
		fmt.Print(m, c, d)
		// Invalid usage
		fmt.Println("usage: gegit branch [-d] <branch-name> | -m <old-branch> <new-branch> | -c <existing-branch> <new-branch>")
		os.Exit(1)
	}

}
