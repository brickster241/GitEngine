package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/constants"
)

// Invoked from main.go. MergeBranch handles 'gegit merge <branch>': join the
// given branch's history into the current branch — fast-forward when HEAD is
// an ancestor, otherwise a true 3-way merge (merge-base → tree merge → merge
// commit with two parents). Conflicts stop before committing: marker-annotated
// files land in the worktree, .git/MERGE_HEAD records the in-progress merge,
// and 'gegit merge --abort' restores the pre-merge state.
func MergeBranch(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("merge",
		"Incorporate changes from the named branch into the current branch: fast-forward when possible, otherwise a three-way merge with conflict detection.",
		"gegit merge <branch> | gegit merge --abort")
	abort := fls.Bool("abort", false, "Abort the in-progress merge and restore the pre-merge working tree.")
	fls.Parse(args[1:])
	pos := fls.Args()

	// HEAD state is needed by every path below.
	headInfo, err := plumbing.ReadHEADInfo()
	if err != nil {
		fmt.Println("Error reading .git/HEAD:", err)
		os.Exit(1)
	}

	mergeHeadPath := filepath.Join(".git", "MERGE_HEAD")

	// --- gegit merge --abort ---------------------------------------------
	if *abort {
		if _, err := os.Stat(mergeHeadPath); err != nil {
			fmt.Println("fatal: There is no merge to abort (MERGE_HEAD missing).")
			os.Exit(1)
		}
		headCommit, err := plumbing.ReadCommit(headInfo.SHA)
		if err != nil {
			fmt.Println("Error reading HEAD commit:", err)
			os.Exit(1)
		}
		headContent := fmt.Sprintf("ref: refs/heads/%s\n", headInfo.Branch)
		if headInfo.Detached {
			headContent = hex.EncodeToString(headInfo.SHA[:]) + "\n"
		}
		if err := plumbing.CheckoutToTreeSHA(headCommit.TreeSHA, headContent); err != nil {
			fmt.Println("Error restoring working tree:", err)
			os.Exit(1)
		}
		os.Remove(mergeHeadPath)
		fmt.Println("Merge aborted. Working tree restored to HEAD.")
		return
	}

	// --- gegit merge <branch> --------------------------------------------
	if len(pos) != 1 {
		fmt.Println("usage: gegit merge <branch> | gegit merge --abort")
		os.Exit(1)
	}
	if headInfo.SHA == [20]byte{} {
		fmt.Println("fatal: no commits on the current branch yet")
		os.Exit(1)
	}
	otherName := pos[0]
	otherSHA, err := plumbing.ResolveCommitish(otherName)
	if err != nil {
		fmt.Printf("merge: %s - not something we can merge\n", otherName)
		os.Exit(1)
	}
	headSHA := headInfo.SHA

	// Already up to date: other is an ancestor of HEAD (or the same commit).
	if anc, err := plumbing.IsAncestor(otherSHA, headSHA); err == nil && anc {
		fmt.Println("Already up to date.")
		return
	}

	// Fast-forward: HEAD is an ancestor of other — just move the ref.
	if anc, err := plumbing.IsAncestor(headSHA, otherSHA); err == nil && anc {
		otherCommit, err := plumbing.ReadCommit(otherSHA)
		if err != nil {
			fmt.Println("Error reading commit:", err)
			os.Exit(1)
		}
		headContent := fmt.Sprintf("ref: refs/heads/%s\n", headInfo.Branch)
		if err := plumbing.CheckoutToTreeSHA(otherCommit.TreeSHA, headContent); err != nil {
			fmt.Println("Error updating working tree:", err)
			os.Exit(1)
		}
		if err := plumbing.UpdateBranchRefWithSHA(headInfo.Branch, otherSHA); err != nil {
			fmt.Println("Error updating branch ref:", err)
			os.Exit(1)
		}
		fmt.Printf("Updating %s..%s\nFast-forward\n",
			hex.EncodeToString(headSHA[:])[:7], hex.EncodeToString(otherSHA[:])[:7])
		return
	}

	// True merge: find the base, merge the trees.
	baseSHA, err := plumbing.MergeBase(headSHA, otherSHA)
	if err != nil {
		fmt.Println("Error finding merge base:", err)
		os.Exit(1)
	}
	baseCommit, _ := plumbing.ReadCommit(baseSHA)
	headCommit, _ := plumbing.ReadCommit(headSHA)
	otherCommit, _ := plumbing.ReadCommit(otherSHA)

	merged, conflicts, err := plumbing.MergeTreesThreeWay(
		baseCommit.TreeSHA, headCommit.TreeSHA, otherCommit.TreeSHA,
		"HEAD", otherName)
	if err != nil {
		fmt.Println("Error merging trees:", err)
		os.Exit(1)
	}

	// Materialize the merge result in the working tree (clean files AND
	// marker-annotated conflict files).
	for path, entry := range merged {
		_, content, err := plumbing.ReadObject(hex.EncodeToString(entry.SHA[:]))
		if err != nil {
			fmt.Println("Error reading merged blob:", err)
			os.Exit(1)
		}
		if err := os.MkdirAll(filepath.Dir(path), constants.DefaultDirPerm); err != nil && filepath.Dir(path) != "." {
			fmt.Println("Error creating directories:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path, content, constants.DefaultFilePerm); err != nil {
			fmt.Println("Error writing merged file:", err)
			os.Exit(1)
		}
	}

	// Conflicts: record MERGE_HEAD, report, and stop before committing. The
	// index intentionally stays pre-merge so markers can never be committed
	// by accident.
	if len(conflicts) > 0 {
		os.WriteFile(mergeHeadPath, []byte(hex.EncodeToString(otherSHA[:])+"\n"), constants.DefaultFilePerm)
		for _, c := range conflicts {
			fmt.Printf("CONFLICT (%s): Merge conflict in %s\n", c.Kind, c.Path)
		}
		fmt.Println("Automatic merge failed; fix conflicts and then commit the result.")
		os.Exit(1)
	}

	// Clean merge: index → tree → merge commit with two parents.
	indexEntries := plumbing.EntriesToIndex(merged)
	if err := plumbing.WriteIndexLocked(indexEntries); err != nil {
		fmt.Println("Error writing index:", err)
		os.Exit(1)
	}
	root := plumbing.BuildTreeFromIndex(indexEntries)
	treeSHA, err := plumbing.WriteTree(root)
	if err != nil {
		fmt.Println("Error writing merged tree:", err)
		os.Exit(1)
	}

	author, err := getAuthorInfo()
	if err != nil {
		fmt.Println("Error fetching author info from .git/config:", err)
		os.Exit(1)
	}
	message := fmt.Sprintf("Merge branch '%s'", otherName)
	if headInfo.Branch != "master" && headInfo.Branch != "" {
		message = fmt.Sprintf("Merge branch '%s' into %s", otherName, headInfo.Branch)
	}
	commitSHA, err := plumbing.WriteCommit(treeSHA, [][20]byte{headSHA, otherSHA}, author, message)
	if err != nil {
		fmt.Println("Error writing merge commit:", err)
		os.Exit(1)
	}
	if err := plumbing.UpdateBranchRefWithSHA(headInfo.Branch, commitSHA); err != nil {
		fmt.Println("Error updating branch ref:", err)
		os.Exit(1)
	}

	fmt.Printf("Merge made by the 'three-way' strategy. [%s] %s\n",
		hex.EncodeToString(commitSHA[:])[:7], message)
}
