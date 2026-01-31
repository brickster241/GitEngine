package plumbing

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
)

// ReadHEADInfo reads .git/HEAD and determines whether HEAD is detached. It returns: ref path (if symbolic), commit SHA (if detached), detached flag
func ReadHEADInfo() (*types.HeadInfo, error) {
	data, err := os.ReadFile(filepath.Join(".git", "HEAD"))
	if err != nil {
		return nil, err
	}

	// Symbolic Ref
	line := strings.TrimSpace(string(data))
	if strings.HasPrefix(line, "ref: refs/heads/") {
		return &types.HeadInfo{
			Branch:   strings.TrimPrefix(line, "ref: refs/heads/"),
			Detached: false,
		}, nil
	}

	// Detached HEAD
	shaBytes, err := hex.DecodeString(line)
	if err != nil || len(shaBytes) != 20 {
		return nil, fmt.Errorf("invalid HEAD contents")
	}

	// Copy SHA into new variable
	var sha [20]byte
	copy(sha[:], shaBytes)

	return &types.HeadInfo{
		SHA:      sha,
		Detached: true,
	}, nil
}

// ReadBranchRef reads a branch name (e.g. master). Returns: SHA, exists flag (false if branch does not exist)
func ReadBranchRef(branch string) ([20]byte, bool) {
	data, err := os.ReadFile(filepath.Join(".git", "refs", "heads", branch))
	if err != nil {
		return [20]byte{}, false
	}

	// Read SHA value
	line := strings.TrimSpace(string(data))
	shaBytes, err := hex.DecodeString(line)
	if err != nil || len(shaBytes) != 20 {
		return [20]byte{}, false
	}

	// Copy SHA bytes
	var sha [20]byte
	copy(sha[:], shaBytes)
	return sha, true

}

// UpdateBranch updates a branch ref to point to the given SHA. This is used during commit when HEAD is not detached.
func UpdateBranch(branch string, sha [20]byte) error {
	refPath := filepath.Join(".git", "refs", "heads", branch)

	// Create directory and file
	if err := os.MkdirAll(filepath.Dir(refPath), constants.DefaultDirPerm); err != nil {
		return err
	}

	// Update the file with SHA
	return os.WriteFile(
		refPath,
		[]byte(fmt.Sprintf("%x\n", sha)),
		constants.DefaultFilePerm,
	)
}

// UpdateHEADDetached moves HEAD directly to a commit SHA. Used ONLY when HEAD is detached.
func UpdateHEADDetached(sha [20]byte) error {

	// Write SHA to file
	return os.WriteFile(
		filepath.Join(".git", "HEAD"),
		[]byte(fmt.Sprintf("%x\n", sha)),
		constants.DefaultFilePerm,
	)
}
