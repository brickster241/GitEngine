package porcelain

const (
	DefaultFilePerm = 0o644                      // rw-r--r--
	DefaultDirPerm  = 0o755                      // rwxr-xr-x
	Head            = "ref: refs/heads/master\n" // Default .git/HEAD content
	DirModeStr      = "040000"
	FileModeStr     = "100644"
	Config          = `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
	ignorecase = true
	precomposeunicode = true

[user]
	name = username
	email = user@email.com

	` // Default .git/config content
)

// Define the necessary directory structure
var Dir_paths = []string{
	".git",
	".git/objects",
	".git/refs",
	".git/refs/heads",
	".git/refs/tags",
	// ".git/hooks",
	// ".git/info",
}

// Tree represents a tree object
type Tree struct {
	Entries []TreeEntry // list of entries in this tree
}

// TreeEntry represents an entry in a tree object
type TreeEntry struct {
	Mode string   // "100644", "100755", "40000"
	Name string   // filename or directory name
	SHA  [20]byte // raw SHA-1 of blob or subtree
}

// CommitNode represents a commit object
type CommitNode struct {
	treeSHA    [20]byte   // root tree SHA
	parentsSHA [][20]byte // parents commit SHA, can be multiple for merges
	author     Author     // author info
	committer  string     // committer info
	message    string     // commit message
}

type Author struct {
	Name  string
	Email string
}

// TreeNode represents a node in the in-memory tree structure. Will be used to build tree objects from the index.
type TreeNode struct {
	Files map[string]IndexEntry // blobs
	Dirs  map[string]*TreeNode  // subtrees
}
