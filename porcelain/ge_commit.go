package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
)

// Invoked from main.go. CommitChanges handles the 'gegit commit' command to commit changes to the repository.
// It creates a new commit containing the current contents of the index and the given log message describing the changes. The new commit is a direct child of HEAD, usually the tip of the current branch, and the branch is updated to point to it
func CommitChanges(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("commit",
		"Create a new commit containing the current contents of the index and the given log message describing the changes. The new commit is a direct child of HEAD, usually the tip of the current branch, and the branch is updated to point to it.",
		"gegit commit -m <message>")
	message := fls.String("m", "", "The commit message.")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// Check if there are any args left
	if *message == "" || len(pos) != 0 {
		fmt.Println("usage: gegit commit -m <message>")
		os.Exit(1)
	}

	// Load the index
	entries, err := plumbing.LoadIndex()
	if err != nil {
		fmt.Println("Error loading index:", err)
		os.Exit(1)
	} else if len(entries) == 0 {
		fmt.Println("Error: Nothing to commmit")
		os.Exit(1)
	}

	// Build in-memory tree structure
	root := plumbing.BuildTreeFromIndex(entries)

	// Write tree Objects (recursive)
	treeSHA, err := plumbing.WriteTree(root)
	if err != nil {
		fmt.Println("Error writing tree object:", err)
		os.Exit(1)
	}

	// Read HEAD (for a parent commit, if any)
	headInfo, err := plumbing.ReadHEADInfo()
	if err != nil {
		fmt.Println("Error reading .git/HEAD:", err)
		os.Exit(1)
	}

	parentsSHA := [][20]byte{}
	if headInfo.Detached {
		parentsSHA = append(parentsSHA, headInfo.SHA)
	} else {
		parentSHA, exists := plumbing.ReadBranchRef(headInfo.Branch)
		if exists {
			parentsSHA = append(parentsSHA, parentSHA)
		}
		// Else initial commit, no parents
	}

	// Check if there are no changes between Head tree and current index tree
	if len(parentsSHA) > 0 {
		headCommit, err := plumbing.ReadCommit(parentsSHA[0])
		if err == nil && headCommit.TreeSHA == treeSHA {
			fmt.Println("nothing to commit, working tree clean")
			os.Exit(0)
		} else if err != nil {
			fmt.Println("Error reading HEAD commit:", err)
			os.Exit(1)
		}
	}

	// Author, Committer Info
	author, err := getAuthorInfo()
	if err != nil {
		fmt.Println("Error fetching author info from .git/config:", err)
		os.Exit(1)
	}

	// Write commit object
	commitSHA, err := plumbing.WriteCommit(treeSHA, parentsSHA, author, *message)
	if err != nil {
		fmt.Println("Error writing commit object:", err)
		os.Exit(1)
	}

	// Update HEAD reference
	if headInfo.Detached {
		if err := plumbing.UpdateHEADDetached(commitSHA); err != nil {
			fmt.Println("Error updating .git/HEAD:", err)
			os.Exit(1)
		}
	} else {
		if err := plumbing.UpdateBranch(headInfo.Branch, commitSHA); err != nil {
			fmt.Println("Error updating .git/HEAD:", err)
			os.Exit(1)
		}
	}

	// hex value of Commit SHA, print it on the console.
	commitHex := hex.EncodeToString(commitSHA[:])

	fmt.Printf("[%s] %s\n",
		commitHex[:6],
		strings.Split(*message, "\n")[0],
	)
}
