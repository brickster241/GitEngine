package porcelain

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
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
	indexPath := filepath.Join(".git", "index")
	entries, err := loadIndex(indexPath)
	if err != nil {
		fmt.Println("Error loading index:", err)
		return
	} else if len(entries) == 0 {
		fmt.Println("Error: Nothing to commmit")
		return
	}

	// Build in-memory tree structure
	root := buildTreeFromIndex(entries)

	// Write tree Objects (recursive)
	treeSHA, err := writeTree(root)
	if err != nil {
		fmt.Println("Error writing tree object:", err)
		return
	}

	// Read HEAD (parent commit, if any)
	parentSHA, err := readHEADCommit()
	if err != nil {
		fmt.Println("Error reading .git/HEAD:", err)
		return
	}

	parentsSHA := [][20]byte{}
	if parentSHA != nil {
		parentsSHA = append(parentsSHA, *parentSHA)
	}

	// Author, Committer Info
	author, err := getAuthorInfo()
	if err != nil {
		fmt.Println("Error fetching author info from .git/config:", err)
		return
	}

	// Write commit object
	commitSHA, err := writeCommitObject(treeSHA, parentsSHA, author, message)
	if err != nil {
		fmt.Println("Error writing commit object:", err)
		return
	}

	// Update current branch ref
	if err := updateBranchRef(commitSHA); err != nil {
		fmt.Println("Error updating branch ref:", err)
		return
	}

	// hex value of Commit SHA, print it on the console.
	commitHex := hex.EncodeToString(commitSHA[:])

	fmt.Printf("[%s] %s\n",
		commitHex[:6],
		strings.Split(message, "\n")[0],
	)

}

// buildTreeFromIndex builds an in-memory tree structure from the given index entries.
func buildTreeFromIndex(entries []types.IndexEntry) *types.TreeNode {

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

// writeTree recursively writes tree objects to the object database and returns the SHA of the root tree.
func writeTree(node *types.TreeNode) ([20]byte, error) {
	var entries []types.TreeEntry

	// recursion first (dirs)
	for name, child := range node.Dirs {
		sha, err := writeTree(child)
		if err != nil {
			return [20]byte{}, nil
		}

		// Add TreeEntry to the list of entries
		entries = append(entries, types.TreeEntry{
			Mode: constants.DirModeStr,
			Name: name,
			SHA:  sha,
		})
	}

	// Files
	for name, ie := range node.Files {
		entries = append(entries, types.TreeEntry{
			Mode: constants.FileModeStr,
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
func hashTreeObject(entries []types.TreeEntry) ([20]byte, error) {
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
	if err := os.MkdirAll(dir, constants.DefaultDirPerm); err != nil {
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
	if err := os.WriteFile(file, buf.Bytes(), constants.DefaultFilePerm); err != nil {
		return [20]byte{}, err
	}

	return sum, nil
}

// writeCommitObject creates a Git commit object, writes it to the object database, and returns the commit SHA.
func writeCommitObject(treeSHA [20]byte, parentsSHA [][20]byte, author types.Author, message string) ([20]byte, error) {
	var content bytes.Buffer

	// Tree Line : "tree <sha_hex>\n"
	content.WriteString("tree ")
	content.WriteString(hex.EncodeToString(treeSHA[:]))
	content.WriteByte('\n')

	// Parent Line per parent (if exists) : "parent <sha_parent1>\n"
	for _, parentSHA := range parentsSHA {
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
	if err := os.MkdirAll(dir, constants.DefaultDirPerm); err != nil {
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
	if err := os.WriteFile(file, buf.Bytes(), constants.DefaultFilePerm); err != nil {
		return [20]byte{}, err
	}

	return sum, nil
}

// readHEADCommit returns the current HEAD commit SHA. if no commits exist, it returns nil.
func readHEADCommit() (*[20]byte, error) {

	headPath := filepath.Join(".git", "HEAD")

	// Read .git/HEAD file
	data, err := os.ReadFile(headPath)
	if err != nil {
		return nil, err
	}

	head := strings.TrimSpace(string(data))

	// Only support attached HEAD for now
	if !strings.HasPrefix(head, "ref: ") {
		return nil, fmt.Errorf("detached HEAD not supported")
	}

	refPath := filepath.Join(".git", strings.TrimPrefix(head, "ref: "))

	// Read file at refPath if exists
	refData, err := os.ReadFile(refPath)
	if err != nil {
		// No commits yet -> first commit
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	shaHex := strings.TrimSpace(string(refData))
	if shaHex == "" {
		return nil, nil
	}

	// Decode SHA
	raw, err := hex.DecodeString(shaHex)
	if err != nil || len(raw) != 20 {
		return nil, fmt.Errorf("invalid commit SHA in ref")
	}

	var sha [20]byte
	copy(sha[:], raw)
	return &sha, nil
}

// updateBranchRef updates the current branch to point to the given commmit SHA.
func updateBranchRef(commitSHA [20]byte) error {
	headPath := filepath.Join(".git", "HEAD")

	// Read .git/HEAD file
	data, err := os.ReadFile(headPath)
	if err != nil {
		return err
	}

	head := strings.TrimSpace(string(data))

	// Only support attached HEAD for now
	if !strings.HasPrefix(head, "ref: ") {
		return fmt.Errorf("detached HEAD not supported")
	}

	refPath := filepath.Join(".git", strings.TrimPrefix(head, "ref: "))

	shaHex := hex.EncodeToString(commitSHA[:]) + "\n"
	// Write shaHex to file
	if err := os.WriteFile(refPath, []byte(shaHex), constants.DefaultFilePerm); err != nil {
		return err
	}

	return nil
}
