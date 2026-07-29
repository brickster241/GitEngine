package porcelain

import (
	"encoding/hex"
	"strings"
	"fmt"
	"os"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
)

// Invoked from main.go. RebaseBranch handles 'gegit rebase <upstream>':
// replay the current branch's commits on top of upstream, one three-way
// cherry-pick per commit (base = the commit's own parent), preserving each
// commit's author and author date while stamping a fresh committer. Conflicts
// abort the whole rebase BEFORE any ref moves, so the branch is never left in
// a half-rebased state.
func RebaseBranch(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("rebase",
		"Reapply the current branch's commits on top of the given upstream commit-ish, preserving authorship. Aborts atomically on conflict.",
		"gegit rebase <upstream>")
	fls.Parse(args[1:])
	pos := fls.Args()
	if len(pos) != 1 {
		fmt.Println("usage: gegit rebase <upstream>")
		os.Exit(1)
	}

	// Resolve both ends.
	headInfo, err := plumbing.ReadHEADInfo()
	if err != nil {
		fmt.Println("Error reading .git/HEAD:", err)
		os.Exit(1)
	}
	if headInfo.Detached || headInfo.Branch == "" {
		fmt.Println("fatal: rebase requires being on a branch")
		os.Exit(1)
	}
	upstreamSHA, err := plumbing.ResolveCommitish(pos[0])
	if err != nil {
		fmt.Printf("fatal: invalid upstream '%s'\n", pos[0])
		os.Exit(1)
	}
	headSHA := headInfo.SHA

	// Nothing to do / fast-forward cases.
	if anc, err := plumbing.IsAncestor(upstreamSHA, headSHA); err == nil && anc {
		fmt.Println("Current branch is up to date.")
		return
	}
	base, err := plumbing.MergeBase(headSHA, upstreamSHA)
	if err != nil {
		fmt.Println("Error finding merge base:", err)
		os.Exit(1)
	}
	if base == headSHA {
		// HEAD is behind upstream: rebase degenerates to fast-forward.
		upCommit, _ := plumbing.ReadCommit(upstreamSHA)
		headContent := fmt.Sprintf("ref: refs/heads/%s\n", headInfo.Branch)
		if err := plumbing.CheckoutToTreeSHA(upCommit.TreeSHA, headContent); err != nil {
			fmt.Println("Error updating working tree:", err)
			os.Exit(1)
		}
		plumbing.UpdateBranchRefWithSHA(headInfo.Branch, upstreamSHA)
		fmt.Println("Fast-forwarded to upstream.")
		return
	}

	// The commits to replay, oldest first.
	chain, err := plumbing.RevListFirstParent(headSHA, base)
	if err != nil {
		fmt.Println("Error listing commits:", err)
		os.Exit(1)
	}

	committer, err := getAuthorInfo()
	if err != nil {
		fmt.Println("Error fetching committer info from .git/config:", err)
		os.Exit(1)
	}

	// Replay: every commit is a 3-way merge of (its parent → it) onto the
	// growing new tip. All objects are written BEFORE any ref moves — a
	// conflict aborts with the branch untouched.
	newTip := upstreamSHA
	for i, commitSHA := range chain {
		commit, err := plumbing.ReadCommit(commitSHA)
		if err != nil {
			fmt.Println("Error reading commit:", err)
			os.Exit(1)
		}

		// Base tree = the commit's own parent's tree (empty history handled
		// by the initial-commit case: no parents means replay onto tip as-is).
		var parentTree [20]byte
		if len(commit.ParentsSHA) > 0 {
			parentCommit, err := plumbing.ReadCommit(commit.ParentsSHA[0])
			if err != nil {
				fmt.Println("Error reading parent commit:", err)
				os.Exit(1)
			}
			parentTree = parentCommit.TreeSHA
		}
		tipCommit, err := plumbing.ReadCommit(newTip)
		if err != nil {
			fmt.Println("Error reading tip commit:", err)
			os.Exit(1)
		}

		merged, conflicts, err := plumbing.MergeTreesThreeWay(
			parentTree, tipCommit.TreeSHA, commit.TreeSHA,
			"HEAD", hex.EncodeToString(commitSHA[:])[:7])
		if err != nil {
			fmt.Println("Error merging trees:", err)
			os.Exit(1)
		}
		if len(conflicts) > 0 {
			fmt.Printf("error: could not apply %s (%s)\n",
				hex.EncodeToString(commitSHA[:])[:7], firstLine(commit.Message))
			for _, c := range conflicts {
				fmt.Printf("CONFLICT (%s): %s\n", c.Kind, c.Path)
			}
			fmt.Println("Rebase aborted; branch left unchanged.")
			os.Exit(1)
		}

		indexEntries := plumbing.EntriesToIndex(merged)
		root := plumbing.BuildTreeFromIndex(indexEntries)
		treeSHA, err := plumbing.WriteTree(root)
		if err != nil {
			fmt.Println("Error writing tree:", err)
			os.Exit(1)
		}

		// Preserve author + author date; committer is "us, now". ReadCommit
		// returns the message WITH its trailing newline and WriteCommitFull
		// appends one — trim so the replayed bytes match the original.
		newTip, err = plumbing.WriteCommitFull(treeSHA, [][20]byte{newTip},
			commit.Author, commit.AuthorDate,
			committer, plumbing.DefaultCommitDate("GIT_COMMITTER_DATE"),
			strings.TrimSuffix(commit.Message, "\n"))
		if err != nil {
			fmt.Println("Error writing replayed commit:", err)
			os.Exit(1)
		}
		fmt.Printf("Applied %d/%d: %s\n", i+1, len(chain), firstLine(commit.Message))
	}

	// Publish: move the branch ref, then make the worktree + index match.
	if err := plumbing.UpdateBranchRefWithSHA(headInfo.Branch, newTip); err != nil {
		fmt.Println("Error updating branch ref:", err)
		os.Exit(1)
	}
	tipCommit, _ := plumbing.ReadCommit(newTip)
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", headInfo.Branch)
	if err := plumbing.CheckoutToTreeSHA(tipCommit.TreeSHA, headContent); err != nil {
		fmt.Println("Error updating working tree:", err)
		os.Exit(1)
	}
	fmt.Printf("Successfully rebased %d commits onto %s.\n",
		len(chain), hex.EncodeToString(upstreamSHA[:])[:7])
}

// firstLine returns the first line of a commit message.
func firstLine(msg string) string {
	for i := 0; i < len(msg); i++ {
		if msg[i] == '\n' {
			return msg[:i]
		}
	}
	return msg
}
