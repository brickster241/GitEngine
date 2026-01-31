package plumbing

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brickster241/GitEngine/utils/types"
)

// writeCommit creates a Git commit object, writes it to the object database, and returns the commit SHA.
func WriteCommit(treeSHA [20]byte, parentsSHA [][20]byte, author types.Author, message string) ([20]byte, error) {
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

	// Write Commit Object to .git/objects
	return WriteObject(types.CommitObject, content.Bytes())
}

// ReadCommit reads and parses a commit object from the object database.
func ReadCommit(sha [20]byte) (*types.CommitNode, error) {
	objType, data, err := ReadObject(hex.EncodeToString(sha[:]))
	if err != nil {
		return nil, err
	}

	// Check whether it is a commit object
	if objType != types.CommitObject {
		return nil, fmt.Errorf("object is not a commit")
	}

	// Iterate Line by Line
	lines := strings.Split(string(data), "\n")
	var c types.CommitNode
	i := 0

	// Parse headers
	for ; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			i++ // skip blank lines
			break
		}

		switch {
		case strings.HasPrefix(line, "tree"): // Tree Line
			hash, _ := hex.DecodeString(line[5:])
			copy(c.TreeSHA[:], hash)

		case strings.HasPrefix(line, "parent "): // Parent Line(s)
			hash, _ := hex.DecodeString(line[7:])
			var p [20]byte
			copy(p[:], hash)
			c.ParentsSHA = append(c.ParentsSHA, p)

		case strings.HasPrefix(line, "author "): // Author Line
			parts := strings.Split(line, " ")
			emailLen := len(parts[2])
			c.Author = types.Author{
				Name:  parts[1],
				Email: parts[2][1 : emailLen-1],
			}

		case strings.HasPrefix(line, "committer "): // Committer Line
			c.Committer = line[10:]
		}
	}

	// Remaining Lines = commit message
	c.Message = strings.Join(lines[i:], "\n")
	return &c, nil
}

// ResolveCommitish takes a commit-ish string, and returns the commit sha associated with it.
func ResolveCommitish(commitIsh string) ([20]byte, error) {

	var base, suffix string
	var resultSHA [20]byte // Store resultSHA

	// Check for <base>[(^~)<suffix>]+ pattern.
	idx := strings.IndexAny(commitIsh, "^~")
	if idx != -1 {
		base = commitIsh[:idx]
	} else {
		base = commitIsh[:]
		idx = len(commitIsh)
	}

	// if base is HEAD
	if base == "HEAD" {

		// Fetch HEAD Info
		headInfo, err := ReadHEADInfo()
		if err != nil {
			return [20]byte{}, err
		}

		// If HEAD is detached, return SHA directly, else use ReadBranchRef
		if headInfo.Detached {
			resultSHA = headInfo.SHA
		} else {
			// If HEAD is not detached, use ref to get commit SHA
			SHA, exists := ReadBranchRef(headInfo.Branch)
			if !exists {
				return [20]byte{}, fmt.Errorf("could not read HEAD ref")
			}
			resultSHA = SHA
		}
	} else {

		// Assume it is a branch instead
		shaHex, exists := ReadBranchRef(base)
		if !exists {

			// Check if it is an commit type object in .git/objects
			objType, _, err := ReadObject(base)
			if err != nil || objType != types.CommitObject {
				return [20]byte{}, fmt.Errorf("invalid object name: %s", base)
			}
		}
		resultSHA = shaHex
	}

	// Iterate the loop, for each ^ or ~, come up with logic
	for idx < len(commitIsh) {

		// Get the next sign and do strconv.Atoi to get the number
		sign := commitIsh[idx]
		suffix = commitIsh[idx+1:]
		numStr := "1"

		// If there is some part on the right remaining. Extract the number from it.
		if len(suffix) > 0 {
			nxtIdx := strings.IndexAny(suffix, "^~")
			if nxtIdx == -1 {
				numStr = suffix
				idx = len(commitIsh) // Point to the next sign index, which is end of string.
			} else {
				numStr = suffix[:nxtIdx]
				if len(numStr) == 0 {
					idx += 1
					numStr = "1"
				} else {
					idx += (1 + len(numStr))
				}
			}
		} else {
			idx += 1
		}

		// Convert numStr to int to get the number of times we should loop
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return [20]byte{}, fmt.Errorf("%s is not valid suffix after %v", numStr, sign)
		}

		switch sign {
		case '~':
			// Get num(th) ancestor from baseSHA
			for i := 0; i < num; i++ {
				commit, err := ReadCommit(resultSHA)
				if err != nil {
					return [20]byte{}, err
				}
				// If parent commit exists, go to it.
				if len(commit.ParentsSHA) > 0 {
					resultSHA = commit.ParentsSHA[0]
				} else {
					return [20]byte{}, fmt.Errorf("invalid object name: %s", commitIsh)
				}
			}
		case '^':
			// Get num(th) Parent from the baseSHA
			commit, err := ReadCommit(resultSHA)
			if err != nil {
				return [20]byte{}, err
			}
			// If Nth parent for the curr commit exists, go to it.
			if len(commit.ParentsSHA) >= num && num > 0 {
				resultSHA = commit.ParentsSHA[num-1]
			} else {
				return [20]byte{}, fmt.Errorf("invalid object name: %s", commitIsh)
			}
		default:
			return [20]byte{}, fmt.Errorf("Error: invalid suffix. Should be ^ or ~.")
		}
	}

	// idx reached the end of the commitIsh string, that means we can return the sha now.
	return resultSHA, nil
}
