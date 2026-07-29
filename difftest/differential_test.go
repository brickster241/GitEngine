// Package difftest is GitEngine's differential test suite: every scenario is
// executed twice — once with real git, once with the gegit binary — through
// identical steps under pinned identities and dates, then the resulting SHAs
// (blobs, trees, commits, merge commits, rebased tips) are compared. Agreement
// with native Git is the acceptance criterion, not our own expectations.
package difftest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brickster241/GitEngine/plumbing"
)

var gegitBin string

// pinnedEnv makes commits reproducible across both tools: same identity,
// same author/committer timestamps (the override env vars real Git honors
// and gegit mirrors).
var pinnedEnv = []string{
	"GIT_AUTHOR_DATE=1700000000 +0530",
	"GIT_COMMITTER_DATE=1700000000 +0530",
	"GIT_AUTHOR_NAME=username",
	"GIT_AUTHOR_EMAIL=user@email.com",
	"GIT_COMMITTER_NAME=username",
	"GIT_COMMITTER_EMAIL=user@email.com",
	// Neutralize user/system git config so tests run identically everywhere.
	"GIT_CONFIG_GLOBAL=/dev/null",
	"GIT_CONFIG_SYSTEM=/dev/null",
	"HOME=/tmp",
}

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "gegit_bin_*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	gegitBin = filepath.Join(tmp, "gegit")
	build := exec.Command("go", "build", "-o", gegitBin, "../cmd")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("building gegit: " + err.Error())
	}
	os.Exit(m.Run())
}

// run executes a command in dir with the pinned environment.
func run(t *testing.T, dir, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), pinnedEnv...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out), code
}

func git(t *testing.T, dir string, args ...string) string {
	out, code := run(t, dir, "git", args...)
	if code != 0 {
		t.Fatalf("git %v failed (%d):\n%s", args, code, out)
	}
	return strings.TrimSpace(out)
}

func ge(t *testing.T, dir string, args ...string) string {
	out, code := run(t, dir, gegitBin, args...)
	if code != 0 {
		t.Fatalf("gegit %v failed (%d):\n%s", args, code, out)
	}
	return strings.TrimSpace(out)
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// pair creates twin repos: one for git, one for gegit.
func pair(t *testing.T) (string, string) {
	t.Helper()
	gitDir := t.TempDir()
	geDir := t.TempDir()
	git(t, gitDir, "init", "-b", "master")
	ge(t, geDir, "init")
	return gitDir, geDir
}

// headSHA reads the current HEAD commit hex from either repo layout.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	head, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	ref := strings.TrimSpace(strings.TrimPrefix(string(head), "ref: "))
	sha, err := os.ReadFile(filepath.Join(dir, ".git", ref))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(sha))
}

func branchSHA(t *testing.T, dir, branch string) string {
	t.Helper()
	sha, err := os.ReadFile(filepath.Join(dir, ".git", "refs", "heads", branch))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(sha))
}

// both applies the same file writes to both repos.
func both(t *testing.T, gitDir, geDir string, files map[string]string) {
	for name, content := range files {
		write(t, gitDir, name, content)
		write(t, geDir, name, content)
	}
}

// addCommitBoth stages the given paths and commits with the same message in
// both repos, returning (gitSHA, geSHA).
func addCommitBoth(t *testing.T, gitDir, geDir, msg string, paths ...string) (string, string) {
	git(t, gitDir, append([]string{"add"}, paths...)...)
	ge(t, geDir, append([]string{"add"}, paths...)...)
	git(t, gitDir, "commit", "-m", msg)
	ge(t, geDir, "commit", "-m", msg)
	return headSHA(t, gitDir), headSHA(t, geDir)
}

// TestCommitChainParity proves blob → tree → commit byte-compatibility,
// including the two classic tree-encoding traps: directory modes must render
// as "40000" (no leading zero) and directories must sort as "name/".
func TestCommitChainParity(t *testing.T) {
	gitDir, geDir := pair(t)

	both(t, gitDir, geDir, map[string]string{
		"foo.txt":       "hello\n",
		"foo/inner.txt": "nested\n",
		"foo-bar":       "tricky sort neighbor\n", // '-' < '.' < '/' — the trap
		"docs/a/b.md":   "deep\n",
	})
	gitSHA, geSHA := addCommitBoth(t, gitDir, geDir, "first commit",
		"foo.txt", "foo/inner.txt", "foo-bar", "docs/a/b.md")
	if gitSHA != geSHA {
		t.Fatalf("commit 1 diverged:\ngit   %s\ngegit %s", gitSHA, geSHA)
	}

	// Second commit on top: parent linkage must also match.
	both(t, gitDir, geDir, map[string]string{"foo.txt": "hello v2\n"})
	gitSHA, geSHA = addCommitBoth(t, gitDir, geDir, "second commit", "foo.txt")
	if gitSHA != geSHA {
		t.Fatalf("commit 2 diverged:\ngit   %s\ngegit %s", gitSHA, geSHA)
	}
}

// forkedRepos builds the shared fixture: master with two commits, a feature
// branch forked after the first commit carrying its own commit.
func forkedRepos(t *testing.T, featureFiles, masterFiles map[string]string) (string, string) {
	gitDir, geDir := pair(t)

	both(t, gitDir, geDir, map[string]string{"base.txt": "line1\nline2\nline3\nline4\nline5\n"})
	addCommitBoth(t, gitDir, geDir, "base", "base.txt")

	git(t, gitDir, "branch", "feature")
	ge(t, geDir, "branch", "feature")

	// Advance master.
	both(t, gitDir, geDir, masterFiles)
	paths := keys(masterFiles)
	addCommitBoth(t, gitDir, geDir, "master work", paths...)

	// Work on feature.
	git(t, gitDir, "checkout", "feature")
	ge(t, geDir, "checkout", "feature")
	both(t, gitDir, geDir, featureFiles)
	paths = keys(featureFiles)
	git(t, gitDir, append([]string{"add"}, paths...)...)
	ge(t, geDir, append([]string{"add"}, paths...)...)
	git(t, gitDir, "commit", "-m", "feature work")
	ge(t, geDir, "commit", "-m", "feature work")

	// Back on master for merge/rebase scenarios.
	git(t, gitDir, "checkout", "master")
	ge(t, geDir, "checkout", "master")
	return gitDir, geDir
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestMergeBaseParity(t *testing.T) {
	gitDir, geDir := forkedRepos(t,
		map[string]string{"feature.txt": "f\n"},
		map[string]string{"master.txt": "m\n"})

	gitBase := git(t, gitDir, "merge-base", "master", "feature")

	// gegit exposes merge-base through plumbing; call it in the repo dir.
	cwd, _ := os.Getwd()
	if err := os.Chdir(geDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	a, _ := plumbing.ResolveCommitish("master")
	b, _ := plumbing.ResolveCommitish("feature")
	base, err := plumbing.MergeBase(a, b)
	if err != nil {
		t.Fatal(err)
	}
	geBase := shaHex(base)
	if gitBase != geBase {
		t.Fatalf("merge-base diverged:\ngit   %s\ngegit %s", gitBase, geBase)
	}
}

func shaHex(sha [20]byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 40)
	for i, b := range sha {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0xf]
	}
	return string(out)
}

// TestFastForwardMergeParity: feature is strictly ahead — both tools must
// land on the identical tip without creating a commit.
func TestFastForwardMergeParity(t *testing.T) {
	gitDir, geDir := pair(t)
	both(t, gitDir, geDir, map[string]string{"a.txt": "one\n"})
	addCommitBoth(t, gitDir, geDir, "c1", "a.txt")

	git(t, gitDir, "branch", "feature")
	ge(t, geDir, "branch", "feature")
	git(t, gitDir, "checkout", "feature")
	ge(t, geDir, "checkout", "feature")
	both(t, gitDir, geDir, map[string]string{"b.txt": "two\n"})
	addCommitBoth(t, gitDir, geDir, "c2", "b.txt")

	git(t, gitDir, "checkout", "master")
	ge(t, geDir, "checkout", "master")
	git(t, gitDir, "merge", "feature")
	ge(t, geDir, "merge", "feature")

	if g, e := headSHA(t, gitDir), headSHA(t, geDir); g != e {
		t.Fatalf("fast-forward diverged:\ngit   %s\ngegit %s", g, e)
	}
}

// TestCleanMergeCommitParity: divergent branches touching different files —
// the full merge COMMIT SHA must match native Git (same tree, parents,
// message, pinned dates).
func TestCleanMergeCommitParity(t *testing.T) {
	gitDir, geDir := forkedRepos(t,
		map[string]string{"feature.txt": "feature content\n"},
		map[string]string{"master.txt": "master content\n"})

	git(t, gitDir, "merge", "feature", "-m", "Merge branch 'feature'")
	ge(t, geDir, "merge", "feature")

	if g, e := headSHA(t, gitDir), headSHA(t, geDir); g != e {
		t.Fatalf("merge commit diverged:\ngit   %s\ngegit %s", g, e)
	}
}

// TestCleanMergeSameFileDistinctRegions: both branches edit the SAME file in
// different regions — content-level 3-way must produce git's exact tree.
func TestCleanMergeSameFileDistinctRegions(t *testing.T) {
	gitDir, geDir := forkedRepos(t,
		map[string]string{"base.txt": "line1\nline2\nline3\nline4\nFEATURE5\n"},
		map[string]string{"base.txt": "MASTER1\nline2\nline3\nline4\nline5\n"})

	git(t, gitDir, "merge", "feature", "-m", "Merge branch 'feature'")
	ge(t, geDir, "merge", "feature")

	if g, e := headSHA(t, gitDir), headSHA(t, geDir); g != e {
		t.Fatalf("same-file clean merge diverged:\ngit   %s\ngegit %s", g, e)
	}
}

// TestConflictParity: both branches edit the same line — both tools must
// refuse to commit and flag the same file.
func TestConflictParity(t *testing.T) {
	gitDir, geDir := forkedRepos(t,
		map[string]string{"base.txt": "line1\nline2\nFEATURE\nline4\nline5\n"},
		map[string]string{"base.txt": "line1\nline2\nMASTER\nline4\nline5\n"})

	_, gitCode := run(t, gitDir, "git", "merge", "feature")
	geOut, geCode := run(t, geDir, gegitBin, "merge", "feature")

	if gitCode == 0 || geCode == 0 {
		t.Fatalf("expected both to conflict: git=%d gegit=%d", gitCode, geCode)
	}
	gitConflicts := git(t, gitDir, "diff", "--name-only", "--diff-filter=U")
	if gitConflicts != "base.txt" {
		t.Fatalf("git conflicted on %q", gitConflicts)
	}
	if !strings.Contains(geOut, "Merge conflict in base.txt") {
		t.Fatalf("gegit did not flag base.txt:\n%s", geOut)
	}
	// The conflicted worktree file must carry standard markers.
	content, _ := os.ReadFile(filepath.Join(geDir, "base.txt"))
	for _, m := range []string{"<<<<<<<", "=======", ">>>>>>>"} {
		if !strings.Contains(string(content), m) {
			t.Fatalf("marker %q missing in conflicted file:\n%s", m, content)
		}
	}
}

// TestRebaseParity: feature replayed onto an advanced master — with pinned
// dates the rebased TIP SHA must equal native Git's.
func TestRebaseParity(t *testing.T) {
	gitDir, geDir := forkedRepos(t,
		map[string]string{"feature.txt": "f1\n"},
		map[string]string{"master.txt": "m1\n"})

	// A second feature commit so the rebase replays a chain, not one commit.
	git(t, gitDir, "checkout", "feature")
	ge(t, geDir, "checkout", "feature")
	both(t, gitDir, geDir, map[string]string{"feature2.txt": "f2\n"})
	addCommitBoth(t, gitDir, geDir, "feature work 2", "feature2.txt")

	git(t, gitDir, "rebase", "master")
	ge(t, geDir, "rebase", "master")

	if g, e := branchSHA(t, gitDir, "feature"), branchSHA(t, geDir, "feature"); g != e {
		t.Fatalf("rebase diverged:\ngit   %s\ngegit %s", g, e)
	}
}

// TestFsckCrossValidation: gegit fsck must certify a repository whose every
// object was written by NATIVE git — the object store formats are the same
// store.
func TestFsckCrossValidation(t *testing.T) {
	gitDir := t.TempDir()
	git(t, gitDir, "init", "-b", "master")
	write(t, gitDir, "x.txt", "one\n")
	write(t, gitDir, "d/y.txt", "two\n")
	git(t, gitDir, "add", "x.txt", "d/y.txt")
	git(t, gitDir, "commit", "-m", "by real git")

	out := ge(t, gitDir, "fsck")
	if !strings.Contains(out, "Object store integrity: OK") {
		t.Fatalf("fsck failed on a native-git repo:\n%s", out)
	}
}
