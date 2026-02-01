package plumbing

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
)

// BuildTreeFromIndex builds an in-memory tree structure from the given index entries.
func BuildTreeFromIndex(entries []types.IndexEntry) *types.TreeNode {

	// Build the tree structure
	root := &types.TreeNode{
		Files: make(map[string]types.IndexEntry),
		Dirs:  make(map[string]*types.TreeNode),
	}

	// Populate the tree structure
	for _, entry := range entries {
		parts := strings.Split(entry.Filename, "/")

		currNode := root
		// Traverse or create directories
		for i := 0; i < len(parts)-1; i++ {
			dir := parts[i]
			if currNode.Dirs[dir] == nil {
				currNode.Dirs[dir] = &types.TreeNode{
					Files: make(map[string]types.IndexEntry),
					Dirs:  make(map[string]*types.TreeNode),
				}
			}
			currNode = currNode.Dirs[dir]
		}

		// Add the file to the current directory node
		file := parts[len(parts)-1]
		currNode.Files[file] = entry
	}
	return root
}

// WriteTree recursively writes tree objects to the object database and returns the SHA of the root tree.
func WriteTree(node *types.TreeNode) ([20]byte, error) {
	var entries []types.TreeEntry

	// recursion first (dirs)
	for name, child := range node.Dirs {
		sha, err := WriteTree(child)
		if err != nil {
			return [20]byte{}, nil
		}

		// Add TreeEntry to the list of entries
		entries = append(entries, types.TreeEntry{
			Mode: constants.ModeTree,
			Name: name,
			SHA:  sha,
			Type: types.TreeObject,
		})
	}

	// Files
	for name, ie := range node.Files {
		entries = append(entries, types.TreeEntry{
			Mode: constants.ModeFile,
			Name: name,
			SHA:  ie.SHA1,
			Type: types.BlobObject,
		})
	}

	// Sort the entries now
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	var content bytes.Buffer

	// Build Tree content (no header yet)
	for _, e := range entries {
		// "<mode> <name>\0"
		modeStr := fmt.Sprintf("%06o", e.Mode)
		content.WriteString(modeStr)
		content.WriteByte(' ')
		content.WriteString(e.Name)
		content.WriteByte(0)

		// raw 20-byte SHA
		content.Write(e.SHA[:])
	}

	// Write Tree Object to .git/objects
	return WriteObject(types.TreeObject, content.Bytes())
}

// ReadTreeCurrentLevel reads one shaHex object, decodes it and prints it in a type-specific but non-recursive way.
func ReadTreeCurrentLevel(shaHex string) ([]types.TreeEntry, error) {

	// Read Tree Object
	objType, content, err := ReadObject(shaHex)
	if err != nil {
		return nil, err
	}

	// Added check - see whether it is a tree or not.
	if objType != types.TreeObject {
		return nil, fmt.Errorf("object %s is not a tree", shaHex)
	}

	entries := []types.TreeEntry{}
	i := 0

	for i < len(content) {
		// Find NUL separating "<mode> <name>" and SHA
		nullIdx := bytes.IndexByte(content[i:], 0)
		if nullIdx == -1 {
			return nil, fmt.Errorf("corrupt tree object")
		}

		header := string(content[i : i+nullIdx])
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid tree entry header")
		}

		mode := parts[0]
		name := parts[1]

		// RAW SHA (next 20 bytes)
		shaStart := i + nullIdx + 1
		shaEnd := shaStart + 20
		if shaEnd > len(content) {
			return nil, fmt.Errorf("truncated tree object")
		}

		var sha [20]byte
		copy(sha[:], content[shaStart:shaEnd])

		// Parse Mode
		uint32Mode, err := utils.ParseModeStr(mode)
		if err != nil {
			return nil, fmt.Errorf("invalid mode format")
		}

		entryType := types.BlobObject
		if uint32Mode == constants.ModeTree {
			entryType = types.TreeObject
		}

		entries = append(entries, types.TreeEntry{
			Name: name,
			Mode: uint32Mode,
			SHA:  sha,
			Type: entryType,
		})

		i = shaEnd
	}

	return entries, nil
}

// FlattenTree recursively walks a tree object and returns a flat map of path → TreeEntry (like Git's index representation).
func FlattenTree(treeSHA [20]byte) (map[string]types.TreeEntry, error) {
	out := make(map[string]types.TreeEntry)
	err := flattenTreeRecur(treeSHA, "", out)
	return out, err
}

func flattenTreeRecur(treeSHA [20]byte, prefix string, out map[string]types.TreeEntry) error {

	// Read Tree at current level
	entries, err := ReadTreeCurrentLevel(hex.EncodeToString(treeSHA[:]))
	if err != nil {
		return err
	}

	// Go through entries at current level
	for _, e := range entries {

		// Generate Path using Prefix and Name
		path := e.Name
		if prefix != "" {
			path = filepath.Join(prefix, e.Name)
		}

		// In either case of blob or a tree, we add it to the map
		out[path] = types.TreeEntry{
			Name: path,
			Type: e.Type,
			SHA:  e.SHA,
			Mode: e.Mode,
		}

		// It is a Tree, recursive call
		if e.Type == types.TreeObject {
			if err := flattenTreeRecur(e.SHA, path, out); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReadHEADTreeSHA returns the tree SHA pointed to by HEAD. If no commits exist yet, returns (nil, false).
func ReadHEADTreeSHA() ([20]byte, bool, error) {

	// Get HEAD Info
	headInfo, err := ReadHEADInfo()
	if err != nil {
		return [20]byte{}, false, err
	}

	var commitSHA [20]byte

	if headInfo.Detached {
		commitSHA = headInfo.SHA
	} else {
		sha, exists := ReadBranchRef(headInfo.Branch)
		if !exists {
			return [20]byte{}, false, nil // no commits yet
		}
		commitSHA = sha
	}

	// Read Content for the Commit object
	_, content, err := ReadObject(hex.EncodeToString(commitSHA[:]))
	if err != nil {
		return [20]byte{}, false, err
	}

	// Parse tree line
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "tree ") {
			treeHex := strings.TrimPrefix(line, "tree ")
			treeSHA, _ := hex.DecodeString(treeHex)
			var shaArr [20]byte
			copy(shaArr[:], treeSHA)
			return shaArr, true, nil
		}
	}

	// Return error if not found
	return [20]byte{}, false, fmt.Errorf("invalid commit object content: missing tree line")
}

// ResolveTreeish takes a tree-ish string, and returns the tree sha associated with it.
func ResolveTreeish(treeIsh string) ([20]byte, error) {

	// Check whether the tree-ish is actually a commit-ish. If Tree-ish is actually a Commit-ish, return SHA directly using ReadCommit.
	commitSHA, err := ResolveCommitish(treeIsh)
	if err == nil {
		// Use commit SHA to get tree val
		commit, err := ReadCommit(commitSHA)
		if err != nil {
			return [20]byte{}, err
		}
		return commit.TreeSHA, nil
	}

	// Special case, if treeIsh = HEAD^{tree}. HEAD^{N} are already covered above
	if treeIsh == "HEAD^{tree}" {
		treeSHA, ok, err := ReadHEADTreeSHA()
		if err != nil {
			return [20]byte{}, err
		}
		if !ok {
			return [20]byte{}, fmt.Errorf("No commits yet")
		}
		return treeSHA, nil
	}

	// Check whether the tree-ish object is a valid SHA and is of type tree / commit
	objType, _, err := ReadObject(treeIsh)
	if err != nil {
		return [20]byte{}, err
	}

	// Decode Tree-ish SHA
	treeSHA, err := hex.DecodeString(treeIsh)
	if err != nil {
		return [20]byte{}, err
	}
	var shaArr [20]byte
	copy(shaArr[:], treeSHA)

	// Check objType and confirm whether it's tree or blob Object. Commit object are parsed already.
	if objType == types.TreeObject {
		return shaArr, nil
	} else {
		// Blob Object : return error
		return [20]byte{}, fmt.Errorf("Object Type is not Tree-ish")
	}
}
