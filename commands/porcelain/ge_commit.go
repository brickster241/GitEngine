package porcelain

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Invoked from main.go. CommitChanges handles the 'gegit commit' command to commit changes to the repository.
func CommitChanges(args []string) {
	if len(args) != 3 || args[1] != "-m" {
		fmt.Println("usage: gegit commit -m <message>")
		os.Exit(1)
	}
}

// buildTreeFromIndex builds an in-memory tree structure from the given index entries.
func buildTreeFromIndex() *TreeNode {

	// Load the index
	indexPath := filepath.Join(".git", "index")
	entries, err := loadIndex(indexPath)
	if err != nil {
		fmt.Println("Error loading index:", err)
		return nil
	}

	// Build the tree structure
	root := &TreeNode{
		Files: make(map[string]IndexEntry),
		Dirs:  make(map[string]*TreeNode),
	}

	// Populate the tree structure
	for _, entry := range entries {
		parts := strings.Split(entry.Filename, "/")

		currNode := root
		// Traverse or create directories
		for i := 0; i < len(parts)-1; i++ {
			dir := parts[i]
			if currNode.Dirs[dir] == nil {
				currNode.Dirs[dir] = &TreeNode{
					Files: make(map[string]IndexEntry),
					Dirs:  make(map[string]*TreeNode),
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

// writeTree recursively writes tree objects to the object database and returns the SHA of the root tree.
func writeTree(node *TreeNode) ([20]byte, error) {
	var entries []TreeEntry

	// recursion first (dirs)
	for name, child := range node.Dirs {
		sha, err := writeTree(child)
		if err != nil {
			return [20]byte{}, nil
		}

		// Add TreeEntry to the list of entries
		entries = append(entries, TreeEntry{
			Mode: DirModeStr,
			Name: name,
			SHA:  sha,
		})
	}

	// Files
	for name, ie := range node.Files {
		entries = append(entries, TreeEntry{
			Mode: FileModeStr,
			Name: name,
			SHA:  ie.SHA1,
		})
	}

	// Sort the entries now
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return hashTreeObject(entries)
}

// writeTreeObject serializes a list of sorted TreeEntry values into a Git tree object.
func hashTreeObject(entries []TreeEntry) ([20]byte, error) {
	var content bytes.Buffer

	// Build Tree content (no header yet)
	for _, e := range entries {
		// "<mode> <name>\0"
		content.WriteString(e.Mode)
		content.WriteByte(' ')
		content.WriteString(e.Name)
		content.WriteByte(0)

		// raw 20-byte SHA
		content.Write(e.SHA[:])
	}

	// Build full object: "tree <size>\0<content>"
	header := fmt.Sprintf("tree %d\x00", content.Len())
	store := append([]byte(header), content.Bytes()...)

	// Compute SHA-1 hash
	sum := sha1.Sum(store)
	hash := hex.EncodeToString(sum[:])

	// Prepare the tree object file path
	dir := filepath.Join(".git", "objects", hash[:2])
	file := filepath.Join(dir, hash[2:])

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, DefaultDirPerm); err != nil {
		return [20]byte{}, err
	}

	// Z-lib compress and write the object
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(store); err != nil {
		return [20]byte{}, err
	}

	// Close the writer
	if err := w.Close(); err != nil {
		return [20]byte{}, err
	}

	// Write to file
	if err := os.WriteFile(file, buf.Bytes(), DefaultFilePerm); err != nil {
		return [20]byte{}, err
	}

	return sum, nil
}

// writeCommitObject creates a Git commit object, writes it to the object database, and returns the commit SHA.
func writeCommitObject(treeSHA [20]byte, parentsSHA *[][20]byte, author Author, message string) ([20]byte, error) {
	var content bytes.Buffer

	// Tree Line : "tree <sha_hex>\n"
	content.WriteString("tree ")
	content.WriteString(hex.EncodeToString(treeSHA[:]))
	content.WriteByte('\n')

	// Parent Line per parent (if exists) : "parent <sha_parent1>\n"
	for _, parentSHA := range *parentsSHA {
		content.WriteString("parent ")
		content.WriteString(hex.EncodeToString(parentSHA[:]))
		content.WriteByte('\n')
	}

	// Calculate sign, and timezone
	now := time.Now()
	timestamp := now.Unix()
	_, offset := now.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}

	tz := fmt.Sprintf("%s%02d%02d", sign, offset/3600, (offset%3600)/60)

	// Author Line : "author <name> <email> <timestamp> <timezone>"
	authorLine := fmt.Sprintf(
		"author %s <%s> %d %s\n",
		author.Name,
		author.Email,
		timestamp,
		tz,
	)
	// Author Line : "committer <name> <email> <timestamp> <timezone>"
	committerLine := fmt.Sprintf(
		"committer %s <%s> %d %s\n",
		author.Name,
		author.Email,
		timestamp,
		tz,
	)

	content.WriteString(authorLine)
	content.WriteString(committerLine)

	// blank line before message
	content.WriteByte('\n')

	// Commit Message (must end with newline)
	content.WriteString(message)
	content.WriteByte('\n')

	// Build full commit object
	header := fmt.Sprintf("commit %d\x00", content.Len())
	store := append([]byte(header), content.Bytes()...)

	// Compute SHA-1 hash
	sum := sha1.Sum(store)
	hash := hex.EncodeToString(sum[:])

	// Prepare the commit object file path
	dir := filepath.Join(".git", "objects", hash[:2])
	file := filepath.Join(dir, hash[2:])

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, DefaultDirPerm); err != nil {
		return [20]byte{}, err
	}

	// Z-lib compress and write the object
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(store); err != nil {
		return [20]byte{}, err
	}

	// Close the writer
	if err := w.Close(); err != nil {
		return [20]byte{}, err
	}

	// Write to file
	if err := os.WriteFile(file, buf.Bytes(), DefaultFilePerm); err != nil {
		return [20]byte{}, err
	}

	return sum, nil
}

// readHEADCommit returns the current HEAD commit SHA. if no commits exist, it returns nil.
func readHEADCommit() (*[20]byte, error) {
	return nil, nil
}

// updateBranchRef updates the current branch to point to the given commmit SHA.
func updateBranchRef(commitSHA [20]byte) error {
	return nil
}
