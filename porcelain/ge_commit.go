package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/brickster241/GitEngine/plumbing"
)

// Invoked from main.go. CommitChanges handles the 'gegit commit' command to commit changes to the repository. It creates a new commit from the current index and advances the current branch to point to it.
func CommitChanges(args []string) {

	// Check args length and if correct flag is present.
	if len(args) != 3 || args[1] != "-m" {
		fmt.Println("usage: gegit commit -m <message>")
		os.Exit(1)
	}

	// Extract the message
	message := args[2]

	// Load the index
	entries, err := plumbing.LoadIndex()
	if err != nil {
		fmt.Println("Error loading index:", err)
		return
	} else if len(entries) == 0 {
		fmt.Println("Error: Nothing to commmit")
		return
	}

	// Build in-memory tree structure
	root := plumbing.BuildTreeFromIndex(entries)

	// Write tree Objects (recursive)
	treeSHA, err := plumbing.WriteTree(root)
	if err != nil {
		fmt.Println("Error writing tree object:", err)
		return
	}

	// Read HEAD (for a parent commit, if any)
	headInfo, err := plumbing.ReadHEADInfo()
	if err != nil {
		fmt.Println("Error reading .git/HEAD:", err)
		return
	}

	parentsSHA := [][20]byte{}
	if headInfo.Detached {
		parentsSHA = append(parentsSHA, headInfo.SHA)
	} else {
		parentSHA, exists := plumbing.ReadBranchRef(headInfo.Ref)
		if exists {
			parentsSHA = append(parentsSHA, parentSHA)
		}
		// Else initial commit, no parents
	}

	// Author, Committer Info
	author, err := getAuthorInfo()
	if err != nil {
		fmt.Println("Error fetching author info from .git/config:", err)
		return
	}

	// Write commit object
	commitSHA, err := plumbing.WriteCommit(treeSHA, parentsSHA, author, message)
	if err != nil {
		fmt.Println("Error writing commit object:", err)
		return
	}

	// Update HEAD reference
	if headInfo.Detached {
		if err := plumbing.UpdateHEADDetached(commitSHA); err != nil {
			fmt.Println("Error updating .git/HEAD:", err)
			return
		}
	} else {
		if err := plumbing.UpdateBranch(headInfo.Ref, commitSHA); err != nil {
			fmt.Println("Error updating .git/HEAD:", err)
			return
		}
	}

	// hex value of Commit SHA, print it on the console.
	commitHex := hex.EncodeToString(commitSHA[:])

	fmt.Printf("[%s] %s\n",
		commitHex[:6],
		strings.Split(message, "\n")[0],
	)
}
