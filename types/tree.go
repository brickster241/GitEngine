package types

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

// TreeNode represents a node in the in-memory tree structure. Will be used to build tree objects from the index.
type TreeNode struct {
	Files map[string]IndexEntry // blobs
	Dirs  map[string]*TreeNode  // subtrees
}
