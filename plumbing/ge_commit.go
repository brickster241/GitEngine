package plumbing

import (
	"bytes"
	"encoding/hex"
	"fmt"
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
